{{- if .Imports }}
{{- range .Imports }}
import {{.ImportPath}} as {{.Alias}}
{{- end }}
{{- "\n" }}
{{- end }}

# _items = [
#     metav1alpha1.CompositionTest{
#         spec= {
#             assertResources: []
#             composition: ""
#             xr: ""
#             xrd: ""
#             context: []
#             extraResources: []
#             observedResources: []
#             timeoutSeconds: 60
#             validate: False
#         }
#     }
# ]
# items = _items