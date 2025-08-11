The `mirror` command mirrors all required OCI artifacts for a specific Spaces
version.

#### Examples

Mirror all artifacts for Spaces version 1.9.0 into a local directory as
`.tar.gz` files, using the token file for authentication:

```shell
up space mirror -v 1.9.0 --output-dir=/tmp/output --token-file=upbound-token.json
```

Mirror all artifacts for Spaces version 1.9.0 to a specified container registry,
using the token file for authentication. Note that you must log in to the mirror
registry first using a command like `docker login myregistry.io`:

```shell
up space mirror -v 1.9.0 --destination-registry=myregistry.io --token-file=upbound-token.json
```

Print the artifacts that would be mirrored into a local directory for Spaces
version 1.9.0, using the token file for authentication. A request is made to the
Upbound registry to confirm network access:

```shell
up space mirror -v 1.9.0 --output-dir=/tmp/output --token-file=upbound-token.json --dry-run
```
