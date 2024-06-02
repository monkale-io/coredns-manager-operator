/*
Copyright 2024 monkale.io.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monkalev1alpha1 "github.com/monkale.io/coredns-manager-operator/api/v1alpha1"
)

// DNSConnectorReconciler reconciles a DNSConnector object
type DNSConnectorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monkale.monkale.io,resources=dnsconnectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monkale.monkale.io,resources=dnsconnectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monkale.monkale.io,resources=dnsconnectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete

func (r *DNSConnectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	var dnsConnector monkalev1alpha1.DNSConnector

	// Fetch DNSConnector from kubernetes
	if err := r.Get(ctx, req.NamespacedName, &dnsConnector); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else if !apierrors.IsNotFound(err) {
			log.Log.Error(err, "DNSConnector instance. Failed to get DNSConnector", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
	}

	// DNSConnector has been just created. Add finalizer.
	if !controllerutil.ContainsFinalizer(&dnsConnector, monkalev1alpha1.DnsConnectorsFinalizerName) {
		dnsConnObj := types.NamespacedName{Name: dnsConnector.Name, Namespace: dnsConnector.Namespace}
		clientK8sObj := dnsConnector.DeepCopy()
		if err := addFinalizer(ctx, r.Client, dnsConnObj, clientK8sObj, monkalev1alpha1.DnsConnectorsFinalizerName); err != nil {
			log.Log.Error(err, "DNSConnector instance. Failed to add finalizer", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// DNSConnector has been deleted. Reconcile Delete
	if !dnsConnector.DeletionTimestamp.IsZero() {
		log.Log.Info("DNSConnector instance. Connector is being deleted has been removed", "DNSConnector.Name", dnsConnector.Name)
		return r.reconcileDelete(ctx, &dnsConnector)
	}

	log.Log.Info("DNSConnector instance. Reconciling", "DNSConnector.Name", dnsConnector.Name)
	return r.reconcileDNSConnector(ctx, &dnsConnector)
}

// reconcileDNSConnector - processes DNSConnector.
func (r *DNSConnectorReconciler) reconcileDNSConnector(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	previousState := dnsConnector.DeepCopy()

	// detect coredns-config ConfigMap, looks up for the configmap. also ensure that CM has Corefile key. returns CM object
	log.Log.Info("DNSConnector instance. Reconciling. Fetch Coredns-config ConfigMap", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
	corednsConfCM, err := r.fetchCorednsConfCM(ctx, dnsConnector)
	if err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("could not detect corefile: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorError, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not fetch coredns Configmap", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}
	_, ok := corednsConfCM.Data[dnsConnector.Spec.CorednsCM.CorefileKey]
	if !ok {
		err := fmt.Errorf("key %s not found in CoreDNS ConfigMap", dnsConnector.Spec.CorednsCM.CorefileKey)
		message := fmt.Sprintf("could not detect corefile: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorError, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not extract corefile from coredns-config configMap", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}

	// backup original corefile
	log.Log.Info("DNSConnector instance. Reconciling. Back up original ConfigMap", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
	if err := r.backupOriginalCorefileCM(ctx, corednsConfCM); err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("could not backup the original corefile: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorError, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not backup original coredns-config configMap", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}

	// fetch coredns deployment
	log.Log.Info("DNSConnector instance. Reconciling. Fetch coredns deployment", "DNSConnector.Name", dnsConnector.Name)
	corednsDeployment, err := r.fetchCorednsDeployment(ctx, dnsConnector)
	if err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("could not find coredns deployment: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorError, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not detect coredns deployment", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}

	// get good zonefiles cm
	log.Log.Info("DNSConnector instance. Reconciling. Fetch DNSZones and Zonefile configMaps", "DNSConnector.Name", dnsConnector.Name)
	dnsZonesList, zonefileCMList, err := r.fetchGoodZonefileCM(ctx, dnsConnector)
	if err != nil {
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not fetch zonefile configMaps", "DNSConnector.Name", dnsConnector.Name)
		return ctrl.Result{}, err
	}

	// prepare corefile content.
	log.Log.Info("DNSConnector instance. Reconciling. Generate a new Corefile content for the configMap", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name, "CorednsDeployment.Name", corednsDeployment.GetName())
	updatedCorefileCM, err := generateCorefileCM(dnsConnector, &corednsConfCM, &zonefileCMList)
	if err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("could not generate a new corefile: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorUpdateErr, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Generate a new Corefile failure.", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name, "CorednsDeployment.Name", corednsDeployment.GetName())
		return ctrl.Result{}, err
	}

	// attach configmap to the zone configmaps to the corednsDeployment, but not updates!
	log.Log.Info("DNSConnector instance. Reconciling. Attach configmaps to coredns deployment", "DNSConnector.Name", dnsConnector.Name, "CorednsDeployment.Name", corednsDeployment.GetName())
	updatedCorednsDeployment, err := setZoneFileConifgMaps(*dnsConnector, corednsDeployment, &zonefileCMList)
	if err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("could not attach zone file config maps to deployment: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorUpdateErr, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not add Zonefile CMs to the coredns", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}

	// apply changes corefile
	log.Log.Info("DNSConnector instance. Reconciling. Apply all pending changes", "DNSConnector.Name", dnsConnector.Name, "CorednsDeployment.Name", corednsDeployment.GetName())
	if err := r.Update(ctx, updatedCorednsDeployment); err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("could not attach zone file config maps: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorUpdateErr, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not apply zone config maps to the Coredns", "DNSConnector.Name", dnsConnector.Name, "Deployment.metadata.name", updatedCorednsDeployment.GetName())
		return ctrl.Result{}, err
	}
	if err := r.Update(ctx, &updatedCorefileCM); err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("could not update corefile cm: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorUpdateErr, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could not apply changes to the corefile", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}

	// update status
	if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
		log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
		return ctrl.Result{}, err
	}
	setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorUpdating, "coredns is being updated")
	if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
		return ctrl.Result{}, err
	}

	// wait until coredns finishes to load changes
	if err := r.corednsIsHealthy(ctx, dnsConnector); err != nil {
		if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
			log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
			return ctrl.Result{}, err
		}
		err := errors.New("coredns is not healthy. Check coredns deployment log")
		message := fmt.Sprintf("healthcheck failure: %v", err)
		setDnsConnectorCondition(dnsConnector, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonConnectorUpdateErr, message)
		if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Error(err, "DNSConnector instance. Reconciling. Healthcheck failure", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}

	// Update status for all DNSZones
	if err := r.notifyDNSZones(ctx, &dnsZonesList, metav1.ConditionTrue, monkalev1alpha1.ConditionReasonZoneActive, "Picked up by DNSConnector"); err != nil {
		log.Log.Error(err, "DNSConnector instance. Reconciling. Could update DNSZone status", "DNSConnector.Name", dnsConnector.Name)
		return ctrl.Result{}, err
	}

	// Update status DNSConnector
	dnsZoneStats := []monkalev1alpha1.ProvisionedDNSZone{}
	for _, zoneCM := range zonefileCMList.Items {
		var dnsZoneStat monkalev1alpha1.ProvisionedDNSZone
		var okN, okD, okS bool
		dnsZoneStat.Name, okN = zoneCM.Annotations["DNSZoneRef"]
		dnsZoneStat.Domain, okD = zoneCM.Annotations["DomainName"]
		dnsZoneStat.SerialNumber, okS = zoneCM.Annotations["SerialNumber"]
		if !okN || !okD || !okS {
			err := errors.New("missing required annotation")
			log.Log.Error(err, "DNSConnector instance. Reconciling. Could not find required annotation", "ConfigMap.Name", zoneCM.Name, "Missing DNSZoneRef", !okN, "Missing Domain", !okD, "Missing SerialNumber", !okS)
			return ctrl.Result{}, err
		}
		dnsZoneStats = append(dnsZoneStats, dnsZoneStat)
	}
	if err := r.refreshDNSConnectorResource(ctx, previousState); err != nil {
		log.Log.Error(err, "DNSConnector instance. Reconciling. Failed to refresh DNSConnector resource", "DNSConnector.Name", dnsConnector.Name)
		return ctrl.Result{}, err
	}
	dnsConnector.Status.ProvisionedDNSZones = dnsZoneStats
	setDnsConnectorCondition(dnsConnector, metav1.ConditionTrue, monkalev1alpha1.ConditionReasonConnectorActive, "CoreDNS Ready")
	if err := r.dnsConnectorUpdateStatus(ctx, previousState, dnsConnector); err != nil {
		return ctrl.Result{}, err
	}

	log.Log.Info("DNSConnector instance. Reconcilation has been completed")
	return ctrl.Result{}, nil
}

// notifyDNSZones is used to Iterate over DNSZones and update theirs condition
func (r *DNSConnectorReconciler) notifyDNSZones(ctx context.Context, dnsZonesList *monkalev1alpha1.DNSZoneList, status metav1.ConditionStatus, reason, message string) error {
	for _, dnsZone := range dnsZonesList.Items {
		dnsZoneType := types.NamespacedName{Name: dnsZone.Name, Namespace: dnsZone.Namespace}
		dnsZoneObj := dnsZone.DeepCopy()
		if err := getObjFromK8s(ctx, r.Client, dnsZoneType, dnsZoneObj); err != nil {
			return fmt.Errorf("failed to refresh DNSRecord resource: %v", err)
		}
		setDnsZoneCondition(dnsZoneObj, status, reason, message)
		if err := r.Status().Update(ctx, dnsZoneObj); err != nil {
			return fmt.Errorf("failed to update status and condition: %v", err)
		}
	}
	return nil
}

// fetchCorednsConfCM used to lookup for confCM for DNSConnector and then returns coredns configmap
func (r *DNSConnectorReconciler) fetchCorednsConfCM(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) (corev1.ConfigMap, error) {
	//var corednsConfObj corev1.ConfigMap
	corednsConfObj := corev1.ConfigMap{}
	corednsConfType := types.NamespacedName{Name: dnsConnector.Spec.CorednsCM.Name, Namespace: dnsConnector.Namespace}
	if err := getObjFromK8s(ctx, r.Client, corednsConfType, &corednsConfObj); err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("failed to fetch coredns deployment object: %v", err)
	}
	return corednsConfObj, nil
}

// fetchCorednsDeployment determines the type of the CoreDNS deployment,
// asserts the kind, and fetches the corresponding object from Kubernetes.
func (r *DNSConnectorReconciler) fetchCorednsDeployment(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) (client.Object, error) {
	corednsResObj, err := monkalev1alpha1.AssertCorednsDeploymentType(dnsConnector.Spec.CorednsDeployment.Type)
	if err != nil {
		return nil, fmt.Errorf("DNSConnector type assertion failure: %v", err)
	}

	corednsResType := types.NamespacedName{Name: dnsConnector.Spec.CorednsDeployment.Name, Namespace: dnsConnector.Namespace}
	if err := getObjFromK8s(ctx, r.Client, corednsResType, corednsResObj); err != nil {
		return nil, fmt.Errorf("failed to fetch coredns deployment object: %v", err)
	}

	return corednsResObj, nil
}

// fetchGoodZonefileCM used to fetch zonefiles configMaps related to dnsConnector.
// It will fetch all dnsZones for DnsZoneConnectorIndex, then it will filter only Ready dnsZones.
// Eventually it will extract configmaps. Returns list of zone configmaps sorted a-z
func (r *DNSConnectorReconciler) fetchGoodZonefileCM(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) (monkalev1alpha1.DNSZoneList, corev1.ConfigMapList, error) {
	dnsZones := &monkalev1alpha1.DNSZoneList{}
	// list all DNSZone for using ConnectorName reference.
	fieldSelector := fields.OneTermEqualSelector(monkalev1alpha1.DnsZoneConnectorIndex, dnsConnector.Name)
	listOps := &client.ListOptions{
		FieldSelector: fieldSelector,
		Namespace:     dnsConnector.Namespace,
	}
	if err := r.List(ctx, dnsZones, listOps); err != nil {
		return monkalev1alpha1.DNSZoneList{}, corev1.ConfigMapList{}, fmt.Errorf("could not list DNSZones: %v", err)
	}

	// get only ready records
	goodZones := &monkalev1alpha1.DNSZoneList{}
	for _, dnsZone := range dnsZones.Items {
		for _, condition := range dnsZone.Status.Conditions {
			if condition.Type == monkalev1alpha1.ConditionZoneTypeReady && condition.Status == metav1.ConditionTrue {
				goodZones.Items = append(goodZones.Items, dnsZone)
				break
			}
		}
	}

	// Create a ConfigMapList to return the result
	configMapList := corev1.ConfigMapList{}
	for _, dnsZone := range goodZones.Items {
		cmObj := types.NamespacedName{Name: dnsZone.Status.ZoneConfigmap, Namespace: dnsZone.Namespace}
		configMap := corev1.ConfigMap{}
		if err := getObjFromK8s(ctx, r.Client, cmObj, &configMap); err != nil {
			return monkalev1alpha1.DNSZoneList{}, corev1.ConfigMapList{}, fmt.Errorf("could not get ConfigMap for DNSZone %s: %v", dnsZone.Name, err)
		}
		configMapList.Items = append(configMapList.Items, configMap)
	}

	// sort records a-z
	sort.Slice(configMapList.Items, func(i, j int) bool {
		return configMapList.Items[i].Name < configMapList.Items[j].Name
	})

	return *goodZones, configMapList, nil
}

// backupOriginalCorefileCM check if backup coredns configfile is already exist. If not create it. Otherwise just exit.
func (r *DNSConnectorReconciler) backupOriginalCorefileCM(ctx context.Context, corednsOrigConfCMObj corev1.ConfigMap) error {
	corednsBkpConfObj := corednsOrigConfCMObj.DeepCopy()
	corednsBkpConfObj.ResourceVersion = "" // resource version should not be set
	corednsBkpConfObj.Name = corednsBkpConfObj.Name + monkalev1alpha1.CorednsOriginalConfBkpSuffix
	corednsBkpConfType := types.NamespacedName{Name: corednsBkpConfObj.Name, Namespace: corednsBkpConfObj.Namespace}
	fetchErr := r.Get(ctx, corednsBkpConfType, corednsBkpConfObj)
	if apierrors.IsNotFound(fetchErr) {
		controllerutil.AddFinalizer(corednsBkpConfObj, monkalev1alpha1.DnsConnectorsFinalizerName)
		if err := r.Create(ctx, corednsBkpConfObj); err != nil {
			return fmt.Errorf("failed to create coredns backup configmap: %v", fetchErr)
		}
		return nil
	} else if fetchErr != nil {
		return fmt.Errorf("failure during getting the object from k8s: %v", fetchErr)
	}
	return nil
}

// restoreOriginalCorefileCM restores the original CoreDNS ConfigMap from its backup during the reconcile delete process.
func (r *DNSConnectorReconciler) restoreOriginalCorefileCM(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) error {
	// Define the name of the backup ConfigMap
	backupConfigMapName := dnsConnector.Spec.CorednsCM.Name + monkalev1alpha1.CorednsOriginalConfBkpSuffix
	backupConfigMapType := types.NamespacedName{Name: backupConfigMapName, Namespace: dnsConnector.Namespace}

	// fetch bkp cm
	backupConfigMapObj := &corev1.ConfigMap{}
	fetchErr := getObjFromK8s(ctx, r.Client, backupConfigMapType, backupConfigMapObj)
	if fetchErr != nil {
		if apierrors.IsNotFound(fetchErr) {
			return fmt.Errorf("backup configmap not found: %v", fetchErr)
		}
		return fmt.Errorf("failure during getting the backup configmap from k8s: %v", fetchErr)
	}

	// fetch original cm
	originalConfigMapType := types.NamespacedName{Name: dnsConnector.Spec.CorednsCM.Name, Namespace: dnsConnector.Namespace}
	originalConfigMapObj := &corev1.ConfigMap{}
	fetchErr = getObjFromK8s(ctx, r.Client, originalConfigMapType, originalConfigMapObj)
	if fetchErr != nil {
		return fmt.Errorf("failure during getting the original configmap from k8s: %v", fetchErr)
	}

	// resource version should not be set
	originalConfigMapObj.ResourceVersion = ""
	// overwrite cm data and apply
	originalConfigMapObj.Data = backupConfigMapObj.Data
	if err := r.Update(ctx, originalConfigMapObj); err != nil {
		return fmt.Errorf("failed to restore original coredns configmap: %v", err)
	}
	return nil
}

// corednsIsHealthy waits for the CoreDNS deployment to be ready within the specified timeout.
// If the deployment does not become ready within the timeout, it returns an error.
func (r *DNSConnectorReconciler) corednsIsHealthy(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) error {
	_ = log.FromContext(ctx)
	delay := 6 * time.Second
	interval := 3 * time.Second
	timeout := time.Duration(dnsConnector.Spec.WaitForUpdateTimeout) * time.Second
	endTime := time.Now().Add(timeout)
	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		time.Sleep(delay) // avoid race conditions, and false positive logs.
		corednsActualState, err := r.fetchCorednsDeployment(ctx, dnsConnector)
		if err != nil {
			return false, err
		}
		waitMsg := "DNSConnector instance. Waiting for CorednsDeployment to become healhy"
		remainingTime := int(time.Until(endTime).Seconds())
		remainingTimeFormatted := fmt.Sprintf("%d seconds", remainingTime)

		switch res := corednsActualState.(type) {
		case *appsv1.StatefulSet:
			log.Log.Info(waitMsg, "DNSConnector.Name", dnsConnector.Name, "Remaining time", remainingTimeFormatted)
			return isStatefulSetReady(res), nil
		case *appsv1.Deployment:
			log.Log.Info(waitMsg, "DNSConnector.Name", dnsConnector.Name, "Remaining time", remainingTimeFormatted)
			return isDeploymentReady(res), nil
		case *appsv1.DaemonSet:
			log.Log.Info(waitMsg, "DNSConnector.Name", dnsConnector.Name, "Remaining time", remainingTimeFormatted)
			return isDaemonSetReady(res), nil
		default:
			return false, fmt.Errorf("unsupported resource type: %T", res)
		}
	})
}

// reconcileDelete reconciles if DNSConnector resource has been removed.
func (r *DNSConnectorReconciler) reconcileDelete(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// detect ConfigMap, looks up for the configmap. also ensure that CM has Corefile key. returns CM object
	if err := r.restoreOriginalCorefileCM(ctx, dnsConnector); err != nil {
		log.Log.Error(err, "DNSConnector instance. DNSConnector is being deleted. Restore original corefile CM", "DNSConnector.Name", dnsConnector.Name, "ConfigMap.metadata.name", dnsConnector.Spec.CorednsCM.Name)
		return ctrl.Result{}, err
	}

	// remove configmap from the deployment
	/*
		HERE SOME CODE
	*/
	// Remove finazlizer from DNSConnector
	dnsRecObj := types.NamespacedName{Name: dnsConnector.Name, Namespace: dnsConnector.Namespace}
	clientK8sObj := dnsConnector.DeepCopy()
	log.Log.Info("DNSConnector instance. DNSConnector is being deleted. Removing finalizer from DNSConnector", "DNSConnector.Name", dnsConnector.Name)
	if err := removeFinalizer(ctx, r.Client, dnsRecObj, clientK8sObj, monkalev1alpha1.DnsConnectorsFinalizerName); err != nil {
		log.Log.Error(err, "DNSConnector instance. Failed to delete finalizer", "DNSConnector.Name", dnsConnector.Name)
		return ctrl.Result{}, err
	}

	// Done
	log.Log.Info("DNSConnector instance. The DNSConnector has been deleted.", "DNSConnector.Name", dnsConnector.Name)
	return ctrl.Result{}, nil
}

