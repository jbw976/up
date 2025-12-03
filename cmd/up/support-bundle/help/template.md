The `up support-bundle template` command outputs the default SupportBundle YAML configuration
template that can be used as a starting point for custom support bundle configurations.

## Usage

```bash
up support-bundle template [flags]
```

### Examples

```bash
# Output the default support bundle template
up support-bundle template

# Output a template with specific namespaces
up support-bundle template --include-namespaces crossplane-system,upbound-system

# Save the template to a file
up support-bundle template > my-support-bundle-config.yaml

# Use the configuration file with the collect command
up support-bundle collect --config my-support-bundle-config.yaml
```
