The `run` command executes project tests. By default, only composition tests are
executed; with the `--e2e` flag, only e2e tests are executed.

#### Examples

Run all composition tests located in the 'tests/' directory:

```shell
up test run tests/*
```

Override function annotations for a remote Docker daemon:
```shell
DOCKER_HOST=tcp://192.168.1.100:2376 up test run tests/*  \
	--function-annotations render.crossplane.io/runtime-docker-publish-address=0.0.0.0 \
	--function-annotations render.crossplane.io/runtime-docker-target=192.168.1.100
```


Run all end-to-end (e2e) tests located in the 'tests/' directory:

```shell
up test run tests/* --e2e
```

Run all operation tests located in the 'tests/' directory:

```shell
up test run tests/* --operation
```

Run e2e tests in `tests/` while specifying custom paths for the `kubectl`
binary:

```shell
up test run tests/* --e2e --kubectl=.tools/kubectl
```

Run e2e tests in `tests/`, overriding the default control plane version:

```shell
up test run tests/* --e2e --control-plane-version=v2.0.2-up.5
```
