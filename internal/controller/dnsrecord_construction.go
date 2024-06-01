package controller

import (
	"bytes"
	"context"
	"errors"
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

// handleGenericRecordWithNamedValue handle record that in value contains FQDN. This function ensures
// that the value as well as the domain is fqdn.
func (r *DNSRecordReconciler) handleGenericRecordWithNamedValue(ctx context.Context, dnsRecord *monkalev1alpha1.DNSRecord) (string, error) {
	dnsRec := dnsRecord.DeepCopy()
	dnsRec.Spec.Record.Name = monkalev1alpha1.EnsureFQDN(dnsRecord.Spec.Record.Name)
	dnsRec.Spec.Record.Value = monkalev1alpha1.EnsureFQDN(dnsRecord.Spec.Record.Value)
	record, err := r.handleGenericRecord(ctx, dnsRec)
	if err != nil {
		return "", fmt.Errorf("handle ARecord Error: %v", err)
	}
	return record, nil
}

// handleARecord handler A record + AutoPTR creation.
func (r *DNSRecordReconciler) handleARecord(ctx context.Context, dnsRecord *monkalev1alpha1.DNSRecord) (string, error) {
	var records string

	// Create A Record is the same as handleGenericRecord, but users might want to use auto PTR.
	aRecord, err := r.handleGenericRecord(ctx, dnsRecord)
	if err != nil {
		return "", fmt.Errorf("handle ARecord Error: %v", err)
	}
	records = aRecord

	// Construct and validate Auto PTR
	if dnsRecord.Spec.Record.SetPTR && dnsRecord.Spec.Record.Type == "A" {
		// create
		ptrRec, err := constructAutoIPv4PTR(*dnsRecord)
		if err != nil {
			return "", fmt.Errorf("handle AutoPTR Error: %v", err)
		}
		// validate
		if err := validateRecords(ptrRec); err != nil {
			return "", fmt.Errorf("handle ARecord Error: %v", err)
		}
		records = aRecord + "\n" + ptrRec
	}

	// refresh&update condition and status
	if err := r.refreshDNSRecordResource(ctx, dnsRecord); err != nil {
		return "", fmt.Errorf("failed to refresh DNSRecord: %v", err)
	}
	previousState := dnsRecord.DeepCopy()
	setDnsRecordCondition(dnsRecord, metav1.ConditionFalse, monkalev1alpha1.ConditionReasonRecordPending, recordHasBeenConstructedMsg)
	dnsRecord.Status.GeneratedRecord = records
	dnsRecord.Status.ValidationPassed = true
	dnsRecord.Status.AutoIPv4PTR = dnsRecord.Spec.Record.SetPTR
	if err := r.dnsRecordUpdateStatus(ctx, previousState, dnsRecord); err != nil {
		return "", fmt.Errorf("failed to update status and condition: %v", err)
	}
	return records, nil
}

// constructRecord templates DNS record according to RFC1035.
func constructRecord(dnsRecord monkalev1alpha1.DNSRecord) (string, error) {
	dnsRec := dnsRecord.DeepCopy()
	dnsRec.Spec.Record.Name = monkalev1alpha1.EnsureFQDN(dnsRecord.Spec.Record.Name)

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
	recordParser := dns.NewZoneParser(recordReader, "", "")
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

// constructAutoIPv4PTR autogenerates PTR for record A.
func constructAutoIPv4PTR(dnsRecord monkalev1alpha1.DNSRecord) (string, error) {
	if !dnsRecord.Spec.Record.SetPTR || dnsRecord.Spec.Record.Type != "A" {
		err := errors.New("set PTR is not requested or Record type is not A")
		return "", fmt.Errorf("wrong function usage constructAutoIPv4PTR: %v", err)
	}
	recordTmpl := `{{ ptrRecord .Spec.Record.Value .Spec.Record.Name -}}`
	funcMap := template.FuncMap{
		"ptrRecord": func(value, name string) string {
			// Convert the IP address to a PTR record format
			parts := strings.Split(value, ".")
			if len(parts) != 4 {
				return ""
			}
			return fmt.Sprintf("%s.in-addr.arpa. IN PTR %s", parts[3]+"."+parts[2]+"."+parts[1]+"."+parts[0], monkalev1alpha1.EnsureFQDN(name))
		},
	}
	tmpl, err := template.New("record").Funcs(funcMap).Parse(recordTmpl)
	if err != nil {
		return "", fmt.Errorf("could not template DNS Record: %v", err)
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, dnsRecord); err != nil {
		return "", fmt.Errorf("could not template DNS Record: %v", err)
	}
	return result.String(), nil
}
