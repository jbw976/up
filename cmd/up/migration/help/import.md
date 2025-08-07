The `import` command imports resources from an exported bundle into a Managed
Control Plane.

By default, all managed resources will be paused during the import process for
possible manual inspection/validation. You can use the --unpause-after-import
flag to automatically unpause all claim,composite,managed resources after the
import process completes.

#### Examples

Automatically import the control plane state from `my-export.tar.gz`. Claim and
composite resources that were paused during export will remain paused. Managed
resources will be paused. If they were already paused during export, the
annotation `migration.upbound.io/already-paused: "true"` will be added to
preserve their paused state:

```shell
up migration import --input=`my-export.tar.gz`
```

Automatically import and unpause claims, composites, and managed resources after
importing them. Resources with the annotation
`migration.upbound.io/already-paused: "true"` will remain paused:

```shell
up migration import --unpause-after-import
```

Automatically import and unpause claims, composites, and managed resources after
importing them. The `metadata.name` of claims will be adjusted for MCP Connector
compatibility, and the corresponding composite's `claimRef` will also be
updated. Resources annotated with `migration.upbound.io/already-paused: "true"`
will remain paused:

```shell
up migration import --unpause-after-import --mcp-connector-claim-namespace=default \
    --mcp-connector-cluster-id=my-cluster-id
```
