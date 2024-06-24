# Exposing CoreDNS
The CoreDNS Manager Operator is designed to manage CoreDNS zones, but it does not expose CoreDNS to the network by default. Administrators must handle exposure themselves using available methods according to their security requirements. This can include ingress rules, external load balancers, or other methods.

## Exposing master nodes approach
This document covers scenarios where master nodes are used as DNS servers. Administrators will need to set up all nodes accordingly.

### HostPort Method
One common way to expose CoreDNS is by using hostPort. This method involves scaling CoreDNS pods to match the number of Kubernetes master nodes, exposing port 53, and configuring a rolling update strategy to replace pods during updates. This approach has minimal downtime during CoreDNS updates, typically just a few seconds, and is reliable during master node failures.

For example, with three master nodes, you can use the following commands:
```sh
kubectl scale deployment coredns --replicas 0 -n kube-system
kubectl patch deployment coredns -n kube-system --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/template/spec/containers/0/ports/0/hostPort",
    "value": 53
  },
  {
    "op": "add",
    "path": "/spec/template/spec/containers/0/ports/1/hostPort",
    "value": 53
  }
]'
kubectl patch deployment coredns -n kube-system --type='json' -p='[{"op": "replace", "path": "/spec/strategy", "value": {"type": "RollingUpdate","rollingUpdate": {"maxUnavailable": 1,"maxSurge": 1}}}]'
kubectl scale deployment coredns --replicas 3 -n kube-system
```

For a practical example, refer to this guide: [Installing Monkale CoreDNS Manager Operator on Single Node Talos](https://medium.com/@nicholas5421/installing-monkale-coredns-manager-operator-on-single-node-talos-16f8be900585)


### K3s Klipper LB Method

K3s has a built-in Klipper Load Balancer, which simplifies exposure. For K3s users, you can patch the kube-dns service spec type to `LoadBalancer`.

```sh
kubectl patch service kube-dns --type='merge' --namespace kube-system \
  -p='{"spec":{"type": "LoadBalancer"}}'
```

Using this approach, it is recommended to add more replicas for each node to ensure zone updates occur without disruption.

**Note:** If you have a multi-node setup and use this method, keep in mind that name resolution could be disrupted for up to 2 minutes in the event of a master node failure. The secondary DNS resolution process and reconciliation could take this long.

For a detailed guide, refer to this article: [Managing Internal DNS in Air-Gapped K3s Clusters with Monkale CoreDNS Manager Operator](https://medium.com/@nicholas5421/managing-internal-dns-in-air-gapped-k3s-clusters-with-monkale-coredns-manager-operator-fa1c9136cc2c)
