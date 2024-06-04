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
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

// DNSZoneReconciler reconciles a DNSZone object
type DNSZoneReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=monkale.monkale.io,resources=dnszones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monkale.monkale.io,resources=dnszones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monkale.monkale.io,resources=dnszones/finalizers,verbs=update

// Reconcile is responsible to reconcile DNSZone resource.
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *DNSZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	var dnsZone monkalev1alpha1.DNSZone
	// Fetch DNSZone from kubernetes
	if err := r.Get(ctx, req.NamespacedName, &dnsZone); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else if !apierrors.IsNotFound(err) {
			log.Log.Error(err, "DNSZone instance. Failed to get DNSZone", "DNSZone.Name", dnsZone.Name)
			return ctrl.Result{}, err
		}
	}

	// DNSZone has been just created. Add finalizer.
	if !controllerutil.ContainsFinalizer(&dnsZone, monkalev1alpha1.DnsZonesFinalizerName) {
		dnsZoneObj := types.NamespacedName{Name: dnsZone.Name, Namespace: dnsZone.Namespace}
		clientK8sObj := dnsZone.DeepCopy()
		if err := addFinalizer(ctx, r.Client, dnsZoneObj, clientK8sObj, monkalev1alpha1.DnsZonesFinalizerName); err != nil {
			log.Log.Error(err, "DNSZone instance. Failed to add finalizer", "DNSZone.Name", dnsZone.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// DNSZone has been deleted. Reconcile Delete
	if !dnsZone.DeletionTimestamp.IsZero() {
		log.Log.Info("DNSZone instance. Zone has been removed. Reconciling", "DNSZone.Name", dnsZone.Name)
		return r.reconcileDelete(ctx, &dnsZone)
	}

	// DNSZone has been created or updated. Reconsile create or update
	log.Log.Info("DNSZone instance. Reconciling", "DNSZone.Name", dnsZone.Name)
	return r.reconcileCreateOrUpdate(ctx, &dnsZone)
}

// reconcileDelete reconciles if DNSZone resource has been removed.
func (r *DNSZoneReconciler) reconcileDelete(ctx context.Context, dnsZone *monkalev1alpha1.DNSZone) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	previousState := dnsZone.DeepCopy()
	// Remove finalizer from the CM
	var currentCM corev1.ConfigMap
	cmConnObj := types.NamespacedName{Name: dnsZone.Spec.CMPrefix + dnsZone.Name, Namespace: dnsZone.Namespace}
	cmErr := r.Get(ctx, cmConnObj, &currentCM)

	// Any not "isNotFound" error while fetching ConfigMap
	if cmErr != nil && !apierrors.IsNotFound(cmErr) {
		log.Log.Error(cmErr, "DNSZone instance is being deleted. Error while fetching ConfigMap", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return ctrl.Result{}, cmErr
	} else if apierrors.IsNotFound(cmErr) {
		log.Log.Info("DNSZone instance is being deleted. DNZZone configMap does not exist", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
	} else {
		log.Log.Info("DNSZone instance is being deleted. Removing finalizer from configmap", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		if err := removeFinalizer(ctx, r.Client, cmConnObj, &currentCM, monkalev1alpha1.DnsZonesFinalizerName); err != nil {
			log.Log.Error(err, "DNSZone instance is being deleted. Failed to delete finalizer from Zone CM", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
			return ctrl.Result{}, err
		}
	}

	// Notify all DNSRecords that the zone has been removed
	log.Log.Info("DNSZone instance is being deleted. Notify DNSRecords.", "DNSZone.Name", dnsZone.Name)
	dnsRecordList, err := r.getGoodDnsRecords(ctx, dnsZone)
	if err != nil {
		log.Log.Error(err, "DNSZone instance is being deleted. Notify DNSRecords. Failed to get DNSRecords", "DNSZone.Name", dnsZone.Name)
		message := fmt.Sprintf("Update dnsrecords failure. Preserving the previous version. Failed to get dnsrecords: %s", err)
		setDnsZoneCondition(dnsZone, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonZoneUpdateErr, message)
		if err := r.dnsZoneUpdateStatus(ctx, previousState, dnsZone); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status and condition: %v", err)
		}
		return ctrl.Result{}, err
	}

	for _, dnsRecord := range dnsRecordList.Items {
		// refresh resource
		dnsRecType := types.NamespacedName{Name: dnsRecord.Name, Namespace: dnsRecord.Namespace}
		dnsRecObj := dnsRecord.DeepCopy()
		if err := getObjFromK8s(ctx, r.Client, dnsRecType, dnsRecObj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to refresh DNSRecord resource: %v", err)
		}
		// update resource
		message := fmt.Sprintf("DNSZone has been removed: %s", dnsZone.Name)
		setDnsRecordCondition(dnsRecObj, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonRecordPending, message)
		if err := r.Status().Update(ctx, dnsRecObj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status and condition: %v", err)
		}
	}

	// Remove finazlizer from DNSZone
	dnsZoneObj := types.NamespacedName{Name: dnsZone.Name, Namespace: dnsZone.Namespace}
	clientK8sObj := dnsZone.DeepCopy()
	log.Log.Info("DNSZone instance is being deleted. Removing finalizer from DNSZone", "DNSZone.Name", dnsZone.Name)
	if err := removeFinalizer(ctx, r.Client, dnsZoneObj, clientK8sObj, monkalev1alpha1.DnsZonesFinalizerName); err != nil {
		log.Log.Error(err, "Deleting DNSZone. Failed to delete finalizer", "DNSZone.Name", dnsZone.Name)
		return ctrl.Result{}, err
	}

	// Done
	log.Log.Info("DNSZone instance has deleted.", "DNSZone.Name", dnsZone.Name)
	return ctrl.Result{}, nil
}

// reconcileCreateOrUpdate reconciles if DNSZone resource has been created or updated
func (r *DNSZoneReconciler) reconcileCreateOrUpdate(ctx context.Context, dnsZone *monkalev1alpha1.DNSZone) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	// Get DNSRecords for the Zone.
	log.Log.Info("DNSZone instance. Generate ZoneCM. Fetching DNSRecords", "DNSZone.Name", dnsZone.Name)
	previousState := dnsZone.DeepCopy()
	var records bakedRecords
	dnsRecordList, err := r.getGoodDnsRecords(ctx, dnsZone)
	if err != nil {
		log.Log.Error(err, "DNSZone instance. Generate ZoneCM. Failed to get DNSRecords", "DNSZone.Name", dnsZone.Name)
		message := fmt.Sprintf("Update dnsrecords failure. Preserving the previous version. Failed to get dnsrecords: %s", err)
		setDnsZoneCondition(dnsZone, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonZoneUpdateErr, message)
		if err := r.dnsZoneUpdateStatus(ctx, previousState, dnsZone); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status and condition: %v", err)
		}
		return ctrl.Result{}, err
	} else if len(dnsRecordList.Items) <= 0 {
		// If no records, make it empty. It will generate only SOA and NS 1
		records = bakedRecords{
			count:         0,
			recordsString: "",
		}
	} else {
		// Convert DNSRecords to coredns entries
		records, err = bakeRecords(dnsRecordList)
		if err != nil {
			log.Log.Error(err, "DNSZone instance. Generate ZoneCM. Failed to construct record list for coredns", "DNSZone.Name", dnsZone.Name)
			return ctrl.Result{}, err
		}
	}

	// Construct and Apply zone CM
	if err := r.createOrUpdateZoneCM(ctx, dnsZone, records); err != nil {
		log.Log.Error(err, "DNSZone instance. Generate ZoneCM. Failed to create or update Zone CM", "DNSZone.Name", dnsZone.Name)
		return ctrl.Result{}, err
	}

	// Update DNSRecord status - at this point all records are inserted into the Zonefile.
	for _, dnsRecord := range dnsRecordList.Items {
		// refresh resource
		dnsRecType := types.NamespacedName{Name: dnsRecord.Name, Namespace: dnsRecord.Namespace}
		dnsRecObj := dnsRecord.DeepCopy()
		if err := getObjFromK8s(ctx, r.Client, dnsRecType, dnsRecObj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to refresh DNSRecord resource: %v", err)
		}
		// update resource
		message := fmt.Sprintf("Record has joined to the DNSZone: %s", dnsZone.Name)
		setDnsRecordCondition(dnsRecObj, metav1.ConditionTrue, monkalev1alpha1.ConditionReasonRecordReady, message)
		if err := r.Status().Update(ctx, dnsRecObj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status and condition: %v", err)
		}
	}
	log.Log.Info("DNSZone instance. Generate ZoneCM. Reconciled successfully", "DNSZone.Name", dnsZone.Name)
	return ctrl.Result{}, nil
}

// createOrUpdateZoneCM constructs SOA,NS, fetches DNSrecords, validates the zone and then creates/updates Zone Config Map
func (r *DNSZoneReconciler) createOrUpdateZoneCM(ctx context.Context, dnsZone *monkalev1alpha1.DNSZone, bakedRecords bakedRecords) error {
	_ = log.FromContext(ctx)
	previousState := dnsZone.DeepCopy()
	var currentCM corev1.ConfigMap
	// Get currentCM
	cmConnObj := types.NamespacedName{Name: dnsZone.Spec.CMPrefix + dnsZone.Name, Namespace: dnsZone.Namespace}
	cmErr := r.Get(ctx, cmConnObj, &currentCM)

	// Any not "isNotFound" error while fetching ConfigMap
	if cmErr != nil && !apierrors.IsNotFound(cmErr) && !apierrors.IsAlreadyExists(cmErr) {
		log.Log.Error(cmErr, "DNSZone instance. Reconciling ZoneCM. Error while fetching ConfigMap", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return cmErr
	}

	// Create new serial for the zone
	serialNumber, err := monkalev1alpha1.DNSZoneGenerateSerial()
	if err != nil {
		log.Log.Error(err, "DNSZone instance. Reconciling ZoneCM. Could not generate Serial number", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return err
	}

	// Construct the zone
	log.Log.Info("DNSZone instance. Reconciling ZoneCM. Constructing zone", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
	zone, err := constructZoneFile(dnsZone, bakedRecords.recordsString, serialNumber)
	if err != nil {
		message := fmt.Sprintf("Zone construction failure: %s", err)
		setDnsZoneCondition(dnsZone, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonRecordDegraded, message)
		if err := r.dnsZoneUpdateStatus(ctx, previousState, dnsZone); err != nil {
			return fmt.Errorf("failed to update status and condition: %v", err)
		}
		log.Log.Error(err, "DNSZone instance. Reconciling ZoneCM. Failed to construct zone", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return err
	}

	// Validate zone
	if err := validateRecords(zone); err != nil {
		if err := r.refreshDNSZoneResource(ctx, previousState); err != nil {
			return fmt.Errorf("failed to refresh DNSZone resource: %v", err)
		}
		message := fmt.Sprintf("Zone validation failure. Preserving the previous version. Error: %s", err)
		dnsZone.Status.ValidationPassed = false
		setDnsZoneCondition(dnsZone, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonZoneUpdateErr, message)
		if err := r.dnsZoneUpdateStatus(ctx, previousState, dnsZone); err != nil {
			return fmt.Errorf("failed to update status and condition: %v", err)
		}
		log.Log.Error(err, "DNSZone instance. Reconciling ZoneCM.", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return err
	}

	// Construct the Zone ConfigMap
	upcomingCMAnnotations := map[string]string{"SerialNumber": serialNumber, "DomainName": dnsZone.Spec.Domain, "DNSZoneRef": dnsZone.Name}
	upcomingCM, err := constructZoneConfigMap(cmConnObj.Name, dnsZone, zone, upcomingCMAnnotations)
	if err != nil {
		if err := r.refreshDNSZoneResource(ctx, previousState); err != nil {
			return fmt.Errorf("failed to refresh DNSZone resource: %v", err)
		}
		message := fmt.Sprintf("Zone ConfigMap creation failure: %s", err)
		setDnsZoneCondition(dnsZone, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonZoneUpdateErr, message)
		if err := r.dnsZoneUpdateStatus(ctx, previousState, dnsZone); err != nil {
			return fmt.Errorf("failed to update status and condition: %v", err)
		}
		log.Log.Error(err, "DNSZone instance. Reconciling ZoneCM. Failed to construct zoneCM", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return err
	}

	// Create ConfigMap if does not exist
	// If ConfigMap does not exist -> Create, else Update.
	if apierrors.IsNotFound(cmErr) {
		// Create CM
		log.Log.Info("DNSZone instance. Reconciling ZoneCM. Creating", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		if err := r.Create(ctx, &upcomingCM); err != nil {
			log.Log.Error(err, "DNSZone instance. Reconciling ZoneCM. Failed to create zone", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
			return err
		}
	}

	// ConfigMap exists. Check if update is needed.
	// If not needed, exit, if needed update the set the new status for serialNumber
	same := compareZonefileConfigMaps(&currentCM, &upcomingCM)
	if same {
		log.Log.Info("DNSZone instance. No changes detected")
		return nil
	} else {
		dnsZone.Status.CurrentZoneSerial = serialNumber
	}

	// Update needed. Update ConfigMap
	log.Log.Info("DNSZone instance. Reconciling ZoneCM. Updating", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
	if err := r.Update(ctx, &upcomingCM); err != nil {
		log.Log.Error(err, "DNSZone instance. Reconciling ZoneCM. Failed to update zone configmap", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return err
	}

	// Update DNSZone Status
	if err := r.refreshDNSZoneResource(ctx, previousState); err != nil {
		return fmt.Errorf("failed to refresh DNSRecord resource: %v", err)
	}

	message := fmt.Sprintf("Zone ConfigMap has been created: %s", cmConnObj.Name)
	setDnsZoneCondition(dnsZone, metav1.ConditionTrue, monkalev1alpha1.ConditionReasonZonePending, message)
	dnsZone.Status.RecordCount = bakedRecords.count
	dnsZone.Status.ValidationPassed = true
	dnsZone.Status.Checkpoint = true
	dnsZone.Status.ZoneConfigmap = cmConnObj.Name
	if err := r.dnsZoneUpdateStatus(ctx, previousState, dnsZone); err != nil {
		return fmt.Errorf("failed to update status and condition: %v", err)
	}

	// Add finalizer
	if err := addFinalizer(ctx, r.Client, cmConnObj, &corev1.ConfigMap{}, monkalev1alpha1.DnsZonesFinalizerName); err != nil {
		log.Log.Error(err, "DNSZone instance. Reconciling ZoneCM. Failed to add finalizer", "ConfigMap.metadata.name", cmConnObj.Name, "DNSZone.Name", dnsZone.Name)
		return err
	}

	return nil
}

// setDnsZoneCondition adds or updates a given condition in the ManagedZone status.
func setDnsZoneCondition(dnsZone *monkalev1alpha1.DNSZone, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	cond := metav1.Condition{
		Type:               string(monkalev1alpha1.ConditionZoneTypeReady),
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: dnsZone.Generation,
	}
	meta.SetStatusCondition(&dnsZone.Status.Conditions, cond)
}

// refreshDNSZoneResources fetch from kubernetes a new version of DNSZoneResource
func (r *DNSZoneReconciler) refreshDNSZoneResource(ctx context.Context, dnsZone *monkalev1alpha1.DNSZone) error {
	dnsZoneObj := types.NamespacedName{Name: dnsZone.Name, Namespace: dnsZone.Namespace}
	clientK8sObj := dnsZone.DeepCopy()
	if err := getObjFromK8s(ctx, r.Client, dnsZoneObj, clientK8sObj); err != nil {
		return fmt.Errorf("failed to refresh DNSZone resource: %v", err)
	}
	return nil
}

// dnsZoneUpdateStatus updates the status of the DNSzone Update Status, only if status has been changed. If the only thing that changed
// is LastTransitionTime skip!
func (r *DNSZoneReconciler) dnsZoneUpdateStatus(ctx context.Context, previous, current *monkalev1alpha1.DNSZone) error {
	// avoid resource update if the only thing that changed is the LastTransitionTime
	previousCopy := previous.DeepCopy()
	currentCopy := current.DeepCopy()
	timeNow := metav1.Time{}
	for i := range previousCopy.Status.Conditions {
		previousCopy.Status.Conditions[i].LastTransitionTime = timeNow
	}
	for i := range currentCopy.Status.Conditions {
		currentCopy.Status.Conditions[i].LastTransitionTime = timeNow
	}
	// update status
	if !equality.Semantic.DeepEqual(previousCopy.Status, currentCopy.Status) {
		if err := r.Status().Update(ctx, current); err != nil {
			return fmt.Errorf("failed to update status and condition: %v", err)
		}
	}
	return nil
}

// compareZonefileConfigMaps compares two configmaps with the zonefile.
// during the check it will remove serial number. Returns true if they the same.
func compareZonefileConfigMaps(previousCM, upcomingCM *corev1.ConfigMap) bool {
	previousCMCopy := previousCM.DeepCopy()
	upcomingCMCopy := upcomingCM.DeepCopy()

	// Remove serial number from both conifgMaps
	for k, v := range previousCMCopy.Data {
		previousCMCopy.Data[k] = removeSerialNumber(v)
	}

	for k, v := range upcomingCMCopy.Data {
		upcomingCMCopy.Data[k] = removeSerialNumber(v)
	}

	// Do check
	return equality.Semantic.DeepEqual(previousCMCopy.Data, upcomingCMCopy.Data)
}

// removeSerialNumber used to remove serial number from the zonefile string
func removeSerialNumber(zonefile string) string {
	lines := strings.Split(zonefile, "\n")
	for i, line := range lines {
		if strings.Contains(line, "Serial") {
			// Line for example: " 0525132744     ; Serial"
			lines[i] = strings.Split(line, "\t")[0] + " ; Serial"
		}
	}
	return strings.Join(lines, "\n")
}

// dnsRecordChangedReconcileRequest requests DNSZone reconcilation if DNSRecord has been created/updated/deleted.
func (r *DNSZoneReconciler) dnsRecordChangedReconcileRequest(ctx context.Context, dnsRecord client.Object) []reconcile.Request {
	_ = log.FromContext(ctx)
	dnsRecordObj, ok := dnsRecord.(*monkalev1alpha1.DNSRecord)
	if !ok {
		log.Log.Error(nil, "DNSZone instance. Failed to cast dnsRecord to monkalev1alpha1.DNSRecord")
		return []reconcile.Request{}
	}
	// delay before trying to reconcile. it avoids false positive logs
	time.Sleep(3 * time.Second)
	log.Log.Info("DNSZone instance. DNSRecord change detected. Requesting reconcilation for the zone", "DNSZone.Name", dnsRecordObj.Spec.DNSZoneRef.Name)
	// Create a reconcile request for the associated DNSZone
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      dnsRecordObj.Spec.DNSZoneRef.Name,
				Namespace: dnsRecordObj.GetNamespace(),
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
// https://book.kubebuilder.io/reference/watching-resources/externally-managed
func (r *DNSZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// index DNSZone Reference name
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &monkalev1alpha1.DNSRecord{}, monkalev1alpha1.DnsRecordIndex, func(rawObj client.Object) []string {
		// Extract the DNSZone name from the DNSRecord Spec
		dnsRecord := rawObj.(*monkalev1alpha1.DNSRecord)
		if dnsRecord.Spec.DNSZoneRef.Name == "" {
			return nil
		}
		return []string{dnsRecord.Spec.DNSZoneRef.Name}
	}); err != nil {
		return err
	}

	// DNSZone is primary resource, DNSRecord is secondary.
	return ctrl.NewControllerManagedBy(mgr).
		For(&monkalev1alpha1.DNSZone{}).WithEventFilter(predicate.GenerationChangedPredicate{}).
		Watches(
			&monkalev1alpha1.DNSRecord{},
			handler.EnqueueRequestsFromMapFunc(r.dnsRecordChangedReconcileRequest),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
