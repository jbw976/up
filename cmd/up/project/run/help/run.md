The `run` command builds and runs a project on a development control plane for
testing.

This command:

- Builds all embedded functions defined in the project
- Creates or uses an existing development control plane
- Pushes packages to the container registry
- Installs the project configuration on the control plane
- Updates kubeconfig to use the development control plane

#### Development Control Planes

There are two kinds of development control planes:

1. Local development control planes, which run in a KIND cluster on the
   development machine.
2. Cloud development control planes, which run in Upbound Cloud Spaces.

Cloud development control planes are used by default when the current `up`
context is an Upbound Cloud Space. Use `up ctx` to update the current context.
Local development control planes are used by default otherwise, and can be
explicitly requested using the `--local` flag.

Local development control planes always use UXP v2.0 or newer, defaulting to the
latest version available. The default UXP version for cloud development control
planes depends on your project version: v1.x for v1alpha1 projects or v2.x for
v2alpha1 projects. The default version can be overridden with the
`--control-plane-version` flag.

It is also possible to run a project on an arbitrary UXP cluster referenced by
the current kubeconfig context by using the `--use-current-context` flag. Note
that this can be destructive, as it will create resources and install packages
in your cluster; it is not recommended to use `up project run` on shared or
production clusters.

#### Examples

Run the project using the default development control plane type (see above):

```shell
up project run
```

Run the project on a control plane with a specific name, using the default
type. The control plane will be created if it doesn't exist:

```shell
up project run --control-plane-name=my-dev-cp
```

Force a local development control plane to be used instead of a cloud
development control plane:

```shell
up project run --local
```

Create a local development control plane with an ingress controller enabled.
The Web UI will be accessible at localhost on a randomly assigned port:

```shell
up project run --local --ingress
```

Create a local development control plane with ingress mapped to specific port.
The Web UI will be accessible at http://127-0-0-1.nip.io:8080:

```shell
up project run --local --ingress --ingress-port=8080:80
```

Run a project using the current kubeconfig context:

```shell
up project run --use-current-context
```

Override the repository specified in the project file to push to a different
container registry. Note that when using a local development control plane
packages are side-loaded, avoiding the need to push:

```shell
up project run --repository=xpkg.upbound.io/example/my-project
```

Run on a Spaces control plane with a specific name, allowing a non-development
control plane to be used. This works with disconnected Spaces as well as Cloud
Spaces:

```shell
up project run --force --control-plane-name=my-cp
```

Override the default UXP version used for a Spaces development control plane,
for example to test a v1 project on a v2 control plane:

```shell
up project run --control-plane-version=v1.20.1-up.1
```
