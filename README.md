# Coredns-manager-operator
The CoreDNS Manager Operator is designed for offline or home lab setups, eliminating the need for additional DNS software like dnsmasq or named and leveraging a GitOps approach for managing DNS configurations.

In many on-premises environments, managing DNS records can be complex and often requires separate DNS servers. With the CoreDNS Manager Operator, you can handle internal DNS directly within your Kubernetes cluster, simplifying the process and reducing infrastructure needs. This operator integrates with Kubernetes, making it easy to manage DNS records efficiently using a GitOps workflow.

## Features
* Manage DNS records within a Kubernetes cluster using `DNSRecord` and `DNSZone` CRDs.
* Attach DNS zones to Kubernetes' CoreDNS server.
* Automatically generate zone files, including SOA and NS records.
* Support all DNS record types specified by RFC1035.
* Good choice for home labs and disconnected environments, eliminating the need for additional DNS software and enables GitOps approach for managing DNS configurations.

## Limitations
* Compatibility: Currently tested with k3s(v1.29.5+k3s1), talos(kubernetes v1.30.1) and KinD(0.20.0). It is expected to work with other Kubernetes distributions.

* CoreDNS Exposure: The project is designed to manage CoreDNS zones, but it does not expose CoreDNS to the network by default. Administrators must handle exposure themselves using methods such as LoadBalancer services, iptables, NodePorts, HAProxy, IngressRoutes or even kubectl port-forwarding.

## Project Resources
* DNSRecord: Represents individual DNS records such as A, AAAA, CNAME, etc.
  
  [DNSRecord Documentation](docs/dnsrecords.md)

* DNSZone: Defines the domain (SOA) and groups all related DNSRecord resources into a zone file.

  [DNSZones Documentation](docs/dnszones.md)

* DNSConnector: Integrates the operator with Kubernetes' CoreDNS. It ensures that any changes to DNSRecord and DNSZone resources are reflected in CoreDNS.

  [DNSConnector Documentation](docs/dnsconnector.md)

## Quick start
During this guide you we will briefly learn coredns-manager-operator' resources and debug commands. In case of problems visit [troubleshoot guide](docs/troubleshoot.md).


**> IMPORTANT:** Make sure to deploy the coredns-manager-operator and its resources in the same namespace as CoreDNS, usually `kube-system`. Always include the `--namespace kube-system` to all your queries or just switch the context.

1. Expose CoreDNS to your network: Ensure CoreDNS is accessible on port 53 (TCP/UDP). The method depends on your deployment.
   ```sh
   # in this example 192.168.122.10 is kubernetes cluster.
   # udp
   $ nc -zvuw3 192.168.122.10 53
   Connection to 192.168.122.10 53 port [udp/domain] succeeded!
   # tcp test
   $ nc -zvw3 192.168.122.10 53
   Connection to 192.168.122.10 53 port [tcp/domain] succeeded!
   ```

2. Increase Coredns replicas(Optional)
   
   Zonefile updates can cause a short interruption in name resolution, typically lasting from <1 to 5 seconds. To minimize downtime during updates, consider increasing CoreDNS replicas to 2 or more.

3. Install the operator. By default it will install in `kube-system` namespace. Modify the manifest if needed.
   ```sh
   $ kubectl apply -f https://raw.githubusercontent.com/monkale-io/coredns-manager-operator/main/deploy/operator.yaml
   ```

   Ensure that the operator is up and running
   ```sh
   $ kubectl get deployments.apps coredns-manager-operator-controller-manager -n kube-system
   NAME                                          READY   UP-TO-DATE   AVAILABLE   AGE
   coredns-manager-operator-controller-manager   1/1     1            1           4m10s
   ```

4. Create DNSConnector:
   * The corednsCM should point to the CoreDNS ConfigMap, usually named `coredns` and located in the `kube-system` namespace.
   * The `corefileKey` points to the Corefile inside the ConfigMap, typically called `Corefile`.
   * The corednsDeployment refers to the CoreDNS deployment, which is generally named `coredns`. This could also be a DaemonSet or StatefulSet.
  
   In most popular Kubernetes distributions, these settings will remain the same, so you can use the following template without modifications: 
   ```sh
   $ cat << EOF | kubectl apply -f -
   apiVersion: monkale.monkale.io/v1alpha1
   kind: DNSConnector
   metadata:
     name: coredns
     namespace: kube-system
   spec:
     corednsCM: 
       name: coredns
       corefileKey: Corefile
     corednsDeployment:
       name: coredns
       type: Deployment
   EOF
   ```
   
   Then check if the resource DNSConnector became "Active"
   ```sh
   $ kubectl get dnsconnectors -n kube-system 
   NAME      LAST CHANGE            STATE    MESSAGE
   coredns   2024-06-03T18:51:44Z   Active   CoreDNS Ready
    ```

