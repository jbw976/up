The `generate` command creates a CompositeResourceDefinition (XRD) from a given
Composite Resource (XR) and generates associated language models for function
usage.

#### Examples

Generate a CompositeResourceDefinition (XRD) based on the specified Composite
Resource and save it to the default APIs folder in the project:

```shell
up xrd generate examples/cluster/example.yaml
```

Generate a CompositeResourceDefinition (XRD) with a specified plural form,
useful for cases where automatic pluralization may not be accurate (e.g.,
"postgres"):

```shell
up xrd generate examples/postgres/example.yaml --plural postgreses
```

Generate a CompositeResourceDefinition (XRD) and save it to a custom path within
the project's default APIs folder.

```shell
up xrd generate examples/postgres/example.yaml --path database/definition.yaml
```
