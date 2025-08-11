The `generate` command creates a composition and adds the required function
packages to the project as dependencies.

#### Examples

Generate a composition from a CompositeResourceDefinition (XRD) and save output
to `apis/xnetworks/composition.yaml`:

```shell
up composition generate apis/xnetwork/definition.yaml
```

Generate a composition from a Composite Resource (XR) and save output to
`apis/xnetworks/composition.yaml`:

```shell
up composition generate examples/xnetwork/xnetwork.yaml
```

Generate a composition from a Composite Resource (XR), prefixing the
`metadata.name` with `aws` and save output to
`apis/xnetworks/composition-aws.yaml`:

```shell
up composition generate examples/network/network-aws.yaml --name aws
```

Generate a composition from a Composite Resource (XR) with a custom plural form
and save output to `apis/xdatabases/composition.yaml`:

```shell
up composition generate examples/xdatabase/database.yaml --plural postgreses
```
