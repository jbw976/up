The `clean-cache` command removes all cached package images from the local cache
directory.  This can help free up disk space or resolve issues with corrupted
cache entries.

#### Examples

Clean the default cache directory (~/.up/cache/), removing all cached package
images:

```shell
up dependency clean-cache
```

Clean a custom cache directory, for example in a CI/CD environment where a
shared cache is used:

```shell
up dependency clean-cache --cache-dir /path/to/cache
```
