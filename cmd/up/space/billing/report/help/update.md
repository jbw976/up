This command takes a billing report produced by 'up space billing export'
and adds its data to a target billing report.
An existing target report is updated in place.
If the target report doesn't exist, it will be created using the data from the
source tarball.

The target report can be stored on the local file system or in cloud storage.

Credentials and other storage provider configuration are supplied according to
the instructions for each provider below.

#### AWS S3

Supply configuration by setting these environment variables: AWS_REGION,
AWS_ACCESS_KEY_ID, and AWS_SECRET_ACCESS_KEY.
For more options, see the documentation at
https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/welcome.html

#### GCP Cloud Storage

Supply credentials by setting the environment variable
GOOGLE_APPLICATION_CREDENTIALS with the location of a credential JSON
file.
For more options, see the documentation at
https://cloud.google.com/docs/authentication/application-default-credentials.

#### Examples

Update a local billing report with data from a new export:

```shell
up space billing report update existing_report.tgz new_data.tgz
```

Update a billing report stored in AWS S3:

```shell
up space billing report update reports/existing_report.tgz new_data.tgz --provider=aws --bucket=my-bucket
```

Update a report in GCP Cloud Storage:

```shell
up space billing report update reports/existing_report.tgz new_data.tgz --provider=gcp --bucket=my-bucket
```
