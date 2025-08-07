The `trace` command traces the relationship between resources in a control
plane.

#### Examples

Trace all buckets, showing relationships and dependencies:

```shell
up alpha trace buckets
```

Trace all Crossplane claims, displaying claim to composite resource
relationships:

```shell
up alpha trace claims
```

Trace buckets and vpcs, showing relationships between multiple resource types:

```shell
up alpha trace buckets,vpc
```

Trace only the buckets called `prod` and `staging`:

```shell
up alpha trace buckets prod staging
```

Trace only the bucket `prod` and the vpc `default`:

```shell
up alpha trace bucket/prod vpc/default
```
