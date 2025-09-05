from .model.io.upbound.dev.meta.operationtest import v1alpha1 as operationtest
from .model.io.k8s.apimachinery.pkg.apis.meta import v1 as k8s

test = operationtest.OperationTest(
    metadata=k8s.ObjectMeta(
        name="",
    ),
    spec = operationtest.Spec(
        operationPath="",
        assertResources=[],
        timeoutSeconds=120,
    )
)