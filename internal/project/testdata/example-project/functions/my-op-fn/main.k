import models.io.upbound.aws.s3.v1beta1 as s3v1beta1
import models.com.example.platform.v1alpha1.xstoragebucket as xstoragebucketv1alpha1

oxr = xstoragebucketv1alpha1.XStorageBucket{**option("params").oxr}

bucketName = "{}-bucket".format(oxr.metadata.name)

_metadata = lambda name: str -> any {
  {
    name = name
    annotations = {
      "krm.kcl.dev/composition-resource-name" = name
    }
  }
}

_items: [any] = [
    # Bucket in the desired region
    s3v1beta1.Bucket{
        metadata: _metadata(bucketName)
        spec = {
            forProvider = {
                region = oxr.spec.parameters.region
            }
        }
    },
    s3v1beta1.BucketOwnershipControls{
        metadata: _metadata("{}-boc".format(oxr.metadata.name))
        spec = {
            forProvider = {
                bucketRef = {
                    name = bucketName
                }
                region = oxr.spec.parameters.region
                rule:[{
                    objectOwnership:"BucketOwnerPreferred"
                }]
            }
        }
    },
    s3v1beta1.BucketPublicAccessBlock{
        metadata: _metadata("{}-pab".format(oxr.metadata.name))
        spec = {
            forProvider = {
                bucketRef = {
                    name = bucketName
                }
                region = oxr.spec.parameters.region
                blockPublicAcls: False
                ignorePublicAcls: False
                restrictPublicBuckets: False
                blockPublicPolicy: False
            }
        }
    },
    # ACL for the bucket
    s3v1beta1.BucketACL{
        metadata: _metadata("{}-acl".format(oxr.metadata.name))
        spec = {
            forProvider = {
                bucketRef = {
                    name = bucketName
                }
                region = oxr.spec.parameters.region
                acl = oxr.spec.parameters.acl
            }
        }
    },
    # Default encryption for the bucket
    s3v1beta1.BucketServerSideEncryptionConfiguration{
        metadata: _metadata("{}-encryption".format(oxr.metadata.name))
        spec = {
            forProvider = {
                region = oxr.spec.parameters.region
                bucketRef = {
                    name = bucketName
                }
                rule = [
                    {
                        applyServerSideEncryptionByDefault = [
                            {
                                sseAlgorithm = "AES256"
                            }
                        ]
                        bucketKeyEnabled = True
                    }
                ]
            }
        }
    }
]

# Set up versioning for the bucket if desired
if oxr.spec.parameters.versioning:
    _items += [
        s3v1beta1.BucketVersioning{
            metadata: _metadata("{}-versioning".format(oxr.metadata.name))
            spec = {
                forProvider = {
                    region = oxr.spec.parameters.region
                    bucketRef = {
                        name = bucketName
                    }
                    versioningConfiguration = [
                        {
                            status = "Enabled"
                        }
                    ]
                }
            }
        }
    ]

items = _items
