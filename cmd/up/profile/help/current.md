The `current` command displays the currently active Upbound profile and its
configuration.

This command outputs JSON-formatted information about the active profile,
including:

- Profile name
- Profile type (cloud or disconnected)
- Organization (for cloud profiles)
- Domain configuration
- Other profile settings (with sensitive data redacted)

#### Examples

Show the current active profile configuration in JSON format:

```shell
up profile current
```
