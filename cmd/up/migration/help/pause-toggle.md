The `pause-toggle` command allows you to manage the paused state of resources
after a migration attempt.

- When `--pause=true`, all resources in the *target control plane* will be
  paused due to a faulty migration. This is useful after running `migration
  import --unpause-after-import=true` and discovering issues in the target.
- When `--pause=false`, only resources paused during the migration will be
  *unpaused in the source control plane*, ensuring that pre-existing paused
  resources remain unchanged.

#### Examples

Pause all resources in the *target control plane* after a migration if the
import caused issues. Useful for stopping resources in a faulty target
environment:

```shell
up migration pause-toggle --pause=true
```

Unpause only the resources that were paused in the *source control plane* due to
migration. This is helpful when reverting migration-induced pauses in the source
after a failed import to the target.

```shell
up migration pause-toggle --pause=false
```
