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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionZoneTypeReady         string = "Ready"               // ConditionZoneTypeReady is used to update condition type
	ConditionReasonZoneActive      string = "Active"              // ConditionReasonZoneActive represents state of the DNSZone
	ConditionReasonZonePending     string = "Pending"             // ConditionReasonRecordPending represents state of the DNSZone
	ConditionReasonZoneUpdateErr   string = "UpdateError"         // ConditionReasonZoneUpdateErr represents state of the DNSZone
	ConditionReasonZoneNoConnector string = "NoConnector"         // ConditionReasonZoneNoConnector represents state of the DNSZone in which the zone has no connector
	ConditionReasonZoneUnknown     string = "Unknown"             // ConditionReasonZoneUnknown string = "Unknown"
	DnsZonesFinalizerName          string = "dnszones/finalizers" // DnsZonesFinalizerName is finalizer used by DNSZone controller
	DnsZoneConnectorIndex          string = "spec.ConnectorName"  // DnsZoneConnectorIndex  is used for indexing and watching
)

// primaryNS defines the primary Nameserver for the DNSZone.
// If the operator used to manage zones in offline deployments
// the "name" could be negligenced.
type PrimaryNS struct {
	// hostname is the server name of the primary name server for this zone.
	// The default value is "ns1".
	// +kubebuilder:default:=ns1
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?)*$`
	Hostname string `json:"hostname"`

	// ipAddress defines IP address to the dns server where the zone hosted.
	// If the zone is managed by k8s coredns specify IP of kubernetes lb/node.
	// Provide either ipv4, or ipv6.
	IPAddress string `json:"ipAddress"`

	// recordType defines the type of the record to be created for the NS's A record.
	// In case of ipv6 set it to "AAAA".
	// The default value is "A".
	// +kubebuilder:default:=A
	// +kubebuilder:validation:Enum=A;AAAA;
	RecordType string `json:"recordType"`
}

// DNSZoneSpec defines the desired state of DNSZone.
// DNSZoneSpec creates the new zone file with the SOA record.
// DNSZoneSpec creates DNSRecords of type NS.
type DNSZoneSpec struct {
	// cmPrefix specifies the prefix for the zone file configmap.
	// The default value is coredns-zone-.
	// The CM Name format is "prefix" + "metadata.name",
	// +kubebuilder:default:=coredns-zone-
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(-)?$`
	CMPrefix string `json:"cmPrefix"`

	// domain specifies domain in which DNSRecors are valid.
	// +kubebuilder:validation:Required
	Domain string `json:"domain"`

	// primaryNS defines NS record for the zone, and its A/AAAA record.
	// +kubebuilder:validation:Required
	PrimaryNS *PrimaryNS `json:"primaryNS"`

	// respPersonEmail is responsible party's email for the domain.
	// Typically formatted as admin@example.com but represented with a dot (.)
	// instead of an at (@) in DNS records. The first dot separates the user name from the domain.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,6}$`
	RespPersonEmail string `json:"respPersonEmail"`

	// ttl specified default Time to Lieve for the zone's records, indicates how long
	// these records should be cached by DNS resolvers.
	// The default value is 86400 seconds (24 hours)
	// +kubebuilder:default:=86400
	// +kubebuilder:validation:Optional
	TTL uint `json:"ttl,omitempty"`

	// refreshRate defines the time a secondary DNS server waits before querying the primary DNS
	// server to check for updates.
	// If the zone file has changed, secondary servers will refresh their data.
	// these records should be cached by DNS resolvers.
	// The default value is 7200 seconds (2 hours)
	// +kubebuilder:default:=7200
	// +kubebuilder:validation:Optional
	RefreshRate uint `json:"refreshRate,omitempty"`

	// retryInterval defines how long secondary server failed should wait
	// before trying again to reconnect to the primary again.
	// The default value is 3600 seconds (1 hour)
	// +kubebuilder:default:=3600
	// +kubebuilder:validation:Optional
	RetryInterval uint `json:"retryInterval,omitempty"`

	// expireTime defines how long the secondary server should wait
	// before discarding the zone data if it cannot reach the primary server.
	// The default value is 1209600 seconds (2 weeks)
	// +kubebuilder:default:=1209600
	// +kubebuilder:validation:Optional
	ExpireTime uint `json:"expireTime,omitempty"`

	// minimumTTL  is the minimum amount of time that should be allowed for caching the DNS records.
	// If individual records do not specify a TTL, this value should be used.
	// The default value is 86400 seconds (24 hours)
	// +kubebuilder:default:=86400
	// +kubebuilder:validation:Optional
	MinimumTTL uint `json:"minimumTTL,omitempty"`

	// connectorName is the pointer to the DNSConnector Resource.
	// Must contain the name of the DNSConnector Resource.
	// +kubebuilder:validation:Required
	ConnectorName string `json:"connectorName,omitempty"`
}

