The `generate` command is used to create an example Composite Resource (XR) or
Composite Resource Claim (XRC). For v2 projects only Composite Resources (XRs)
are supported. XRs are namespace-scoped by default, but you can choose
cluster-scoped using the `--scope=cluster` flag.

#### Examples

Creates an example Composite Resource (XR) or Composite Resource Claim (XRC)
resource using an interactive wizard:

```shell
up example generate
```

Create an example named `example` in the namespace `default` using an
interactive wizard to collect additional inputs:

```shell
up example generate --name example --namespace default
```

Create an example Composite Resource Claim (XRC) with specified api-group,
api-version, kind, and name, using an interactive wizard to collect additional
inputs:

```shell
up example generate --type claim --api-group platform.example.com \
    --api-version v1beta1 --kind Cluster --name example
```

Create an example Composite Resource (XR) or Composite Resource Claim (XRC)
based on the fields and default values in an existing
CompositeResourceDefinition (XRD). Use an interactive wizard to collect inputs:

```shell
up example generate apis/xnetworks/definition.yaml
```

Create an example Composite Resource (XR) based on the fields and default values
in an existing CompositeResourceDefinition (XRD). Use an interactive wizard to
collect inputs:

```shell
up example generate apis/xnetworks/definition.yaml --type xr
```
