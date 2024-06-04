# DNSConnector Resource Documentation

## Overview

The `DNSConnector` resource defines the connection between DNS zones and the CoreDNS deployment within the Kubernetes cluster. This resource allows you to specify the details of the CoreDNS configuration, deployment type, and additional settings required for the DNSConnector to function.

## Specifying a DNSConnector

### Schema
The schema for the DNSConnector resource is as follows:

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSConnector
metadata:
  name: coredns
  namespace: kube-system
spec:
  waitForUpdateTimeout: 300
  corednsCM:
    name: "coredns"
    corefileKey: "Corefile"
  corednsDeployment:
    type: "Deployment"
    name: "coredns"
    zonefilesMountDir: "/opt/coredns"
  corednsZoneEnaledPlugins:
    - "errors"
    - "log"
```

### Fields

#### spec.waitForUpdateTimeout
* `waitForUpdateTimeout` (int, optional): Specifies how long the DNSConnector should wait for CoreDNS to complete the update. If CoreDNS deployment hasn't completed the update within this time, the controller will perform a rollback. The default value is 120 seconds (2 minutes).

#### spec.corednsCM
* `corednsCM` (object, required): The name and corefile key of the CoreDNS ConfigMap.
  * `name` (string, optional): The name of the CoreDNS ConfigMap that contains the Corefile. Default is coredns.
  * `corefileKey` (string, optional): The key whose value is the Corefile. Default is Corefile.

#### spec.corednsDeployment
* `corednsDeployment` (object, required): Specifies the CoreDNS deployment type and name.
  * `type` (string, required): The type of the CoreDNS resource (e.g., Deployment, StatefulSet, DaemonSet).
  * `name` (string, optional): The name of the CoreDNS resource. This field is optional if type is Pod and a LabelSelector is specified.
  * `zonefilesMountDir` (string, optional): Specifies the mount path for zone files. Default is /opt/coredns.

#### spec.corednsZoneEnaledPlugins
`corednsZoneEnaledPlugins` (array of strings, optional): List of enabled CoreDNS plugins. Refer to the CoreDNS plugins documentation for more details. Common plugins include errors and log.
Example Resources

### Examples



#### For most situations
 
In most popular Kubernetes distributions, these settings will remain the same, so you can use the following template without modifications.

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSConnector
metadata:
  name: coredns
spec:
  corednsCM:
    name: "coredns"
    corefileKey: "Corefile"
  corednsDeployment:
    type: "Deployment"
    name: "coredns"
```

#### Adjusted coredns example

In this example we have changed the cm mount path to /var/lib/coredns, and corednsDeployment set to DaemonSet. Additionally, "errors" and "log" plugins have been activated.

```yaml
apiVersion: monkale.monkale.io/v1alpha1
kind: DNSConnector
metadata:
  name: connector
spec:
  corednsCM:
    name: "coredns"
    corefileKey: "Corefile"
  corednsDeployment:
    type: "DaemonSet"
    name: "coredns"
    zonefilesMountDir: "/var/lib/coredns"
  corednsZoneEnaledPlugins:
    - "errors"
    - "log"
```

## Status
The DNSConnector resource also includes status fields that reflect the observed state of the resource.

### Status Fields
* `conditions` (array): Indicates the status of the DNSConnector. Each condition includes:
* `provisionedZones` (array): Displays DNSZones and their versions currently provisioned to CoreDNS.

### Example Status

```json
{
  "conditions": [
    {
      "lastTransitionTime": "2024-06-04T02:25:59Z",
      "message": "CoreDNS Ready",
      "observedGeneration": 2,
      "reason": "Active",
      "status": "True",
      "type": "Ready"
    }
  ],
  "provisionedZones": [
    {
      "domain": "custom.com.",
      "name": "custom-dnszone",
      "serialNumber": "0603210914"
    }
  ]
}
```

## Troubleshoot

* If `kubectl get dnsconnectors` indicates any misconfiguration, it requires prompt attention.

* DNSConnector issues commonly arise during initial implementation of the coredns-manager-operator or due to incorrect patching procedures.

* It's crucial to recognize that certain types of misconfigurations could potentially cause the operator to crash.

### DNSConnector misconfigured example

In the provided example, the `spec.corednsCM.corefileKey` points to the wrong location, resulting in an error when trying to detect the Corefile. To resolve this issue, follow these steps to identify the correct keyfile and patch the DNSConnector:

```sh
kubectl get dnsconnectors
NAME      LAST CHANGE            STATE   MESSAGE
coredns   2024-06-03T16:09:19Z   Error   could not detect corefile: key RandomString not found in CoreDNS ConfigMap
```

To overcome, find the correct keyfile and patch the DNSConnector's `spec.corednsCM.corefileKey`.
```sh
# Identify the correct keyfile location
$ kubectl get configmap coredns -oyaml
apiVersion: v1
data:
  Corefile: |
  ...

# Patch DNSConnector
$ kubectl patch DNSConnector coredns --type='merge' -p='{"spec":{"corednsCM": {"corefileKey": "Corefile"}}}'
```

### Troubleshooting DNSConnector stucked with the message "coredns is being updated" or "healthcheck failure: coredns is not healthy. Check coredns deployment log".

These message may be observed when users have set incorrect values in the `spec.corednsZonePlugins`. Visit [Coredns - plugins](https://coredns.io/plugins) for more info.
Double-checking and ensuring that the specified plugins are correctly configured can help resolve this issue.

Check coredns logs
```sh
$ kubectl logs  -l 'k8s-app=kube-dns'
[ERROR] Restart failed: /etc/coredns/Corefile:51 - Error during parsing: Unknown directive 'banana'
[ERROR] plugin/reload: Corefile changed but reload failed: starting with listener file descriptors: /etc/coredns/Corefile:51 - Error during parsing: Unknown directive 'banana'
/etc/coredns/Corefile:52 - Error during parsing: Unknown directive 'banana'
/etc/coredns/Corefile:52 - Error during parsing: Unknown directive 'banana'
```

To fix it, remove `banana` from `spec.corednsZonePlugins`.

### DNSConnector no status

If `kubectl get dnsconnectors` isn't displaying any status for your connector, check the `controller-manager-operator` logs for potential RBAC issues and report any findings.