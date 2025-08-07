The `generate` command creates an embedded function in the specified language,
and optionally adds it to a composition or operation pipeline.

#### Examples

Create a function with the default language (go-templating) in the folder
`functions/fn1`:

```shell
up function generate fn1
```

Create a Python function in the folder `functions/fn2`:

```shell
up function generate fn2 --language python
```

Create a KCL function in the folder `functions/compose-xcluster` and add it as a
composition pipeline step in the given composition file:

```shell
up function generate compose-xcluster apis/xcluster/composition.yaml --language kcl
```

Creates a Go function in the folder `functions/check-pod-logs` and add it as a
pipeline step to the operation in `operations/watch-pods/operation.yaml`:

```shell
up function generate check-pod-logs operations/watch-pods/operation.yaml --language go
```
