# DNSRecord Resource Documentation

## Overview

The `DNSRecord` resource defines DNS records within the Kubernetes cluster. This resource allows you to specify the details of a DNS record, such as its name, value, type, TTL (Time To Live), and the DNS zone it belongs to.

## Specifying a DNSRecord

### Schema

The schema for the DNSRecord resource is as follows:

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSRecord
metadata:
  name: example-dnsrecord
  namespace: kube-system
spec:
  record:
    name: "www.example.com."
    value: "192.0.2.1"
    type: "A"
    ttl: "300"
  dnsZoneRef:
    name: example-dnszone
    namespace: default
```

### Fields

#### spec.record

* `name` (string): The name of the DNS record, e.g., a domain name for an A record.
* `value` (string): The value of the DNS record, e.g., an IP address for an A record.
* `type` (string): The type of the DNS record according to RFC1035. Supported types are A, AAAA, CNAME, MX, TXT, NS, PTR, SRV, CAA, DNSKEY, DS, NAPTR, RRSIG, DNAME, and HINFO.
* `ttl` (string, optional): The Time To Live (TTL) for the DNS record. If not set, the default is the minimum TTL value in the SOA record. For simple scenarios it suggest to leave the default ttl.

#### spec.dnsZoneRef

* `name` (string): The name of the DNSZone instance to which this record will publish its endpoints.

### Examples

#### A Record

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSRecord
metadata:
  name: a-record-example
  namespace: kube-system
spec:
  record:
    name: "www.example.com."
    value: "192.0.2.1"
    type: "A"
  dnsZoneRef:
    name: example-dnszone
    namespace: default
```

#### CNAME Record

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSRecord
metadata:
  name: cname-record-example
  namespace: kube-system
spec:
  record:
    name: "mail.example.com."
    value: "www.example.com."
    type: "CNAME"
  dnsZoneRef:
    name: example-dnszone
    namespace: default
```

#### MX Record with TTL

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSRecord
metadata:
  name: mx-record-example
  namespace: kube-system
spec:
  record:
    name: "example.com."
    value: "10 mail.example.com."
    type: "MX"
    ttl: "1800"
  dnsZoneRef:
    name: example-dnszone
    namespace: default
```

#### More examples
For more examples visit
[DNSRecord Samples](../config/samples/monkale_v1alpha1_dnsrecord.yaml)

## Status
The DNSRecord resource also includes status fields that reflect the observed state of the resource.

### Status Fields
* `conditions` (array): Indicates the status of the DNSRecord. Each 
* `validationPassed` (boolean): Displays whether the record passed the syntax validation check.
* `generatedRecord` (string): Displays the generated DNS record.

### States
`conditions[].reason` represents DNSRecord state.

* `Ready` - The DNSRecord has passed the syntax validation check and has been added the DNSZone' zonefile.
* `Degraded` - The DNSRecord failed the syntax validation check. `Degraded` records are unresovable. 
* `Pending` - The DNSRecord has been created and passed the syntax validation check. It is waiting to be picked up by the DNSZone controller.

### Example Status

```json
{
  "conditions": [
    {
      "lastTransitionTime": "2024-06-03T16:33:18Z",
      "message": "CoreDNS Ready",
      "observedGeneration": 3,
      "reason": "Active",
      "status": "True",
      "type": "Ready"
    }
  ],
  "provisionedZones": [
    {
      "domain": "market.example.com",
      "name": "market-example-zone",
      "serialNumber": "0603113254"
    }
  ]
}
```

## How does it work
When a DNSRecord resource is created, its specifications are converted into zone file entries using the following template:
```go
{{ .Spec.Record.Name }} {{ .Spec.Record.TTL }} IN {{ .Spec.Record.Type }} {{ .Spec.Record.Value -}}
```
This template generates zone file entries that conform to the RFC1035 format, ensuring compatibility with standard DNS servers like CoreDNS.