// setDnsConnectorCondition adds or updates a given condition in the DNSConnector
func setDnsConnectorCondition(dnsConnector *monkalev1alpha1.DNSConnector, status metav1.ConditionStatus, reason string, message string) {
	now := metav1.Now()
	cond := metav1.Condition{
		Type:               string(monkalev1alpha1.ConditionConnectorTypeReady),
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: dnsConnector.Generation,
	}
	meta.SetStatusCondition(&dnsConnector.Status.Conditions, cond)
}

// dnsConnectorUpdateStatus updates the status of the DNSConnector Update Status, only if status has been changed
func (r *DNSConnectorReconciler) dnsConnectorUpdateStatus(ctx context.Context, previous, current *monkalev1alpha1.DNSConnector) error {
	if !equality.Semantic.DeepEqual(previous.Status, current.Status) {
		if err := r.Status().Update(ctx, current); err != nil {
			return fmt.Errorf("failed to update status and condition: %v", err)
		}
	}
	return nil
}

// refreshDNSConnectorResource fetch from kubernetes a new version of DNSConnector resource
func (r *DNSConnectorReconciler) refreshDNSConnectorResource(ctx context.Context, dnsConnector *monkalev1alpha1.DNSConnector) error {
	dnsConnObj := types.NamespacedName{Name: dnsConnector.Name, Namespace: dnsConnector.Namespace}
	clientK8sObj := dnsConnector.DeepCopy()
	if err := getObjFromK8s(ctx, r.Client, dnsConnObj, clientK8sObj); err != nil {
		return fmt.Errorf("failed to refresh DNSConnector resource: %v", err)
	}
	return nil
}

