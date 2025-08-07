The `install` command installs the API Connector into a consumer cluster.

Note that the API Connector is a preview feature, under active development and
subject to breaking changes. Production use is not recommended.

#### Examples

Install the API Connector into the consumer cluster and connect it to the
control plane referred to by the current context:

```shell
up controlplane api-connector install --consumer-kubeconfig /path/to/kubeconfig
```

Install the API Connector into the cluster and connect it to the control plane
referred to by the current context using the provided robot name for
authentication:

```shell
up controlplane api-connector install --consumer-kubeconfig /path/to/kubeconfig \
    --robot-name upbound-robot-name
```

Install the API Connector into the cluster but do not provision a
`ClusterConnection` resource or create a robot for authentication:

```shell
up controlplane api-connector install --consumer-kubeconfig /path/to/kubeconfig \
    --skip-connection
```