### Pay Attention to Domain Names and FQDNs
Proper configuration of domain names and Fully Qualified Domain Names (FQDNs) is essential for accurate DNS resolution. An FQDN specifies the entire domain path, ending with a trailing dot (e.g., `www.example.com.`), which indicates the root of the DNS, because this format is typical for RFC-compliant zone files.

#### Common Issues
1. FQDN vs. Relative Domain Names:
   * FQDN (with trailing dot): Treated as an absolute domain name (e.g., `some.external.domain.com.`).
   * Relative Domain Name (without trailing dot): Appended with the zone file origin (e.g., `some.external.domain.com` could be interpreted as `some.external.domain.com.<origin>`).
   * `www.` will be treated as an FQDN and may cause a resolution error.
2. Configuration Errors:
   * Without a trailing dot, a record like `www.market.example.com` might be interpreted as `www.market.example.com.market.example.com.`

#### Solutions
1. Set Record as FQDN:
   Add a trailing dot to the record (e.g., www.market.example.com.) to indicate it as an FQDN.

2. Remove Domain Name:
   Use only the necessary part of the domain (e.g., www) to avoid misinterpretation.

## Troubleshoot

### DNSRecord in Degraded state

When a DNS Record is in a degraded state, it typically indicates a syntax check issue. Bad syntax should be treated the same as if you were manually trying to add this record to the zone file.


For example, in this record, there are two dots after `www`, which is obviously a bad fully qualified domain name (FQDN).

```sh
$ kubectl describe dnsrecords bad-domain-record-test
Name:         bad-domain-record-test
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>
API Version:  monkale.monkale.io/v1alpha1
Kind:         DNSRecord
Metadata:
  Creation Timestamp:  2024-06-02T20:02:51Z
  Finalizers:
    dnsrecords/finalizers
  Generation:        3
  Resource Version:  34517
  UID:               6e5cc436-7fac-444b-b222-a854c86a96dc
Spec:
  Dns Zone Ref:
    Name:  market-example-zone
  Record:
    Name:     www..
    Type:     A
    Value:    192.0.2.1
Status:
  Conditions:
    Last Transition Time:  2024-06-03T14:24:25Z
    Message:               Record validation failure: error parsing records: dns: bad owner name: "www.." at line: 1:6
    Observed Generation:   3
    Reason:                Degraded
    Status:                False
    Type:                  Ready
  Generated Record:        www.. IN A 192.0.2.1
Events:                    <none>
```


### DNSRecord in Pending state
When a DNS Record is in a pending state, it means that the record has passed the validation process and is waiting to be picked up by the DNSZone controller. This state typically occurs when the record is newly created or updated and may only last for a few seconds before transitioning to an Ready state.

To troubleshoot a DNSRecord in a pending state, you should first describe the resource and check the message field for any relevant information. Ensure that the `spec.dnsZoneRef` points to a valid DNSZone that exists in the cluster. If the DNSZone exists, proceed to describe the related `DNSZone` to further investigate any potential issues.

```sh
$ kubectl describe dnsrecord record-no-zone-test 
Name:         record-no-zone-test
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>
API Version:  monkale.monkale.io/v1alpha1
Kind:         DNSRecord
Metadata:
  Creation Timestamp:  2024-06-02T18:44:59Z
  Finalizers:
    dnsrecords/finalizers
  Generation:        1
  Resource Version:  592
  UID:               5334c0b1-fdc6-41c8-8efd-5281b95248b2
Spec:
  Dns Zone Ref:
    Name:  this-zone-does-not-exist
  Record:
    Name:     haproxy.cpe
    Type:     A
    Value:    10.149.149.10
Status:
  Conditions:
    Last Transition Time:  2024-06-02T18:44:59Z
    Message:               Record has been constructed. Awaiting for the dnszone controller to pick up the record
    Observed Generation:   1
    Reason:                Pending
    Status:                False
    Type:                  Ready
  Generated Record:        haproxy.cpe IN A 10.149.149.10
  Validation Passed:       true
Events:                    <none>
```