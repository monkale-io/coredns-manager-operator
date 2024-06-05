# DNSZone Resource Documentation

## Overview

The `DNSZone` resource defines DNS zones within the Kubernetes cluster. This resource allows you to specify the details of a DNS zone, such as its domain, primary name server, responsible person's email, and other configuration parameters. Essentually, DNSZone specifications are used to template the SOA record and primary NS record for the zone.

## Specifying a DNSZone

### Schema

The schema for the DNSZone resource is as follows:

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSZone
metadata:
  name: example-dnszone
spec:
  cmPrefix: "coredns-zone-"
  domain: "example.com"
  primaryNS:
    hostname: "ns1"
    ipAddress: "192.0.2.2"
    recordType: "A"
  respPersonEmail: "admin@example.com"
  ttl: 86400
  refreshRate: 7200
  retryInterval: 3600
  expireTime: 1209600
  minimumTTL: 86400
  connectorName: "example-dnsconnector"
```

### Fields

#### spec.cmPrefix
* `cmPrefix` (string, optional): Specifies the prefix for the zone file configmap. The default value is coredns-zone-. The CM Name format is "prefix" + "metadata.name".

#### spec.domain
* `domain` (string, required): Specifies the domain in which DNS records are valid.

#### spec.primaryNS
* `primaryNS` (object, required): Defines the primary nameserver for the zone, including:
* `hostname` (string): The server name of the primary nameserver. Default is ns1.
* `ipAddress` (string): The IP address of the DNS server where the zone is hosted. It should be the address of your kubernetes/load balancer.
* `recordType` (string): The type of the record to be created for the NS's A record. Default is A.

#### spec.respPersonEmail
* `respPersonEmail` (string, required): The responsible party's email for the domain, typically formatted as admin@example.com but represented with a dot (.) instead of an at (@) in DNS records.

#### spec.ttl
* `ttl` (uint, optional): Specifies the default Time to Live (TTL) for the zone's records, indicating how long these records should be cached by DNS resolvers. Default is 86400 seconds (24 hours).

#### spec.refreshRate
* `refreshRate` (uint, optional): Defines the time a secondary DNS server waits before querying the primary DNS server to check for updates. Default is 7200 seconds (2 hours).

#### spec.retryInterval
* `retryInterval` (uint, optional): Defines how long a secondary server should wait before trying again to reconnect to the primary server after a failure. Default is 3600 seconds (1 hour).

#### spec.expireTime
* `expireTime` (uint, optional): Defines how long the secondary server should wait before discarding the zone data if it cannot reach the primary server. Default is 1209600 seconds (2 weeks).
spec.minimumTTL

#### minimumTTL
* `minimumTTL` (uint, optional): Specifies the minimum amount of time that should be allowed for caching the DNS records. If individual records do not specify a TTL, this value should be used. Default is 86400 seconds (24 hours).

#### spec.connectorName
* `connectorName` (string, required): The name of the DNSConnector resource to which this zone will be linked.

### Examples

#### Basic DNSZone (recommended for most users)
```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSZone
metadata:
  name: basic-dnszone
spec:
  domain: "basic.com."
  primaryNS:
    hostname: "ns1"
    ipAddress: "192.0.2.2"
    recordType: "A"
  respPersonEmail: "admin@basic.com"
  connectorName: "coredns"
```

#### Advanced DNSZone with tuned SOA
```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSZone
metadata:
  name: custom-dnszone
spec:
  domain: "custom.com."
  primaryNS:
    hostname: "nameserver1"
    ipAddress: "192.0.2.3"
    recordType: "A"
  respPersonEmail: "support@custom.com"
  ttl: 7200
  refreshRate: 3600
  retryInterval: 1800
  expireTime: 604800
  minimumTTL: 3600
  connectorName: "coredns"
