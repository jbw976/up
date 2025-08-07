The `render` command shows you what composed resources Crossplane would create
by printing them to stdout. It also prints any changes that would be made to the
status of the XR. It doesn't talk to Crossplane. Instead it runs the Composition
Function pipeline specified by the Composition locally, and uses that to render
the XR.

#### Examples

Simulate creating a new XR:

```shell
up composition render composition.yaml xr.yaml
```

Simulate updating an XR that already exists:

```shell
up composition render composition.yaml xr.yaml \
    --observed-resources=existing-observed-resources.yaml
```

Pass context values to the Function pipeline:

```shell
up composition render composition.yaml xr.yaml \
    --context-values=apiextensions.crossplane.io/environment='{"key": "value"}'
```

Pass extra resources requested by functions in the pipeline:

```shell
up composition render composition.yaml xr.yaml \
    --extra-resources=extra-resources.yaml
```

Pass credentials needed by functions in the pipeline:

```shell
up composition render composition.yaml xr.yaml \
    --function-credentials=credentials.yaml
```

#### Docker Configuration

The render command uses Docker (or any Docker-compatible container runtime) to
run composition functions. Configure the Docker connection using these standard
environment variables:

* `DOCKER_HOST`:        Docker daemon socket (e.g., `unix:///var/run/docker.sock`)
* `DOCKER_API_VERSION`: Docker API version to use
* `DOCKER_CERT_PATH`:   Path to Docker TLS certificates
* `DOCKER_TLS_VERIFY`:  Enable TLS verification (1 or 0)
