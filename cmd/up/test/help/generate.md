The `generate` command creates tests in the specified language.

Supported languages: `kcl` (default), `python`, `go`, `go-templating`, `yaml`

#### Examples

Create a composition test with the default language (KCL) in the folder
`tests/test-xstoragebucket`:

```shell
up test generate xstoragebucket
```

Create a composition test in Python and write it to the folder
`tests/test-xstoragebucket`:

```shell
up test generate xstoragebucket --language python
```

Create an e2e test in Python and write it to the folder
`tests/e2etest-xstoragebucket`:

```shell
up test generate xstoragebucket --language python --e2e
```

Create a composition test in raw YAML and write it
to the folder `tests/test-xstoragebucket`:

```shell
up test generate xstoragebucket --language yaml
```