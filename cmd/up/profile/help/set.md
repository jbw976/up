The `set` command updates configuration values for an Upbound profile.

#### Available Keys

- *organization* - Sets the organization for the current profile

#### Examples

Set the default organization to `my-org` for the current profile:

```shell
up profile set organization `my-org`
```

Set the default organization to `other-org` for the profile called `production`:

```shell
up profile set organization my-org --profile=production
```
