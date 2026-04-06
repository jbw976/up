module github.com/upbound/up

go 1.25.5

require (
	cloud.google.com/go/storage v1.59.1
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.21.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.1
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.2
	github.com/Masterminds/semver/v3 v3.4.0
	github.com/alecthomas/assert/v2 v2.11.0
	github.com/alecthomas/chroma v0.10.0
	github.com/alecthomas/kong v1.13.0
	github.com/aws/aws-sdk-go-v2 v1.41.5
	github.com/aws/aws-sdk-go-v2/config v1.32.14
	github.com/aws/aws-sdk-go-v2/service/s3 v1.98.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.10
	github.com/blang/semver/v4 v4.0.0
	github.com/charmbracelet/bubbles v0.21.1-0.20250623103423-23b8fd6302d7
	github.com/charmbracelet/glamour v0.10.0
	github.com/charmbracelet/huh v0.8.0
	github.com/charmbracelet/huh/spinner v0.0.0-20251124111010-6575a6e28cb3
	github.com/crossplane/crossplane-runtime/v2 v2.2.0-rc.0.0.20260203080537-a4cdda495567
	github.com/crossplane/crossplane/v2 v2.2.0-rc.0.0.20260206124132-26d50f66f5fd
	github.com/crossplane/crossplane/v2/xcrd v0.0.0
	github.com/crossplane/uptest/v2 v2.2.1-0.20260224131307-4a16ecb4006f
	github.com/docker/docker-credential-helpers v0.9.4
	github.com/gdamore/tcell/v2 v2.8.1
	github.com/getkin/kin-openapi v0.133.0
	github.com/goccy/go-yaml v1.12.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/golang/tools v0.32.0
	github.com/google/addlicense v1.1.1
	github.com/google/go-cmp v0.7.0
	github.com/google/go-containerregistry v0.20.7
	github.com/google/ko v0.17.1
	github.com/google/uuid v1.6.0
	github.com/invopop/jsonschema v0.13.0
	github.com/kubernetes-sigs/kro v0.8.1
	github.com/kyverno/chainsaw v0.2.14-0.20251026071850-cfa8bb38e7b2
	github.com/kyverno/kyverno-json v0.0.4-0.20241008103124-b294ee72a2bf
	github.com/mesosphere/dkp-cli-runtime/core v0.7.3
	github.com/mhrabovcin/troubleshoot-live v0.0.20
	github.com/oapi-codegen/oapi-codegen/v2 v2.4.1
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/posener/complete v1.2.3
	github.com/pterm/pterm v0.12.82
	github.com/r3labs/diff/v3 v3.0.2
	github.com/radovskyb/watcher v1.0.7
	github.com/replicatedhq/troubleshoot v0.123.17-0.20260114203639-06a8692de5ac
	github.com/rivo/tview v0.0.0-20241227133733-17b7edb88c57
	github.com/sourcegraph/go-lsp v0.0.0-20240223163137-f80c5dd31dfd
	github.com/sourcegraph/jsonrpc2 v0.2.1
	github.com/spf13/afero v1.15.0
	github.com/upbound/controller-manager v0.1.0-rc.0.0.20250820091445-60634b1a874f
	github.com/upbound/up-sdk-go v1.15.0
	github.com/upbound/up-sdk-go/apis v1.15.0
	github.com/upbound/up/pkg/migration v0.0.0-20250813115036-f191c0e8fffc
	github.com/upbound/uxp-licensing v0.0.0-20250818124243-1f3741528237
	github.com/willabides/kongplete v0.4.0
	github.com/wk8/go-ordered-map/v2 v2.1.8
	golang.org/x/sync v0.19.0
	golang.org/x/term v0.39.0
	google.golang.org/api v0.267.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools/v3 v3.5.2
	helm.sh/helm/v3 v3.19.4
	k8s.io/api v0.35.1
	k8s.io/apiextensions-apiserver v0.35.0
	k8s.io/apimachinery v0.35.1
	k8s.io/client-go v0.35.1
	k8s.io/kube-openapi v0.0.0-20251125145642-4e65d59e963e
	k8s.io/kubectl v0.34.2
	k8s.io/utils v0.0.0-20260108192941-914a6e750570
	oras.land/oras-go/v2 v2.6.0
	sigs.k8s.io/controller-runtime v0.23.0
	sigs.k8s.io/e2e-framework v0.6.0
	sigs.k8s.io/kind v0.30.0
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/charmbracelet/bubbletea v1.3.10
	github.com/charmbracelet/lipgloss v1.1.1-0.20250404203927-76690c660834
	github.com/distribution/reference v0.6.0 // indirect
	github.com/dlclark/regexp2 v1.11.0 // indirect
	github.com/gobuffalo/flect v1.0.3
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	rsc.io/qr v0.2.0 // indirect
	sigs.k8s.io/controller-tools v0.19.0 // indirect
)

