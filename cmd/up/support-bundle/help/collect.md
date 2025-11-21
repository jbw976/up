The `up support-bundle collect` command allows you to collect diagnostic information from
your Kubernetes cluster or control plane for troubleshooting purposes.

## Usage

```bash
up support-bundle collect [flags]
```

### Flags

- `--config`, `-c`: Path to a SupportBundle YAML configuration file. If provided,
  this will be used instead of the default configuration. Redactors can be included
  in the same file as a separate YAML document (multi-document YAML).
- `--kubeconfig`, `-k`: Path to the kubeconfig file. If not provided, the default
  kubeconfig resolution will be used.
- `--output`, `-o`: Output file path for the support bundle archive.
  If not specified, a timestamped filename will be used (e.g., `upbound-support-bundle-20250105-163905.tar.gz`).
- `--include-namespaces`: Namespaces to include in the support bundle. Supports glob patterns
  (e.g., `upbound-*` to include all namespaces starting with "upbound-"). Multiple patterns
  can be specified.
- `--exclude-namespaces`: Namespaces to exclude from the support bundle. Supports glob patterns
  (e.g., `upbound-*` to exclude all namespaces starting with "upbound-"). Multiple patterns
  can be specified.
- `--crossplane-resources-only`, `-x`: Collect only Crossplane CRDs and custom resources
  (resources with composites, crossplane, or managed categories). When this flag is set,
  log collectors are excluded and only Crossplane-related resources are included in the bundle.

### Examples

```bash
# Collect a support bundle with default settings
up support-bundle collect

# Collect a support bundle to a specific location
up support-bundle collect --output /tmp/my-support-bundle.tar.gz

# Collect a support bundle with a custom configuration file
up support-bundle collect --config my-config.yaml

# Collect a support bundle from specific namespaces
# By default, upbound-system, crossplane-system, and any control plane namespaces
# will be included.
up support-bundle collect --include-namespaces crossplane-system,upbound-system

# Include namespaces using glob patterns
up support-bundle collect --include-namespaces upbound-*

# Exclude certain namespaces from the support bundle
up support-bundle collect --exclude-namespaces kube-system

# Exclude namespaces using glob patterns
up support-bundle collect --exclude-namespaces upbound-*

# Collect only Crossplane resources (no logs, only CRDs and custom resources)
up support-bundle collect --crossplane-resources-only
```

## Configuration File

You can provide a custom SupportBundle configuration file using the `--config` flag.
The configuration file can include both the SupportBundle spec and Redactors in a
single file using multi-document YAML format (separated by `---`).

When using `--config`, the `--include-namespaces` and `--exclude-namespaces` flags
are ignored. The namespaces specified in the configuration file will be used instead.

```yaml
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: support-bundle
spec:
  collectors:
    - logs:
        namespace: crossplane-system
    - clusterInfo: {}
    - clusterResources:
        namespaces:
          - crossplane-system
          - upbound-system
---
apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: custom-redactors
spec:
  redactors:
    - name: custom-redactor
      removals:
        regex:
          - redactor: ".*password.*"
```

## Security

All sensitive information is automatically redacted from the support bundle,
including:

- Kubernetes secrets
- Passwords
- API keys
- IPv4 addresses in logs
- ConfigMap data fields
- EnvironmentConfig data fields
- Other sensitive data

This ensures that support bundles can be safely shared for troubleshooting
purposes. You can customize redactors by including them in your configuration
file as a separate YAML document.

**Important:** Before sharing a support bundle, always verify that no sensitive
data remains in the bundle. While automatic redaction covers common cases, you
should review the bundle contents to ensure all sensitive information has been
properly removed.
