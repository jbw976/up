The `upgrade` command upgrades a project to a newer API version.

#### Examples

Upgrade the project in the current directory to the latest supported version:

```shell
up project upgrade
```

Upgrade a project with a custom project file name:

```shell
up project upgrade --project-file custom-project.yaml
```

#### Supported Upgrade Paths

- `v1alpha1` → `v2alpha1`: Adds Crossplane v2 features.
