# questdb-operator

The QuestDB Operator is a group of controllers and webhooks that are designed to manage QuestDB instances running in Kubernetes clusters.

## Description

### Prerequisites

The QuestDB resource type should be compatible with mainstream Kubernetes distributions, since it orchestrates `v1` and `apps/v1` components like PersistentVolumeClaims, StatefulSets, Services, and ConfigMaps.

The QuestDBSnapshot resource type requires the [CSI Snapshotter](https://github.com/kubernetes-csi/external-snapshotter) to be installed. This includes installing:
- CRDs <https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd>
- VolumeSnapshot Controller <https://github.com/kubernetes-csi/external-snapshotter/tree/master/pkg/common-controller>
- VolumeSnapshot Validation Webhook (optional, but recommended) <https://github.com/kubernetes-csi/external-snapshotter/tree/master/pkg/validation-webhook>
- A CSI Driver that includes the snapshot capability (see <https://kubernetes-csi.github.io/docs/drivers.html> for an up-to-date list of drivers and their features)

Once you've installed the required components, you need to
- Create a [VolumeSnapshotClass](https://kubernetes.io/docs/concepts/storage/volume-snapshot-classes/) that uses your installed CSI as a driver
- If you want, you can also add the annotation: `snapshot.storage.kubernetes.io/is-default-class: "true"` to the VolumeSnapshotClass's metadata.

Here's a step-by-step example of this installation in an AWS blg post:
<https://aws.amazon.com/blogs/containers/using-ebs-snapshots-for-persistent-storage-with-your-eks-cluster/>

- cert manager

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/questdb-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/questdb-operator:tag
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

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

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

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

### Autoreload

The controller does not automatically update the StatefulSet on config changes, but you can enable this by adding
a `stakater/Reloader` annotation to the StatefulSet directly, pointing to the child ConfigMap. The controller will persist any annotations made to child objects, so this will work with no issues. See <https://github.com/stakater/Reloader> for more information.

### Credentials

To use a secret as a source for ilp or psql credentials, you need to add the following annotations to an existing secret:

```yaml
# ILP Secret
annotations:
  questdb.crd.questdb.io/name: questdb-sample
  questdb.crd.questdb.io/secret-type: ilp

# PSQL Secret
annotations:
  questdb.crd.questdb.io/name: questdb-sample
  questdb.crd.questdb.io/secret-type: psql
```

The ILP Secret must contain an `auth.json` key that contains your JWK used for ILP authentication. This will be mounted to the database container as a file and referenced by the database.

The PSQL Secret must contain the `QDB_PG_USER` and `QDB_PG_PASSWORD` keys. These will be mounted to the container as environment variables. Be sure not to overwrite these in `questdb.spec.extraEnv`, as this can cause unexpected behavior, and add-ons like snapshots will likely break.

See the [yaml examples](config/samples/secrets.yaml) for more information

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
