package controller

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/miekg/dns"
	monkalev1alpha1 "github.com/monkale.io/coredns-manager-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const recordHasBeenConstructedMsg string = "Record has been constructed. Awaiting for the dnszone controller to pick up the record"

// handleGenericRecord - handling of all dns records are basically the same.
func (r *DNSRecordReconciler) handleGenericRecord(ctx context.Context, dnsRecord *monkalev1alpha1.DNSRecord) (string, error) {
	previousState := dnsRecord.DeepCopy()
	// construct
	record, err := constructRecord(*dnsRecord)
	if err != nil {
		return "", fmt.Errorf("handle Record Error: %v", err)
	}

	// validate record and refresh&update status
	if err := validateRecords(record); err != nil {
		if err := r.refreshDNSRecordResource(ctx, previousState); err != nil {
			return "", fmt.Errorf("failed to refresh DNSRecord resource: %v", err)
		}
		message := fmt.Sprintf("Record validation failure: %s", err)
		setDnsRecordCondition(dnsRecord, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonRecordDegraded, message)
		dnsRecord.Status.GeneratedRecord = record
		dnsRecord.Status.ValidationPassed = false
		if err := r.dnsRecordUpdateStatus(ctx, previousState, dnsRecord); err != nil {
			return "", fmt.Errorf("failed to update status and condition: %v", err)
		}
		return "", fmt.Errorf("record validation failure: %v", err)
	}

	// refresh&update status
	if err := r.refreshDNSRecordResource(ctx, previousState); err != nil {
		return "", fmt.Errorf("failed to refresh DNSRecord resource: %v", err)
	}
	setDnsRecordCondition(dnsRecord, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonRecordPending, recordHasBeenConstructedMsg)
	dnsRecord.Status.GeneratedRecord = record
	dnsRecord.Status.ValidationPassed = true
	if err := r.dnsRecordUpdateStatus(ctx, previousState, dnsRecord); err != nil {
		return "", fmt.Errorf("failed to update status and condition: %v", err)
	}
	return record, nil
}

// constructRecord templates DNS record according to RFC1035.
func constructRecord(dnsRecord monkalev1alpha1.DNSRecord) (string, error) {
	dnsRec := dnsRecord.DeepCopy()

	recordTmpl := `{{ .Spec.Record.Name }}{{ if .Spec.Record.TTL }} {{ .Spec.Record.TTL }}{{ end }} IN {{ .Spec.Record.Type }} {{ .Spec.Record.Value -}}`
	tmpl, err := template.New("record").Parse(recordTmpl)
	if err != nil {
		return "", fmt.Errorf("could not template DNS Record: %v", err)
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, dnsRec); err != nil {
		return "", fmt.Errorf("could not template DNS Record: %v", err)
	}
	return result.String(), nil
}

// validateRecords performs syntax check of DNSRecords provided as a string.
func validateRecords(records string) error {
	recordReader := strings.NewReader(records)
	recordParser := dns.NewZoneParser(recordReader, ".", "")
	for {
		_, ok := recordParser.Next()
		if !ok {
			break
		}
		if err := recordParser.Err(); err != nil {
			return fmt.Errorf("error parsing record: %v", err)
		}
	}
	// Check for any final errors
	if err := recordParser.Err(); err != nil {
		return fmt.Errorf("error parsing records: %v", err)
	}
	return nil
}
