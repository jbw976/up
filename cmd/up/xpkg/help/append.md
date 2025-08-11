The `append` command creates a tarball from a local directory of additional
package assets, such as images or documentation, and appends them to a remote
package.

If your remote image is already signed, this command will invalidate current
signatures and the updated image will need to be re-signed.

#### Examples

Add all files from `./extensions` to a remote image and create a new index with
the extensions included:

```shell
up alpha xpkg-append --extensions-root=./extensions \
    registry.example.com/my-organization/my-repo@sha256:digest
```

Add documentation files to a package and save to a different tag, preserving the
original package at the source reference:

```shell
up alpha xpkg-append --extensions-root=./docs \
    --destination=registry.example.com/my-organization/my-repo:v1.0.1 \
    registry.example.com/my-organization/my-repo@sha256:digest
```