```

## Status
The DNSZone resource also includes status fields that reflect the observed state of the resource.

### Status Fields
* `conditions` (array): Indicates the status of the DNSZone. Each condition includes:
* `currentZoneSerial` (string): The current version number of the zone file, implemented as time now formatted. Used to track the Zone version. 

* `recordCount` (int): The number of records in the zone, excluding SOA and primary ns records.

* `validationPassed` (boolean): Displays whether the zone file passed the syntax validation check.

* `zoneConfigmap` (string): The name of the generated zone config map.

* `checkpoint` (bool): Indicates whether the DNSZone was previously active. This flag is used to instruct the DNSConnector to preserve the old version of the DNSZone in case the update process encounters an issue.

### States
`conditions[].reason` represents DNSZone state.

* `Active` - The DNSZone has passed the syntax validation check and has been picked up by the DNSConnector controller.
* `UpdateErr` - An error occurred during the zone file update. In the `UpdateErr` state, the DNSConnector controller keeps the last known good DNS zone version, ensuring uninterrupted name resolution.
* `Pending` - The DNSZone has been created and passed the syntax validation check. It is waiting to be picked up by the DNSConnector controller.
  
### Status Example
```json
{
  "conditions": [
    {
      "lastTransitionTime": "2024-06-04T00:40:11Z",
      "message": "Picked up by DNSConnector",
      "observedGeneration": 7,
      "reason": "Active",
      "status": "True",
      "type": "Ready"
    }
  ],
  "currentZoneSerial": "0603194011",
  "recordCount": 0,
  "validationPassed": true,
  "zoneConfigmap": "coredns-zone-market-example-zone"
}
```

## Troubleshoot

### DNSZone in UpdateError state.

Typically occurs when the DNSZone didn't pass the syntax validation test. It most probably That could happen because of the bad patch made to the DNSZone specs. It's worth noting that even in a degraded state, CoreDNS retains the last "good" zone configuration. This ensures that all previously configured records remain accessible, mitigating the impact on DNS resolution.

```sh
$ kubectl describe dnszones market-example-zone 
Name:         market-example-zone
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>
API Version:  monkale.monkale.io/v1alpha1
Kind:         DNSZone
Metadata:
  Creation Timestamp:  2024-06-02T19:50:47Z
  Finalizers:
    dnszones/finalizers
  Generation:        5
  Resource Version:  35984
  UID:               74d11659-ff73-4197-9f09-499e99a22afd
Spec:
  Cm Prefix:       coredns-zone-
  Connector Name:  coredns
  Domain:          market.example.com
  Expire Time:     1209600
  Minimum TTL:     86400
  Primary NS:
    Hostname:         ns1
    Ip Address:       10.120.120.10.254
    Record Type:      A
  Refrash Rate:       7200
  Resp Person Email:  admin@example.com
  Retry Interval:     3600
  Ttl:                86400
Status:
  Conditions:
    Last Transition Time:  2024-06-03T14:38:58Z
    Message:               Zone validation failure. Preserving the previous version. Error: error parsing records: dns: bad A A: "10.120.120.10.254" at line: 11:26
    Observed Generation:   5
    Reason:                UpdateError
    Status:                False
    Type:                  Ready
  Current Zone Serial:     0603092425
  Record Count:            15
  Validation Passed:       true
  Zone Configmap:          coredns-zone-market-example-zone
Events:
```

### DnsZone is Pending state
A DNSZone enters a pending state when the zonefile has been created and is awaiting pickup by the DNSConnector. This typically occurs during the synchronization process between the DNSZone and the DNSConnector.

Additionally, a DNSZone might remain pending if it's deployed in an incorrect namespace. It's crucial to ensure that all operator-related resources, including DNSZone, are installed in the same namespace as your CoreDNS server.

Describe the DNSZone resource. Verify that the `spec.connectorName` points to the correct DNSConnector responsible for syncing the zonefile. If the connectorName is correct, proceed to describe the DNSConnector and review the controller logs for further insights.

```sh
$ kubectl describe dnszones no-connector-test-zone 
Name:         no-connector-test-zone
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>
API Version:  monkale.monkale.io/v1alpha1
Kind:         DNSZone
Metadata:
  Creation Timestamp:  2024-06-02T18:44:59Z
  Finalizers:
    dnszones/finalizers
  Generation:        1
  Resource Version:  622
  UID:               84efd361-f8d8-41a1-962b-0de16942fe05
Spec:
  Cm Prefix:       coredns-zone-
  Connector Name:  this-connector-does-not-exist
  Domain:          test.com
  Expire Time:     1209600
  Minimum TTL:     86400
  Primary NS:
    Hostname:         ns1
    Ip Address:       10.120.100.11
    Record Type:      A
  Refrash Rate:       7200
  Resp Person Email:  admin@test.local
  Retry Interval:     3600
  Ttl:                86400
Status:
  Conditions:
    Last Transition Time:  2024-06-02T18:45:00Z
    Message:               Zone ConfigMap has been created: coredns-zone-no-connector-test-zone
    Observed Generation:   1
    Reason:                Pending
    Status:                True
    Type:                  Ready
  Current Zone Serial:     0602134500
  Record Count:            0
  Validation Passed:       true
  Zone Configmap:          coredns-zone-no-connector-test-zone
Events:    
```