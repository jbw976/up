The `list` command resolves all transitive dependencies for the current project
or a specific package and outputs a flat, deduplicated list. Each entry shows
the package name, resolved version, and package kind (Provider, Configuration,
Function, etc.).

This is useful for tooling and scripting where a simple array of packages is
needed rather than a tree structure. Use `--format json` or `--format yaml` for
machine-readable output.

#### Examples

List all transitive dependencies for the current project:

```shell
up dependency list
```

List dependencies as JSON for use with other tools:

```shell
up dependency list --format json
```

List all transitive dependencies for a specific package:

```shell
up dependency list xpkg.upbound.io/upbound/platform-ref-aws:v0.8.0
```

Count transitive dependencies:

```shell
up dependency list --format json | jq length
```
