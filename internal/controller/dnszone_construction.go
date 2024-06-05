package controller

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monkalev1alpha1 "github.com/monkale.io/coredns-manager-operator/api/v1alpha1"
)

// bakedRecords represents the records that are members of the Zonefile.
type bakedRecords struct {
	count         int
	recordsString string
}

// constructZoneFile - constructs and validates Zone.
func constructZoneFile(dnsZone *monkalev1alpha1.DNSZone, records string, serialNumber string) (string, error) {
	var newZoneHeader string
	newZoneHeaderValues := monkalev1alpha1.DNSZoneHeader{
		DomainName:        monkalev1alpha1.EnsureFQDN(dnsZone.Spec.Domain),
		PrimaryNSHostname: dnsZone.Spec.PrimaryNS.Hostname,
		PrimaryNSIp:       dnsZone.Spec.PrimaryNS.IPAddress,
		PrimaryNSType:     dnsZone.Spec.PrimaryNS.RecordType,
		RespPerson:        dnsZone.Spec.RespPersonEmail,
		ZoneTTL:           dnsZone.Spec.TTL,
		Serial:            serialNumber,
		Refresh:           dnsZone.Spec.RefreshRate,
		Retry:             dnsZone.Spec.RetryInterval,
		Expire:            dnsZone.Spec.ExpireTime,
		MinimumTTL:        dnsZone.Spec.MinimumTTL,
	}
	newZoneHeader, err := templateZoneHeader(newZoneHeaderValues)
	if err != nil {
		return "", fmt.Errorf("unable to template the zoneHeader: %v", err)
	}
	zonefileContent := newZoneHeader + records
	return zonefileContent, nil
}

// constructZoneConfigMap constructs config map for the Zone
func constructZoneConfigMap(cmObj string, dnsZone *monkalev1alpha1.DNSZone, zonefileContent string, upcomingCMAnnotations map[string]string) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cmObj,
			Namespace:   dnsZone.ObjectMeta.Namespace,
			Labels:      map[string]string{"app": "coredns-addon-operator"},
			Annotations: upcomingCMAnnotations,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: dnsZone.APIVersion,
					Kind:       dnsZone.Kind,
					Name:       dnsZone.Name,
					UID:        dnsZone.UID,
				},
			},
		},
		Data: map[string]string{
			monkalev1alpha1.EnsureFQDN(dnsZone.Spec.Domain) + "zone": zonefileContent,
		},
	}
	return cm, nil
}

// templateZoneHeader builds Zone header: SOA and first NS A record
func templateZoneHeader(header monkalev1alpha1.DNSZoneHeader) (string, error) {
	zoneTmpl := `$ORIGIN {{.DomainName}}
$TTL {{ .ZoneTTL }}s
@ IN SOA {{.PrimaryNSHostname}}.{{.DomainName}} {{.RespPerson}}. (
	{{.Serial}}     ; Serial
	{{.Refresh}}    ; Refresh
	{{.Retry}}      ; Retry
	{{.Expire}}     ; Expire
	{{.MinimumTTL}} ; Minimum TTL
)
@ IN NS {{.PrimaryNSHostname}}.{{.DomainName}}
{{.PrimaryNSHostname}} IN {{.PrimaryNSType}} {{.PrimaryNSIp}}
`
	tmpl, err := template.New("HEADER").Parse(zoneTmpl)
	if err != nil {
		return "", fmt.Errorf("could not template SOA or NS: %v", err)
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, header); err != nil {
		return "", fmt.Errorf("could not template SOA or NS: %v", err)
	}
	zoneHeader := result.String()
	return zoneHeader, nil
}

// getGoodDnsRecords fetches all DNSRecords. fails if bad records found
func (r *DNSZoneReconciler) getGoodDnsRecords(ctx context.Context, dnsZone *monkalev1alpha1.DNSZone) (monkalev1alpha1.DNSRecordList, error) {
	records := &monkalev1alpha1.DNSRecordList{}
	// list all DNSRecord with the same DNSZone.
	fieldSelector := fields.OneTermEqualSelector(monkalev1alpha1.DnsRecordIndex, dnsZone.Name)
	listOps := &client.ListOptions{
		FieldSelector: fieldSelector,
		Namespace:     dnsZone.Namespace,
	}
	if err := r.List(ctx, records, listOps); err != nil {
		return monkalev1alpha1.DNSRecordList{}, fmt.Errorf("could not list DNSRecords: %v", err)
	}

	// get only good records
	goodRecords := monkalev1alpha1.DNSRecordList{}
	for _, record := range records.Items {
		if record.Status.ValidationPassed {
			goodRecords.Items = append(goodRecords.Items, record)
		}
	}

	// sort records a-z
	sort.Slice(goodRecords.Items, func(i, j int) bool {
		return goodRecords.Items[i].Name < goodRecords.Items[j].Name
	})

	return goodRecords, nil
}

// bakeRecords bakes DNSRecords into the single Zone file compatible string.
func bakeRecords(dnsRecords monkalev1alpha1.DNSRecordList) (bakedRecords, error) {
	records := dnsRecords.Items
	var sb strings.Builder
	for i, record := range records {
		sb.WriteString(record.Status.GeneratedRecord)
		if i < len(records)-1 {
			sb.WriteString("\n")
		}
	}
	linesCount := len(strings.Split(sb.String(), "\n"))
	corednsEntries := bakedRecords{
		count:         linesCount,
		recordsString: sb.String(),
	}

	return corednsEntries, nil
}
