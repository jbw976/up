{{- if .Imports }}
{{- range .Imports }}
import {{.ImportPath}} as {{.Alias}}
{{- end }}
{{- "\n" }}
{{- end }}

_items = [
    metav1alpha1.OperationTest{
        metadata.name: ""
        spec= {
            operationPath: ""
            assertResources: []
            timeoutSeconds: 60
        }
    }
]
items= _items