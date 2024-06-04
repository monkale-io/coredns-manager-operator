## Getting Started

* You’ll need a Kubernetes to run against. 
  * kube-dns service is exposed
  * It is recommended to switch the current context to the kube-system

### Go 1.20
The project is done using go1.20.

Install.
```sh
# install go.120
go install golang.org/dl/go1.20@latest
GOPATH=`go env GOPATH`
$GOPATH/bin/go1.20 download

# export 
export GOROOT=`$(go env GOPATH)/bin/go1.20 env GOROOT`
export PATH=${GOROOT}/bin:${PATH}

# check version
go version
```



### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -k config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/coredns-manager-operator:tag
```

3. Update namespace if needed in `config/default/kustomization.yaml`. Operator shall be installed in the same namespace as your coredns. 
   
4. Deploy the controller to the cluster with the image specified by `IMG`:

   ```sh
   make deploy IMG=<some-registry>/coredns-manager-operator:tag
   ```
   OR, Alternatively, generate manifests using kustomize.
   ```sh
    make installation-manifests IMG=<some-registry>/coredns-manager-operator:tag
    kubectl apply -f deploy/operator.yaml
    ```


### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:
   ```sh
   make install
   ```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):
   ```sh
   make run
   ```

   **NOTE:** You can also run this in one step by running: `make install run`

   **NOTE:** You can also run `make from-scratch` to run all together: recreate kind cluster, set kube-system as current context, generate, manifests, install and run.

#### Manual test
Since the project does not have unit tests yet, it is recommended to follow the manual QA guide to ensure the quality of the controller.
[Manual QA Guide](qa/dev-manual-qa-guide.md)

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

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