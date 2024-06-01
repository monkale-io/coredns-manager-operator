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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CorednsOriginalConfBkpSuffix      string = "-original-configmap"      // CorednsOriginalConfBkpSuffix suffix that will be used to create a copy of the original coredns conf
	ConditionConnectorTypeReady       string = "Ready"                    // ConditionConnectorTypeReady is used to update condition type
	ConditionReasonConnectorActive    string = "Active"                   // ConditionReasonConnectorActive represents state of the DNSConnector
	ConditionReasonConnectorError     string = "Error"                    // ConditionReasonConnectorError represents the error state of the DNSConnector
	ConditionReasonConnectorUpdating  string = "Updating"                 // ConditionReasonConnectorError represents the
	ConditionReasonConnectorUpdateErr string = "UpdateError"              // ConditionReasonConnectorUpdateErr represents state of the DNSConnector
	ConditionReasonConnectorUnknown   string = "Unknown"                  // ConditionReasonConnectorUnknown string = "Unknown"
	DnsConnectorsFinalizerName        string = "dnsconnectors/finalizers" // DnsConnectorsFinalizerName is finalizer used by DNSConnector controller
)

type CoreDNSConfigMap struct {
	// name is the name of the CoreDNS ConfigMap that contains Corefile.
	// The default value is "coredns"
	// +kubebuilder:default:=coredns
	// +kubebuilder:validation:Optional

	Name string `json:"name"`
	// corefileKey specifies the key whose value is the Corefile. Typically, this key is "Corefile".
	// The default value is "Corefile"
	// +kubebuilder:default:=Corefile
	// +kubebuilder:validation:Optional
	CorefileKey string `json:"corefileKey"`
}

// CoreDNSDeploymentType defines the desired type and name of the CoreDNS resource
type CoreDNSDeploymentType struct {
	// type of the CoreDNS resource (e.g., Deployment, StatefulSet, DaemonSet, ReplicaSet, Pod)
	// +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet
	Type string `json:"type"`

	// name specifies the name of the CoreDNS resource.
	// This field is optional if Type is Pod and a LabelSelector is specified.
	Name string `json:"name,omitempty"`

	// zonefilesMountDir specifies the mountPath for zonefiles.
	// Default value is /opt/coredns.
	// +kubebuilder:default:=/opt/coredns
	// +kubebuilder:validation:Pattern=`^(/[^/]+)+$`
	// +kubebuilder:validation:Optional
	ZoneFileMountDir string `json:"zonefilesMountDir"`
}

// DNSConnectorSpec defines the desired state of DNSConnector
type DNSConnectorSpec struct {
	// waitForUpdateTimeout specifies how long the DNSConnector for coredns to complete update.
	// if coredns deployment haven't complete the update, the controller will perform rollback.
	// The default value is 300 seconds (5 min)
	// +kubebuilder:default:=300
	WaitForUpdateTimeout int `json:"waitForUpdateTimeout"`

	// corednsCM is the name of the CoreDNS ConfigMap.
	CorednsCM CoreDNSConfigMap `json:"corednsCM"`

	// corednsDeployment specifies the CoreDNS deployment type and name or labels
	CorednsDeployment CoreDNSDeploymentType `json:"corednsDeployment"`

	// corednsZoneEnaledPlugins is list of enabled coredns plugins.
	// https://coredns.io/plugins. The most useful plugins are:
	// errors - prints errors to stdout; log - prints queries to stdout.
	// +kubebuilder:validation:Optional
	CorednsZoneEnaledPlugins []string `json:"corednsZoneEnaledPlugins"`
}

// ProvisionedDNSZone used to display the status of the zones provisioned to the Coredns
type ProvisionedDNSZone struct {
	Name         string `json:"name"`
	Domain       string `json:"domain"`
	SerialNumber string `json:"serialNumber"`
}

// DNSConnectorStatus defines the observed state of DNSConnector
type DNSConnectorStatus struct {
	// conditions indidicate the status of a DNSZone.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// provisionedZones maps domain names to their serial numbers.
	// +optional
	ProvisionedDNSZones []ProvisionedDNSZone `json:"provisionedZones,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Last Change",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].lastTransitionTime",description="Last Change"
//+kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].reason",description="The current state"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description="The current state"

// DNSConnector is the Schema for the dnsconnectors API
type DNSConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSConnectorSpec   `json:"spec,omitempty"`
	Status DNSConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DNSConnectorList contains a list of DNSConnector
type DNSConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSConnector `json:"items"`
}

// AssertCorednsDeploymentType used to validate and set the resource type of Coredns deployment. Return client.Object.
func AssertCorednsDeploymentType(resourceType string) (client.Object, error) {
	var resource client.Object
	switch resourceType {
	case "Deployment":
		resource = &appsv1.Deployment{}
	case "StatefulSet":
		resource = &appsv1.StatefulSet{}
	case "DaemonSet":
		resource = &appsv1.DaemonSet{}
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
	return resource, nil
}

func init() {
	SchemeBuilder.Register(&DNSConnector{}, &DNSConnectorList{})
}
