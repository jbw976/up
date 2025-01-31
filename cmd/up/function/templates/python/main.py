from crossplane.function import resource
from crossplane.function.proto.v1 import run_function_pb2 as fnv1
# Example to add models as import; update as needed
# from model.io.upbound.aws.s3.bucket import v1beta1 as bucketv1beta1


def compose(req: fnv1.RunFunctionRequest, rsp: fnv1.RunFunctionResponse):
    pass
    # Example to retrieve variables from "xr"; update as needed
    # observed_xr = v1alpha1.XBucket(**req.observed.composite.resource)
    # region = "us-west-1"
    # if observed_xr.spec.region is not None:
    #     region = observed_xr.spec.region

    # Example S3 Bucket managed resource configuration; update as needed
    # bucket = v1beta1.Bucket(
    #     apiVersion="s3.aws.upbound.io/v1beta1",
    #     kind="Bucket",
    #     spec=v1beta1.Spec(
    #         forProvider=v1beta1.ForProvider(
    #             region=region,
    #         ),
    #     ),
    # )

    # resource.update(rsp.desired.resources["bucket"], bucket)
