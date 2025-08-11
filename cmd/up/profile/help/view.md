The `view` command displays all configured Upbound profiles in JSON format.

This command outputs detailed information about all profiles, including:

- Profile names as keys
- Profile configuration details (with sensitive data redacted)
- Profile type, organization, domain, and other settings

The output is formatted as indented JSON for easy reading and processing.

#### Examples

Show all profiles in JSON format:

```shell
up profile view
```

Use `jq` to show only the `production` profile:

```shell
up profile view | jq '.["production"]'
```
