The `get` command retrieves resources from a control plane via the Query API.

#### Examples

List all bucket resources in a table:

```shell
up alpha get buckets
```

List all bucket resources in a table with more information:

```shell
up alpha get buckets -o wide
```

List a single bucket with specified NAME:

```shell
up alpha get bucket web-bucket-13je7
```

List bucket resources in JSON output format, in the `v1` version of the
`s3.aws.upbound.io` API group:

```shell
up alpha get buckets.v1.s3.aws.upbound.io -o json
```

List a single bucket in JSON output format:

```shell
up alpha get -o json bucket `web-bucket-13je7`
```

Return only the external-name value of the specified bucket:

```shell
up alpha get -o template bucket/web-bucket-13je7 --template='{{.metadata.annotations.external-name}}'
```

List resource information with custom columns:

```shell
up alpha get bucket test-bucket -o custom-columns=NAME:.spec.forProvider.name,SIZE:.status.atProvider.size
```

List all buckets and vpcs together:

```shell
up alpha get buckets,vpcs
```

List only the vpc called `prod` and the bucket called `backup`:

```shell
up alpha get vpc/prod bucket/backup
```
