The `create` command creates a new Upbound profile with the specified
configuration.

#### Profile Types

- *cloud* - Profile for connecting to Upbound Cloud (default)
- *disconnected* - Profile for disconnected Spaces environments using local
  kubeconfig

The command automatically switches to the newly created profile unless
`--use=false` is specified. For cloud profiles, an organization must be provided
via the `--organization` flag. Cloud profiles can also be created using `up
login`.

#### Examples

Create a new, logged out cloud profile named `my-profile` for organization
`my-org`:

```shell
up profile create my-profile --organization=my-org
```

Create a new disconnected profile named `local-dev` using the current kubeconfig
context:

```shell
up profile create local-dev --type=disconnected
```

Create a new disconnected profile named `local-dev` using the kubeconfig context
called `my-space`:

```shell
up profile create local-dev --type=disconnected --kubecontext=my-space
```

Create a new profile but don't make it the current profile:

```shell
up profile create staging --organization=my-org --use=false
```
