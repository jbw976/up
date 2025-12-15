The `up support-bundle serve` command serves support bundle files over HTTP for live viewing.
It starts a local Kubernetes API server (not a full cluster), imports the support bundle resources,
and provides a kubeconfig file that allows you to interact with the bundle using standard
Kubernetes tools like `kubectl` or `k9s`.

Note: This runs only the API server for viewing collected data, no workloads are actually running.

## Usage

```bash
up support-bundle serve [path] [flags]
```

### Examples

```bash
# Serve a support bundle tar.gz file
up support-bundle serve ./upbound-support-bundle.tar.gz

# Serve on a custom port
up support-bundle serve --port 9090

# Specify a custom kubeconfig output path
up support-bundle serve --kubeconfig-path ./my-kubeconfig
```

## Viewing the Bundle

Once the server is running, you can use standard Kubernetes tools to view the bundle contents:

```bash
kubectl --kubeconfig=./support-bundle-kubeconfig get pods --all-namespaces
kubectl --kubeconfig=./support-bundle-kubeconfig get all -A
kubectl --kubeconfig=./support-bundle-kubeconfig logs <pod-name> -n <namespace>
```
