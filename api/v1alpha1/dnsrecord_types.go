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

package v1alpha1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionRecordTypeReady      string = "Ready"                    // ConditionRecordTypeReady is used to update condition type
	ConditionReasonRecordReady    string = "Ready"                    // ConditionReasonRecordReady represents state of the DNSRecord
	ConditionReasonRecordPending  string = "Pending"                  // ConditionReasonRecordPending represents state of the DNSRecord
	ConditionReasonRecordDegraded string = "Degraded"                 // ConditionReasonRecordDegraded represents state of the DNSRecord
	ConditionReasonRecordUnknown  string = "Unknown"                  // ConditionReasonRecordUnknown represents state of the DNSRecord
	DnsRecorsFinalizerName        string = "dnsrecords/finalizers"    // DnsRecorsFinalizerName is finalizer used by DNSRecord controller
	DnsRecordIndex                string = ".spec.dnsZoneRef.name"    // DnsRecordIndex is used for indexing and watching
	ValidationPassedIndex         string = ".status.ValidationPassed" // ValidationPassedIndex is used for indexing and watching
)

// Record defines DNS record.
type Record struct {
	// name specifes the record name.
	Name string `json:"name"`

	// value is a value of the record
	Value string `json:"value"`

	// type is a record type according to RFC1035.
	// Supported types: A;AAAA;CNAME;MX;TXT;NS;PTR;SRV;CAA;DNSKEY;DS;NAPTR;RRSIG;DNAME;HINFO;
	// +kubebuilder:validation:Enum=A;AAAA;CNAME;MX;TXT;NS;PTR;SRV;CAA;DNSKEY;DS;NAPTR;RRSIG;DNAME;HINFO;
	Type string `json:"type"`

	// ttl is time to live, which tells how ling this record can be cached.
	// if not set, the default is minimumTTL value in the SOA record.
	// +kubebuilder:validation:Optional
	TTL string `json:"ttl,omitempty"`
}

type DNSRecordSpec struct {
	// Record defines the desired DNS record.
	Record *Record `json:"record"`

	// dnsZoneRef is a reference to a DNSZone instance to which this record will publish its endpoints.
	DNSZoneRef *corev1.ObjectReference `json:"dnsZoneRef"`
}

// DNSRecordStatus defines the observed state of DNSRecord.
type DNSRecordStatus struct {
	// conditions indidicate the status of a DNSRecord.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// validationPassed displays whether the record passed syntax validation check
	ValidationPassed bool `json:"validationPassed,omitempty"`

	// generatedRecord displayes the generated dns record.
	GeneratedRecord string `json:"generatedRecord,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Record Name",type="string",JSONPath=".spec.record.name",description="Record name"
//+kubebuilder:printcolumn:name="Record Type",type="string",JSONPath=".spec.record.type",description="Record type"
//+kubebuilder:printcolumn:name="Record Value",type="string",JSONPath=".spec.record.value",description="Record value"
//+kubebuilder:printcolumn:name="Zone Reference",type="string",JSONPath=".spec.dnsZoneRef.name",description="Reference to the zone"
//+kubebuilder:printcolumn:name="Last Change",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",description="Last Change"
//+kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason",description="DNSRecord State"

// DNSRecord is the Schema for the dnsrecords API
type DNSRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSRecordSpec   `json:"spec,omitempty"`
	Status DNSRecordStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DNSRecordList contains a list of DNSRecord
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSRecord `json:"items"`
}

// EnsureFQDN ensures that the given name is a fully qualified domain name.
func EnsureFQDN(name string) string {
	if !strings.HasSuffix(name, ".") {
		return name + "."
	}
	return name
}

func init() {
	SchemeBuilder.Register(&DNSRecord{}, &DNSRecordList{})
}
