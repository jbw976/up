from .model.io.upbound.dev.meta.compositiontest import v1alpha1 as compositiontest
from .model.io.k8s.apimachinery.pkg.apis.meta import v1 as k8s

test = compositiontest.CompositionTest(
    metadata=k8s.ObjectMeta(
        name="",
    ),
    spec = compositiontest.Spec(
        assertResources=[],
        compositionPath="",
        xrPath="",
        xrdPath="",
        timeoutSeconds=120,
        validate=False,
    )
)
