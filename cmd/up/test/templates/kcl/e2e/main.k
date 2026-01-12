{{- if .Imports }}
{{- range .Imports }}
import {{.ImportPath}} as {{.Alias}}
{{- end }}
{{- "\n" }}
{{- end }}

_items = [
    metav1alpha1.E2ETest{
        metadata.name: ""
        spec= {
            crossplane.autoUpgrade.channel: "Rapid"
            defaultConditions: ["Ready"]
            manifests: []
            extraResources: []
            # If true, skip cleanup of test resources and control plane after test
            # completion. Useful for debugging and inspecting test state. Defaults to
            # False.
            skipDelete: False
            timeoutSeconds: 4500
        }
    }
]
items= _items