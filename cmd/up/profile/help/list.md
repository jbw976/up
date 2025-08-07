The `list` command displays all configured Upbound profiles in a table format.

#### Output Columns

- **CURRENT**: Indicates the active profile with an asterisk (*)
- **NAME**: The name of each profile
- **TYPE**: Profile type (cloud or disconnected)
- **ORGANIZATION**: The organization associated with the profile (may be empty
  for disconnected profiles)

The profiles are listed in alphabetical order by name.

#### Examples

Show all configured profiles:

```shell
up profile list
```
