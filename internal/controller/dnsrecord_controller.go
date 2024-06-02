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

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	monkalev1alpha1 "github.com/monkale.io/coredns-manager-operator/api/v1alpha1"
)

// DNSRecordReconciler reconciles a DNSRecord object
type DNSRecordReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monkale.monkale.io,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monkale.monkale.io,resources=dnsrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monkale.monkale.io,resources=dnsrecords/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *DNSRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	var dnsRecord monkalev1alpha1.DNSRecord

	// Fetch DNSRecord from kubernetes
	if err := r.Get(ctx, req.NamespacedName, &dnsRecord); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else if !apierrors.IsNotFound(err) {
			log.Log.Error(err, "DNSRecord instance. Failed to get DNSRecord", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	}

	// DNSRecord has been just created. Add finalizer.
	if !controllerutil.ContainsFinalizer(&dnsRecord, monkalev1alpha1.DnsRecorsFinalizerName) {
		dnsRecObj := types.NamespacedName{Name: dnsRecord.Name, Namespace: dnsRecord.Namespace}
		clientK8sObj := dnsRecord.DeepCopy()
		if err := addFinalizer(ctx, r.Client, dnsRecObj, clientK8sObj, monkalev1alpha1.DnsRecorsFinalizerName); err != nil {
			log.Log.Error(err, "DNSRecord instance. Failed to add finalizer", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// DNSRecord has been deleted. Reconcile Delete
	if !dnsRecord.DeletionTimestamp.IsZero() {
		log.Log.Info("DNSRecord instance. Record is being deleted", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
		return r.reconcileDelete(ctx, &dnsRecord)
	}

	// DNSRecord has been created or updated. Reconsile create or update
	log.Log.Info("DNSRecord instance. Reconciling", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
	return r.reconcileDNSRecord(ctx, &dnsRecord)
}

// reconcileDelete reconciles if DNSRecord resource has been removed.
func (r *DNSRecordReconciler) reconcileDelete(ctx context.Context, dnsRecord *monkalev1alpha1.DNSRecord) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	// Remove finazlizer from DNSRecord
	dnsRecObj := types.NamespacedName{Name: dnsRecord.Name, Namespace: dnsRecord.Namespace}
	clientK8sObj := dnsRecord.DeepCopy()
	log.Log.Info("DNSRecord instance. DNSRecord is being deleted. Removing finalizer from DNSRecord", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
	if err := removeFinalizer(ctx, r.Client, dnsRecObj, clientK8sObj, monkalev1alpha1.DnsRecorsFinalizerName); err != nil {
		log.Log.Error(err, "DNSRecord instance. Failed to delete finalizer", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
		return ctrl.Result{}, err
	}

	// Done
	log.Log.Info("DNSRecord instance. The DNSRecord has been deleted.", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
	return ctrl.Result{}, nil
}

// reconcileDNSRecord - processes DNSRecord. It generates the entry for the zonefile, performs syntax check and updates Status.
func (r *DNSRecordReconciler) reconcileDNSRecord(ctx context.Context, dnsRecord *monkalev1alpha1.DNSRecord) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	recordType := dnsRecord.Spec.Record.Type
	switch recordType {
	case "A":
		_, err := r.handleARecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
		// ctrl.Result{RequeueAfter: time.Duration}
	case "AAAA":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "CNAME":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "MX":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "TXT":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "NS":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "PTR":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "SRV":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "CAA":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "DNSKEY":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "DS":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "NAPTR":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "RRSIG":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "DNAME":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	case "HINFO":
		_, err := r.handleGenericRecord(ctx, dnsRecord)
		if err != nil {
			log.Log.Error(err, "DNSRecord instance. Proccess Record Failure", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
			return ctrl.Result{}, err
		}
	default:
		err := errors.New("unknown redord type: " + recordType)
		log.Log.Error(err, "DNSRecord instance. Bad record Type", "DNSRecord.Name", dnsRecord.Name, "DNSZone.Name", dnsRecord.Spec.DNSZoneRef.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// setDnsRecordCondition adds or updates a given condition in the DNSRecord status.
func setDnsRecordCondition(dnsRecord *monkalev1alpha1.DNSRecord, status metav1.ConditionStatus, reason string, message string) {
	now := metav1.Now()
	cond := metav1.Condition{
		Type:               string(monkalev1alpha1.ConditionRecordTypeReady),
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: dnsRecord.Generation,
	}
	meta.SetStatusCondition(&dnsRecord.Status.Conditions, cond)
}

// refreshDNSRecordResource fetch from kubernetes a new version of DNSRecordResource
func (r *DNSRecordReconciler) refreshDNSRecordResource(ctx context.Context, dnsRecord *monkalev1alpha1.DNSRecord) error {
	dnsRecObj := types.NamespacedName{Name: dnsRecord.Name, Namespace: dnsRecord.Namespace}
	clientK8sObj := dnsRecord.DeepCopy()
	if err := getObjFromK8s(ctx, r.Client, dnsRecObj, clientK8sObj); err != nil {
		return fmt.Errorf("failed to refresh DNSRecord resource: %v", err)
	}
	return nil
}

// dnsRecordUpdateStatus updates the status of the DNSRecord Update Status, only if status has been changed
func (r *DNSRecordReconciler) dnsRecordUpdateStatus(ctx context.Context, previous, current *monkalev1alpha1.DNSRecord) error {
	if !equality.Semantic.DeepEqual(previous.Status, current.Status) {
		if err := r.Status().Update(ctx, current); err != nil {
			return fmt.Errorf("failed to update status and condition: %v", err)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
// Watches https://book.kubebuilder.io/reference/watching-resources/externally-managed
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monkalev1alpha1.DNSRecord{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
