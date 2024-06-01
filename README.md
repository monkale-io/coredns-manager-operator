# Coredns-manager-operator
The CoreDNS Manager Operator enables Kubernetes to function as a standalone DNS server, ideal for offline or home lab environments. It eliminates the need for additional DNS software like named or dnsmasq and supports a GitOps approach for managing DNS configurations.

## Features
* Manage DNS records within a Kubernetes cluster using DNSRecord and DNSZone CRDs.
* Attach DNS zones to Kubernetes' CoreDNS server.
* Automatically generate zone files, including SOA and NS records.
* Support all DNS record types specified by RFC1035.
* Automatically create reverse PTR records for A records.
* Enable GitOps approach for managing DNS configurations.
* Good choice for home labs and disconnected environments, eliminating the need for additional DNS software.

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
