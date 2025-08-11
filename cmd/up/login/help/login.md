The `login` command authenticates with Upbound Cloud and stores session
credentials.

#### Authentication Methods

- **Web Browser (default)** - Opens browser for OAuth authentication
- **Device Code** - Use --use-device-code for headless environments
- **Username/Password** - Provide --username and --password flags
- **Personal Access Token or Robot Token** - Provide --token flag

The command creates or updates a profile with the authenticated session. If no
profile name is specified, it uses the currently active profile. A profile named
`default` will be created if no profiles exist.

#### Examples

Open a browser for OAuth authentication (recommended):

```shell
up login
```

Prompt for password and authenticate with credentials:

```shell
up login --username=user@example.com
```

Authenticate using a personal access token.

```shell
up login --token=upat_xxxxx
```

Use the device code flow for headless/remote environments:

```shell
up login --use-device-code
```

Authenticate and create or update the `production` profile:

```shell
up login --profile=production --organization=my-org
```
