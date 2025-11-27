The `up resource count` command counts Crossplane resources in a Kubernetes cluster,
providing a summary of managed resources, composite resources, claims, and composed resources.

## Usage

```bash
up resource count [flags]
```

### Examples

```bash
# Count resources using the current kubeconfig context
up resource count

# Count resources using a specific kubeconfig file
up resource count --kubeconfig /path/to/kubeconfig

# Count resources using a specific context
up resource count --context my-cluster-context

# Output in JSON format
up resource count --format json

# Output in YAML format
up resource count --format yaml
```

## Resource Types

The command counts the following Crossplane resource types:

- **Managed Resources**: Resources directly managed by Crossplane providers (e.g., S3 Buckets, RDS Instances)
- **Composite Resources (XRs)**: Custom resources defined by CompositeResourceDefinitions
- **Composite Resource Claims (XRCs)**: Namespace-scoped claims for composite resources
- **Composed Resources**: Resources that are part of a composite resource's composition
- **Total Resources**: Sum of all counted resources

## Notes

- Resources are deduplicated to avoid double-counting
- ProviderConfig and related resources are excluded from the count
- The command discovers resources by examining CRDs owned by Crossplane providers and CompositeResourceDefinitions