5. Create `DNSZone`
   Define the root of your domain.

   * Set your domain and set `connectorName` to point to the previously created DNSConnector resource - `coredns`.
   * The `ipAddress` should reflect the IP address of your Kubernetes node or Load Balancer — the address where you want CoreDNS to listen externally.
   * The `respPersonEmail` field is for the responsible person's (admin's) email.
   
   Use the following template:
   ```sh
   $ cat << EOF | kubectl apply -f -
   apiVersion: monkale.monkale.io/v1alpha1
   kind: DNSZone
   metadata:
     name: demo-example-zone
     namespace: kube-system
   spec:
     connectorName: coredns
     domain: "demo.example.com"
     primaryNS:
       ipAddress: "192.168.122.10"
     respPersonEmail: "admin@example.com"
   EOF
   ```
   
   Check DNSZone status. It has to be "Active".
   ```sh
   $ kubectl get dnszones -n kube-system 
   NAME                DOMAIN NAME        RECORD COUNT   LAST CHANGE            CURRENT SERIAL   STATE
   demo-example-zone   demo.example.com   0              2024-06-03T18:57:59Z   0603185759       Active
   ```

6. Create your first `DNSRecord`.

   Let's create our first A record to point any hosts under `*.ingress.demo.example.com` to `10.100.100.10`. This will match all hosts such as `app1.ingress.demo.example.com`.

   * `Record.name` specifies the name of the `DNSRecord`
   * `Record.value` in this case specifies the IP address of the A record
   * `dnszoneref.name` references the `DNSZone` name created previously

   Create the first record:
   ```sh
   $ cat << EOF | kubectl apply -f -
   ---
   apiVersion: monkale.monkale.io/v1alpha1
   kind: DNSRecord
   metadata:
     name: demo-a-record
     namespace: kube-system
   spec:
     record:
       name: "*.ingress"
       value: "10.100.100.10"
       type: "A"
     dnsZoneRef:
       name: "demo-example-zone"
   EOF
   ```

   Inspect the record
   ```sh
   $ kubectl get dnsrecords -n kube-system 
   NAME            RECORD NAME   RECORD TYPE   RECORD VALUE    ZONE REFERENCE      LAST CHANGE            STATE
   demo-a-record   *.ingress     A             10.100.100.10   demo-example-zone   2024-06-03T19:10:18Z   Ready
   ```

   Additional info:
   * The following record types are supported: A, AAAA, CNAME, MX, TXT, NS, PTR, SRV, CAA, DNSKEY, DS, NAPTR, RRSIG, DNAME, HINFO. Visit `config/samples/monkale_v1alpha1_dnsrecord.yaml` to see examples.
   * DNSRecords are templated into the zone file as standardized by RFC 1035. Pay attention to the domain names you insert into the zone files. If a record has a dot at the end, the DNS server will treat it as a fully qualified domain name (FQDN). If there is no dot, the DNS server will add the zone file origin name to the record. 
     
     More here
     [Troubleshoot Guide - FQDNs vs Relative names](docs/dnsrecords.md#pay-attention-to-domain-names-and-fqdns)


7. Try to resolve
   ```sh
   # try app1.ingress.market.example.com
   $ dig +short @192.168.122.10 app1.ingress.demo.example.com 
   10.100.100.10
   # try app2.ingress.market.example.com
   $ dig +short @192.168.122.10 app2.ingress.demo.example.com 
   10.100.100.10
   ```

## Troubleshoot

[Troubleshoot guide](docs/troubleshoot.md)

## Articles

* [Medium: Managing Internal DNS in Air-Gapped k3s Clusters with Monkale CoreDNS-Manager-Operator](https://medium.com/@nicholas5421/managing-internal-dns-in-air-gapped-k3s-clusters-with-monkale-coredns-manager-operator-fa1c9136cc2c)
* [Medium: Installing Monkale CoreDNS Manager Operator on Single-Node Talos](https://medium.com/@nicholas5421/installing-monkale-coredns-manager-operator-on-single-node-talos-16f8be900585)

## Contact

* [Email - monkaleio@gmail.com](mailto:monkaleio@gmail.com)
* [Reddit - r/monkaleio](https://www.reddit.com/r/monkaleio/s/d69geCqifS)

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to contribute to this project.

## Changelog

All notable changes to this project will be documented in [CHANGELOG.md](CHANGELOG.md).

## License

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
