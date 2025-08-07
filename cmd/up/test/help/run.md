The `run` command executes project tests. By default, only composition tests are
executed; with the `--e2e` flag, only e2e tests are executed.

#### Examples

Run all composition tests located in the 'tests/' directory:

```shell
up test run tests/*
```

Run all end-to-end (e2e) tests located in the 'tests/' directory:

```shell
up test run tests/* --e2e
```

Run e2e tests in `tests/` while specifying custom paths for the `kubectl`
binary:

```shell
up test run tests/* --e2e --kubectl=.tools/kubectl
```