// dnsZoneChangedReconcileRequest requests DNSConnector reconcilation if DNSZone has been created/updated/deleted.
func (r *DNSConnectorReconciler) dnsZoneChangedReconcileRequest(ctx context.Context, dnsZone client.Object) []reconcile.Request {
	_ = log.FromContext(ctx)
	dnsZoneObj, ok := dnsZone.(*monkalev1alpha1.DNSZone)
	if !ok {
		log.Log.Error(nil, "DNSConnector instance. Failed to cast dnsZone to monkalev1alpha1.DNSZone")
		return []reconcile.Request{}
	}
	// delay before trying to reconcile. it avoids false positive logs
	time.Sleep(3 * time.Second)
	log.Log.Info("DNSConnector instance. DNSZone change detected. Requesting reconcilation for the connector", "DNSConnector.Name", dnsZoneObj.Spec.ConnectorName, "DNSZone.Name", dnsZoneObj.Name)
	// create a reconcile request for the associated DNSConnector
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      dnsZoneObj.Spec.ConnectorName,
				Namespace: dnsZoneObj.GetNamespace(),
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSConnectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index DNSZoneConnector Reference name
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &monkalev1alpha1.DNSZone{}, monkalev1alpha1.DnsZoneConnectorIndex, func(rawObj client.Object) []string {
		// Extract the DNSZone name from the DNSConnector Spec
		dnsZone := rawObj.(*monkalev1alpha1.DNSZone)
		if dnsZone.Spec.ConnectorName == "" {
			return nil
		}
		return []string{dnsZone.Spec.ConnectorName}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&monkalev1alpha1.DNSConnector{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&monkalev1alpha1.DNSZone{},
			handler.EnqueueRequestsFromMapFunc(r.dnsZoneChangedReconcileRequest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}
