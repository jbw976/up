The `render` command shows you what resources an Operation would create or mutate
by printing them to stdout. It also prints any changes that would be made to the
status of the Operation. It doesn't talk to Crossplane. Instead it runs the Operation
Function pipeline specified by the Operation locally, and uses that to render
the Operation.

#### Examples

Render an Operation:

```shell
up operation render operations/op1/operation.yaml
```

Pass context values to the Function pipeline:

```shell
up operation render operations/op1/operation.yaml \
    --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'
```

Pass required resources requested by functions in the pipeline:

```shell
up operation render operations/op1/operation.yaml \
    --required-resources=required-resources.yaml
```

Pass credentials needed by functions in the pipeline:

```shell
up operation render operations/op1/operation.yaml \
    --function-credentials=credentials.yaml
```

Include function results and context in the output:

```shell
up operation render operations/op1/operation.yaml -f -c
```

Include the full Operation with original spec and metadata:

```shell
up operation render operations/op1/operation.yaml -o
```

#### Docker Configuration

The render command uses Docker (or any Docker-compatible container runtime) to
run operation functions. Configure the Docker connection using these standard
environment variables:

* `DOCKER_HOST`:        Docker daemon socket (e.g., `unix:///var/run/docker.sock`)
* `DOCKER_API_VERSION`: Docker API version to use
* `DOCKER_CERT_PATH`:   Path to Docker TLS certificates
* `DOCKER_TLS_VERIFY`:  Enable TLS verification (1 or 0)