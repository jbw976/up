The `use` command switches the active Upbound profile and updates the current
kubeconfig context.

This command:

- Sets the specified profile as the default profile
- Updates the kubeconfig to use the last context selected with the profile
- Preserves any existing kubeconfig context information from the profile

Note that if the profile has no associated kubeconfig context (e.g., because `up
ctx` has never been used with the profile), only the profile switch occurs
without kubeconfig updates.

#### Examples

Switch to the `production` profile and update the kubeconfig context:

```shell
up profile use production
```
