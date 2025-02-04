{{- if .Imports }}
{{- range .Imports }}
import {{.ImportPath}} as {{.Alias}}
{{- end }}
{{- "\n" }}
{{- end }}

# _items = [
#     metav1alpha1.E2ETest{
#         spec = {
#             crossplane.autoUpgrade.channel = "Rapid"
#             defaultConditions= ["Ready"]
#             manifests= []
#             extraResources= []
#             skipDelete: False
#             timeoutSeconds: 4500
#         }
#     }
# ]
# items = _items