The `export` command collects billing data from cloud storage and creates a
billing report.

The storage location for the billing data used to create the report is supplied
using the optional `--provider`, `--bucket`, and `--endpoint` flags. If these
flags are missing, their values will be retrieved from the Spaces cluster from
your kubeconfig. Set `--endpoint=""` to use the storage provider's default
endpoint without checking your Spaces cluster for a custom endpoint.

Credentials and other storage provider configuration are supplied according to
the instructions for each provider below.

#### AWS S3

Supply configuration by setting these environment variables: `AWS_REGION`,
`AWS_ACCESS_KEY_ID`, and `AWS_SECRET_ACCESS_KEY`. For more options, see the
documentation at
https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/welcome.html

#### GCP Cloud Storage

Supply credentials by setting the environment variable
`GOOGLE_APPLICATION_CREDENTIALS` with the location of a credential JSON
file. For more options, see the documentation at
https://cloud.google.com/docs/authentication/application-default-credentials.

#### Azure Blob Storage

Supply configuration by setting these environment variables: `AZURE_TENANT_ID`,
`AZURE_CLIENT_ID`, and `AZURE_CLIENT_SECRET`. For more options, see the
documentation at
https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication.

#### Examples

Export a billing report for January 2024 from AWS S3 and write it to a file
called `upbound_billing_report.tgz` in the current directory:

```shell
up space billing export --provider=aws --bucket=my-bucket --account=my-account --billing-month=2024-01
```

Export a billing report for a custom date range from GCP Cloud Storage. Note
that the date range is inclusive (Jan 1-15, 2024):

```shell
up space billing export --provider=gcp --bucket=my-bucket --account=my-account --billing-custom=2024-01-01/2024-01-15
```

Export a billing report for February 2024 from Azure Blob Storage and write it
to a custom output location, `report.tgz`:

```shell
up space billing export --provider=azure --bucket=my-container --azure-storage-account=storage-account --account=my-account --billing-month=2024-02 -o report.tgz
```
