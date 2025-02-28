{{- if .Imports }}
{{- range .Imports }}
import {{.ImportPath}} as {{.Alias}}
{{- end }}
{{- "\n" }}
{{- end }}

_items = [
    metav1alpha1.CompositionTest{
        metadata.name: ""
        spec= {
            assertResources: []
            compositionPath: ""
            xrPath: ""
            xrdPath: ""
            timeoutSeconds: 60
            validate: False
        }
    }
]
items= _items