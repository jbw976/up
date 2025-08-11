The `uninstall` command uninstalls the API Connector from a cluster.

#### Examples

Uninstall the API Connector from the cluster but leave the connections and
secrets in place:

```shell
up controlplane api-connector uninstall --target-kubeconfig kubeconfig-path-for-deployment-cluster
```

Uninstall the API Connector from the cluster and delete the connections and
secrets. API objects created by the API Connector initial installation will not
be deleted:

```shell
up controlplane api-connector uninstall --all --target-kubeconfig kubeconfig-path-for-deployment-cluster
```
