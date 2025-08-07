The `export` command exports resources from a Crossplane or Upbound Crossplane
(UXP) cluster to a tarball, for migration to an Upbound Managed Control Plane.

Use the available options to customize the export process, such as specifying
the output file path, including or excluding specific resources and namespaces,
and deciding whether to pause claim,composite,managed resources before
exporting.

#### Examples

Pause all claims, composites, and managed resources before exporting the control
plane state. The state is exported to the default archive file named
`xp-state.tar.gz`. Resources that were already paused will be annotated with
`migration.upbound.io/already-paused: "true"` to preserve their paused state
during the import process:

```shell
up migration export --pause-before-export
```

Export the control plane state to a file called `my-export.tar.gz`:

```shell
up migration export --output=my-export.tar.gz
```

Export the control plane state from only the provided namespaces to the default
file, `xp-state.tar.gz`, with the additional resources specified:

```shell
up migration export --include-extra-resources="customresource.group" \
    --include-namespaces="crossplane-system,team-a,team-b"
```
