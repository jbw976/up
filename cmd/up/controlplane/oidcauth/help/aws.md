The `oidc-auth` command sets up OIDC authentication between an Upbound Cloud
Control Plane and AWS using an AWS IAM Identity Provider.

This command requires the AWS CLI.

#### Examples

Check if the IAM IdentityProvider `proidc.upbound.io` exists and create it if
needed. Create an IAM Role trusted by the identity provider and attach the
`AdministratorAccess` policy. Configure the control plane with a
`ProviderConfig` for `provider-aws`:

```shell
up ctp oidc-auth aws example-project-aws-up-cli arn:aws:iam::aws:policy/AdministratorAccess
```

Check if the IAM IdentityProvider `proidc.upbound.io` exists and create it if
needed. Create an IAM Role with a trust policy using a wildcard match
(`StringLike`) on `sub`. Useful for allowing access from multiple control planes
matching the pattern:

```shell
up ctp oidc-auth aws example-project-aws-up-cli arn:aws:iam::aws:policy/AdministratorAccess \
    --sub 'example-*'
```

Check if the IAM IdentityProvider `example.upbound.io` exists and create it if
needed. Create an IAM Role trusted by the specified identity provider and attach
the `AdministratorAccess` policy. Configure the control plane with the
appropriate `ProviderConfig` for provider-aws:

```shell
up ctp oidc-auth aws example-project-aws-up-cli arn:aws:iam::aws:policy/AdministratorAccess \
    --oidc-provider-name example.upbound.io
```

Show the AWS CLI commands that would be executed to set up OIDC without actually
running them:

```shell
up ctp oidc-auth aws example-project-aws-up-cli arn:aws:iam::aws:policy/AdministratorAccess \
    --dry-run
```