require (
	al.essio.dev/pkg/shellescape v1.6.0 // indirect
	cel.dev/expr v0.25.1 // indirect
	cloud.google.com/go/auth v0.18.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	cyphar.com/go-pathrs v0.2.1 // indirect
	filippo.io/edwards25519 v1.1.1 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.30 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.24 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.13 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.7 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.2 // indirect
	github.com/Azure/go-autorest/tracing v0.6.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.30.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.54.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.54.0 // indirect
	github.com/IGLOU-EU/go-wildcard v1.0.3 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/STARRY-S/zip v0.2.1 // indirect
	github.com/ahmetalpbalkan/go-cursor v0.0.0-20131010032410-8136607ea412 // indirect
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/alecthomas/repr v0.5.2 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/apparentlymart/go-cidr v1.1.0 // indirect
	github.com/aquilax/truncate v1.0.1 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.8 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.14 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.51.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.38.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.19 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.11.0 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/c9s/goprocinfo v0.0.0-20170724085704-0010a05ce49f // indirect
	github.com/casbin/govaluate v1.10.0 // indirect
	github.com/catppuccin/go v0.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/charmbracelet/colorprofile v0.3.1 // indirect
	github.com/charmbracelet/x/ansi v0.10.1 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13 // indirect
	github.com/charmbracelet/x/exp/slice v0.0.0-20250327172914-2fdc97757edf // indirect
	github.com/charmbracelet/x/exp/strings v0.0.0-20240722160745-212f7b056ed0 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20230304212654-82a0ddb27589 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/cilium/ebpf v0.20.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/cncf/xds/go v0.0.0-20251210132809-ee656c7534f5 // indirect
	github.com/containerd/cgroups/v3 v3.1.2 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.1 // indirect
	github.com/coreos/go-oidc/v3 v3.17.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.6.0 // indirect
	github.com/crossplane/crossplane-runtime v1.21.0-rc.0.0.20250719014028-ab24452c43ea // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467 // indirect
	github.com/digitorus/pkcs7 v0.0.0-20230818184609-3a137a874352 // indirect
	github.com/digitorus/timestamp v0.0.0-20231217203849-220c5c2851b7 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/distribution/distribution/v3 v3.0.0 // indirect
	github.com/dprotaso/go-yit v0.0.0-20220510233725-9ba8df137936 // indirect
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/dustinkirkland/golang-petname v0.0.0-20240428194347-eebcea082ee0 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/emicklei/dot v1.10.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.36.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.0 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/external-secrets/external-secrets v0.19.2 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-chi/chi/v5 v5.2.4 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/analysis v0.24.1 // indirect
	github.com/go-openapi/errors v0.22.6 // indirect
	github.com/go-openapi/loads v0.23.2 // indirect
	github.com/go-openapi/runtime v0.29.2 // indirect
	github.com/go-openapi/spec v0.22.3 // indirect
	github.com/go-openapi/strfmt v0.25.0 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-openapi/validate v0.25.1 // indirect
	github.com/go-redis/redis/v7 v7.4.1 // indirect
	github.com/go-sql-driver/mysql v1.9.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/certificate-transparency-go v1.3.2 // indirect
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20250115185438-c4dd792fa06c // indirect
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20250115185438-c4dd792fa06c // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/gorilla/handlers v1.5.2 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.1-0.20210315223345-82c243799c99 // indirect
	github.com/hashicorp/aws-sdk-go-base/v2 v2.0.0-beta.70 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-getter v1.8.4 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/in-toto/attestation v1.1.2 // indirect
	github.com/in-toto/in-toto-golang v0.10.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.8.0 // indirect
	github.com/jedisct1/go-minisign v0.0.0-20230811132847-661be99b8267 // indirect
	github.com/jmespath-community/go-jmespath v1.1.2-0.20240930152130-6eb5a346873f // indirect
	github.com/jwalton/gchalk v1.3.0 // indirect
	github.com/jwalton/go-supportscolor v1.1.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/kyverno/pkg/ext v0.0.0-20250303002756-48769d003e55 // indirect
	github.com/letsencrypt/boulder v0.20251110.0 // indirect
	github.com/longhorn/go-iscsi-helper v0.0.0-20210330030558-49a327fb024e // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mholt/archives v0.1.1 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/microsoft/go-mssqldb v1.9.5 // indirect
	github.com/miekg/dns v1.1.70 // indirect
	github.com/minio/minlz v1.0.0 // indirect
	github.com/mistifyio/go-zfs/v4 v4.0.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/nozzle/throttler v0.0.0-20180817012639-2ea982251481 // indirect
	github.com/nwaples/rardecode/v2 v2.2.0 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/opencontainers/selinux v1.13.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pjbgf/sha1cd v0.4.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sahilm/fuzzy v0.1.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/sassoftware/relic v7.2.1+incompatible // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.10.0 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/shirou/gopsutil/v4 v4.25.12 // indirect
	github.com/sigstore/cosign/v2 v2.4.1 // indirect
	github.com/sigstore/cosign/v3 v3.0.5 // indirect
	github.com/sigstore/protobuf-specs v0.5.0 // indirect
	github.com/sigstore/rekor v1.5.0 // indirect
	github.com/sigstore/rekor-tiles/v2 v2.2.0 // indirect
	github.com/sigstore/sigstore v1.10.4 // indirect
	github.com/sigstore/sigstore-go v1.1.4 // indirect
	github.com/sigstore/timestamp-authority/v2 v2.0.4 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/sorairolake/lzip-go v0.3.5 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/speakeasy-api/openapi-overlay v0.9.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/sylabs/sif/v2 v2.22.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/tchap/go-patricia/v2 v2.3.3 // indirect
	github.com/therootcompany/xz v1.0.1 // indirect
	github.com/theupdateframework/go-tuf v0.7.0 // indirect
	github.com/theupdateframework/go-tuf/v2 v2.4.1 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/transparency-dev/formats v0.0.0-20251017110053-404c0d5b696c // indirect
	github.com/transparency-dev/merkle v0.0.2 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	github.com/vmware-tanzu/velero v1.17.1 // indirect
	github.com/woodsbury/decimal128 v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	github.com/yuin/goldmark-emoji v1.0.5 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zach-klippenstein/goregen v0.0.0-20160303162051-795b5e3961ea // indirect
	go.etcd.io/etcd/api/v3 v3.6.5 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.6.5 // indirect
	go.etcd.io/etcd/client/v3 v3.6.5 // indirect
	go.mongodb.org/mongo-driver v1.17.6 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.39.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.63.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.podman.io/image/v5 v5.38.0 // indirect
	go.podman.io/storage v1.61.1-0.20260113143440-ec033ce69f13 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	k8s.io/code-generator v0.35.0 // indirect
	k8s.io/component-helpers v0.34.2 // indirect
	k8s.io/gengo/v2 v2.0.0-20250922181213-ec3ebc5fd46b // indirect
	k8s.io/kubelet v0.35.0 // indirect
	k8s.io/metrics v0.35.0 // indirect
	periph.io/x/host/v3 v3.8.5 // indirect
	sigs.k8s.io/controller-runtime/tools/setup-envtest v0.0.0-20241206182001-aea2e32a9365 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/kubectl-validate v0.0.5-0.20250909135530-dad770c1cb46 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)