// DNSZoneStatus defines the observed state of DNSZone
type DNSZoneStatus struct {
	// conditions indidicate the status of a DNSZone.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// currentZoneSerial is a version number that changes update of the zone file,
	// signaling to secondary DNS servers when they should synchronize their data.
	// In our reality we use it to represent the zone file version.
	// Zone Serial implemented as time now formatted to MMDDHHMMSS.
	// Zone Serial represents the current version of the zone file.
	// +optional
	// +kubebuilder:default:="000000001"
	CurrentZoneSerial string `json:"currentZoneSerial,omitempty"`

	// recordCount is the number of records in the zone.
	// Does not include SOA and NS.
	// +optional
	// +kubebuilder:default:=0
	RecordCount int `json:"recordCount,omitempty"`

	// validationPassed displays whether the zonefile passed syntax validation check
	ValidationPassed bool `json:"validationPassed,omitempty"`

	// zoneConfigmap displays the name of the generated zone config map
	ZoneConfigmap string `json:"zoneConfigmap,omitempty"`

	// checkpoint flag indicates whether the DNSZone was previously active.
	// This flag is used to instruct the DNSConnector to preserve the old version of the DNSZone
	// in case the update process encounters an issue.
	Checkpoint bool `json:"checkpoint,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Domain Name",type="string",JSONPath=".spec.domain",description="Domain name"
//+kubebuilder:printcolumn:name="Record Count",type="integer",JSONPath=".status.recordCount",description="Record Count. Without SOA and First NS"
//+kubebuilder:printcolumn:name="Last Change",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].lastTransitionTime",description="Last Change"
//+kubebuilder:printcolumn:name="Current Serial",type="string",JSONPath=".status.currentZoneSerial",description="Represents the current version of the zonefile"
//+kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].reason",description="DNSZone state"

// DNSZone is the Schema for the dnszones API
type DNSZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSZoneSpec   `json:"spec,omitempty"`
	Status DNSZoneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DNSZoneList contains a list of DNSZone
type DNSZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSZone `json:"items"`
}

// DNSZoneHeader represents the minimal Zonefile: SOA + First NS records.
// values needed to define the SOA, its NS and A records.
type DNSZoneHeader struct {
	DomainName        string // Zone origin
	PrimaryNSHostname string // Primary nameserver hostname
	PrimaryNSIp       string // Primary nameserver IP
	PrimaryNSType     string // Primary nameserver record type: A or AAAA
	RespPerson        string // Responsible person's email
	Serial            string // Serial number
	ZoneTTL           uint   // Zone ttl
	Refresh           uint   // Refresh time
	Retry             uint   // Retry time
	Expire            uint   // Expire time
	MinimumTTL        uint   // Minimum TTL
}

// DNSZoneGenerateSerial generates serial number in format using time formatted to MMDDHHMMSS
func DNSZoneGenerateSerial() (string, error) {
	now := time.Now()
	timePart := now.Format("0102150405")

	// Ensure that the final serial number is a string
	return timePart, nil
}

func init() {
	SchemeBuilder.Register(&DNSZone{}, &DNSZoneList{})
}
