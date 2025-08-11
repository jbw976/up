The `logout` command invalidates the current session and removes stored
credentials.

This command:

- Invalidates the session token with Upbound Cloud
- Removes the session token from the local profile configuration
- Keeps the profile configuration intact (only removes authentication)

Note that this affects only a single profile. Other profiles remain
authenticated.

After logout, you can log back in using `up login` to re-authenticate with the
same profile.

#### Examples

Log out the active profile:

```shell
up logout
```

Log out the `production` profile:

```shell
up logout --profile=production
```
