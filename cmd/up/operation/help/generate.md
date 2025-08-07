The `generate` command creates a new, empty operation.

#### Examples

Generates a new, empty, one-shot operation named `my-operation`:

```shell
up operation generate my-operation
```

Generate a new, empty cron operation named `my-operation` that runs every hour:

```shell
up operation generate my-operation --cron "0 0 * * *"
```

Generate a new, empty watch operation named `my-operation` triggered by changes
to Deployments in the namespace `my-namespace`:

```shell
up operation generate my-operation --watch-group-version-kind "apps/v1/Deployment" \
    --watch-namespace "my-namespace"
```

Generate a new operation named `claude-pod-watcher` that invokes a Claude prompt
when pods in the `default` namespace change:

```shell
up operation generate claude-pod-watcher --watch-group-version-kind "apps/v1/Pod" \
    --watch-namespace "default" --functions xpkg.upbound.io/upbound/function-claude
```