require (
	atomicgo.dev/cursor v0.2.0 // indirect
	atomicgo.dev/keyboard v0.2.9 // indirect
	atomicgo.dev/schedule v0.1.0 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/Masterminds/squirrel v1.5.4 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go-v2/service/iam v1.53.7
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.8.1 // indirect
	github.com/brianvoe/gofakeit/v6 v6.28.0
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/charmbracelet/harmonica v0.2.0 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/containerd/containerd v1.7.29 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.18.1 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/cli v29.2.0+incompatible
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v28.5.2+incompatible
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.8.0
	github.com/go-git/go-git/v5 v5.17.1
	github.com/go-gorp/gorp/v3 v3.1.0 // indirect
	github.com/go-logr/logr v1.4.3
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.4 // indirect
	github.com/go-openapi/jsonreference v0.21.4 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/cel-go v0.26.1 // indirect
	github.com/google/gnostic-models v0.7.1
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.11 // indirect
	github.com/googleapis/gax-go/v2 v2.17.0 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/gosuri/uitable v0.0.4 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.5 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lithammer/fuzzysearch v1.1.8 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mdp/qrterminal/v3 v3.2.1
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/riywo/loginshell v0.0.0-20200815045211-7d26008be1ab // indirect
	github.com/rubenv/sql-migrate v1.8.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/vbatts/tar-split v0.12.2 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0
	go.opentelemetry.io/otel/trace v1.40.0
	go.opentelemetry.io/proto/otlp v1.8.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/mod v0.32.0
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sys v0.40.0
	golang.org/x/text v0.33.0
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.40.0
	golang.org/x/xerrors v0.0.0-20240716161551-93cc26a95ae9 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/genproto v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260203192932-546029d2fa20 // indirect
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/apiserver v0.35.0 // indirect
	k8s.io/cli-runtime v0.35.0
	k8s.io/component-base v0.35.0 // indirect
	k8s.io/klog/v2 v2.130.1
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.33.0 // indirect
	sigs.k8s.io/kustomize/api v0.20.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.20.1 // indirect
)

replace (
	github.com/crossplane/crossplane/v2/xcrd => ./internal/vendor/github.com/crossplane/crossplane/xcrd
	github.com/golang/tools => ./internal/vendor/golang.org/x/tools
	sigs.k8s.io/controller-runtime v0.23.0 => sigs.k8s.io/controller-runtime v0.22.4
)

// use the local one
replace github.com/upbound/up/pkg/migration => ./pkg/migration

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go v1.2.11

// Prevents using up-sdk-go with go workspaces.
exclude google.golang.org/grpc/stats/opentelemetry v0.0.0-20240907200651-3ffb98b2c93a
