The `group` command interacts with groups within the current Space. Use the `up
profile` command to switch between different Upbound profiles and the `up ctx`
command to switch between Spaces within a Cloud profile.

#### Examples

List all groups in the current Space:

```shell
up group list
```

Create a new group named `my-group` to organize control planes within a Space:

```shell
up group create my-group
```

Get details about a specific group called `my-group`, including configuration
and metadata:

```shell
up group get my-group
```

Delete the group called `my-group`, which must not be protected:

```shell
up group delete my-group
```
