# Overview
When you're troubleshooting the coredns-manager-operator, your go-to tools are `kubectl get` and `kubectl describe`. These commands help you check the status of the resources involved. 

The coredns-manager-operator is made up of different custom resources in your Kubernetes cluster, all linked together. Any issues affecting these resources will show up in their status when you run `kubectl get`. For more details on a specific resource, use `kubectl describe`. If these steps don't solve your problem, then you can check the logs.

## Wrong Namespace

**>IMPORTANT:** Make sure to deploy the coredns-manager-operator and its resources in the same namespace as CoreDNS, usually `kube-system`. Always include the `--namespace kube-system`



# DNSRecords Troubleshoot

* [DNSZones Guide - Troubleshoot](dnszones.md#Troubleshoot)

* [Troubleshoot Guide - Nameresolution Troubleshoot](troubleshoot.md#nameresolution-troubleshoot)

**Hint:** Remember that Specs of DNSRecords are templated into the standardized RFC1035 Zonefile entry format. They are templated using the following template: `{{ .Spec.Record.Name }} {{ .Spec.Record.TTL }} IN {{ .Spec.Record.Type }} {{ .Spec.Record.Value -}}`. Then, they are checked for syntax errors. However, they do not check for logic errors. If a record seems `Ready` but cannot be resolved, investigate potential logic issues in your records. Refer to the "Nameresolution Troubleshoot" section for further guidance.


# DNSZones Troubleshoot

[DNSZones Guide - Troubleshoot](dnszones.md#Troubleshoot)

**Hint:** DNSZone specifications are used to template the Start of Authority (SOA) record and primary NS (Name Server) record for the zone.



# DNSConnectors Troubleshoot

[DNSConnector Guide - Troubleshoot](dnsconnector.md#Troubleshoot)

* DNSConnector serves as the bridge between coredns-manager-operator and Kubernetes' CoreDNS. It references the coredns deployment configMap and the coredns deployment itself. If `kubectl get dnsconnectors` indicates any misconfiguration, it requires prompt attention.

* DNSConnector issues commonly arise during initial implementation of the coredns-manager-operator or due to incorrect patching procedures. Regular monitoring and thorough validation are essential to prevent and resolve such issues effectively.

* It's crucial to recognize that certain types of misconfigurations could potentially cause the operator to crash.



# Nameresolution Troubleshoot

* Individual records and the entire zone file undergo syntax checks, but logic errors are not verified. If a record appears as **"Ready"** but remains unresolved, investigate potential logic issues within your records.

* The operator generates `coredns-zone-<dnszone.name>` configMaps and attaches them to CoreDNS. For troubleshooting, utilize `kubectl describe cm coredns-zone-<dnszone.name>` and `kubectl describe dnsrecord <dnsrecord name>`.

* Remember, the coredns-manager-operator is not a DNS server but rather a tool for templating zone files using DNSZone and DNSRecord resources per RFC1035 standards. Handle debugging as you would with any other DNS server, then apply fixes to DNSRecord or DNSZone accordingly.

## Networking issues

The project is designed to manage CoreDNS zones, but it does not expose CoreDNS to the network by default. Administrators must handle exposure themselves using methods such as LoadBalancer services, iptables, NodePorts, HAProxy, IngressRoutes or even kubectl port-forwarding.

## Resolve name resolution

In this example, the record `www.market.example.com` appears correct at first glance, with an expected resolution to `10.100.100.10`. However, despite this apparent correctness, the record cannot be resolved for an unknown reason.

```sh
# zone domain is market.example.com
market-example-zone      market.example.com   15             2024-06-03T17:23:03Z   0603172349       Active

# record is Ready
$ kubectl get dnsrecords.monkale.monkale.io www-a-market-example
NAME                   RECORD NAME              RECORD TYPE   RECORD VALUE    ZONE REFERENCE        LAST CHANGE            STATE
www-a-market-example   www.market.example.com   A             10.100.100.10   market-example-zone   2024-06-03T16:32:54Z   Ready

# cannot resolve
$ dig @192.168.122.10 www.market.example.com

; <<>> DiG 9.18.26 <<>> @192.168.122.10 www.market.example.com
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 20668
;; flags: qr aa rd; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 1
;; WARNING: recursion requested but not available

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
; COOKIE: a15c22a7e6a3cd00 (echoed)
;; QUESTION SECTION:
;www.market.example.com.                IN      A

;; AUTHORITY SECTION:
market.example.com.     86400   IN      SOA     ns1.market.example.com. admin\@example.com. 603172349 7200 3600 1209600 86400

;; Query time: 1 msec
;; SERVER: 192.168.122.10#53(192.168.122.10) (UDP)
;; WHEN: Mon Jun 03 12:24:57 CDT 2024
;; MSG SIZE  rcvd: 156
```

The problem is that the user has configured the record `www.market.example.com` without indicating it as a fully qualified domain name (FQDN) by adding a `.` at the end. Consequently, the DNS server interprets it as `www.market.example.com.market.example.com` due to the default ORIGIN of `market.example.com`.

To resolve this issue, there are two solutions:

1. Set Record as FQDN:

   Modify the record to www.market.example.com. by adding a `.` at the end to indicate it as a fully qualified domain name.

2. Remove Domain Name from Record:

   Alternatively, remove the domain name from the record, leaving only www.

Here's an example of Solution 1:
```sh
# patch record to fqdn
$ kubectl patch DNSRecord www-a-market-example --type='merge' -p='{"spec":{"record": {"name": "www.market.example.com."}}}'
dnsrecord.monkale.monkale.io/www-a-market-example patched

# record name is not fqdn
$ kubectl get DNSRecord www-a-market-example 
NAME                   RECORD NAME               RECORD TYPE   RECORD VALUE    ZONE REFERENCE        LAST CHANGE            STATE
www-a-market-example   www.market.example.com.   A             10.100.100.10   market-example-zone   2024-06-03T17:26:58Z   Ready

# works
$ dig +short @192.168.122.10 www.market.example.com
10.100.100.10
```

Here's an example of Solution 2:
```sh
# remove domain and trailing dot
$ kubectl patch DNSRecord www-a-market-example --type='merge' -p='{"spec":{"record": {"name": "www"}}}'
dnsrecord.monkale.monkale.io/www-a-market-example patched

# get resource
$ kubectl get DNSRecord www-a-market-example 
NAME                   RECORD NAME   RECORD TYPE   RECORD VALUE    ZONE REFERENCE        LAST CHANGE            STATE
www-a-market-example   www           A             10.100.100.10   market-example-zone   2024-06-03T17:29:25Z   Ready

# works
$ dig +short @192.168.122.10 www.market.example.com
10.100.100.10
```


# Operator Level Issues

To troubleshoot operator-level issues, describe the DNSConnector resource and review its logs. Report any findings for further investigation and resolution. This approach helps to identify and address issues promptly, ensuring the smooth operation of the DNS management system.

However, they do not check for logic errors.