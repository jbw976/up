The `convert` command converts a CompositeResourceDefinition (XRD) to CRDs
(CustomResourceDefinitions) for validation purposes. This enables validation of
claims against schemas in CI workflows without needing to apply resources to a
control plane cluster.

The command always generates a Composite Resource (XR) CRD from the XRD.

**For v1 XRDs:** If the XRD defines claim names, a Claim CRD will also be generated.

**For v2 XRDs:** CRDs are generated based on the scope specified in the XRD
(Cluster-scoped or Namespaced)

#### Examples

Convert an XRD to CRDs and save them in the current directory:

```shell
up xrd convert path/to/xrd.yaml
```

Convert an XRD to CRDs and save them in a specific output directory:

```shell
up xrd convert path/to/xrd.yaml -o ./generated-crds
```

**For v1 XRDs:**
- `xwebapps.platform.example.com.yaml` - Composite Resource (XR) CRD
- `webapps.platform.example.com.yaml` - Claim CRD (if claim names are defined)

**For v2 XRDs:**
- `webapps.platform.example.com.yaml`
