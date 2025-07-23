// Copyright 2025 Upbound Inc.
// All rights reserved

package generator

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
)

// TestTransformStructureKcl tests reorganizing files and adjusting imports.
func TestTransformStructureKcl(t *testing.T) {
	// Test case structure
	tests := []struct {
		name           string
		setupFs        func(fs afero.Fs) // Setup for the filesystem
		sourceDir      string
		targetDir      string
		expectedFiles  map[string]string // expected file paths and their content
		expectedErrors bool
	}{
		{
			name: "TransformStructureKcl",
			setupFs: func(fs afero.Fs) {
				afero.WriteFile(fs, kclModelsFolder+"/kcl.mod", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/kcl.mod.lock", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/k8s/apimachinery/pkg/apis/meta/v1/managed_fields_entry.k", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/k8s/apimachinery/pkg/apis/meta/v1/object_meta.k", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/k8s/apimachinery/pkg/apis/meta/v1/owner_reference.k", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/v1beta1/rds_aws_upbound_io_v1beta1_cluster_activity_stream.k", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/v1beta2/rds_aws_upbound_io_v1beta2_cluster.k", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/v1beta1/rds_aws_upbound_io_v1beta1_subnet_group.k", []byte(""), os.ModePerm)
				afero.WriteFile(fs, kclModelsFolder+"/v1beta1/redshift_aws_upbound_io_v1beta1_subnet_group.k", []byte(""), os.ModePerm)
			},

			sourceDir: kclModelsFolder,
			targetDir: kclAdoptModelsStructure,
			expectedFiles: map[string]string{
				kclAdoptModelsStructure + "/kcl.mod":                                             "",
				kclAdoptModelsStructure + "/kcl.mod.lock":                                        "",
				kclAdoptModelsStructure + "/k8s/apimachinery/pkg/apis/meta/v1/object_meta.k":     "",
				kclAdoptModelsStructure + "/k8s/apimachinery/pkg/apis/meta/v1/owner_reference.k": "",
				kclAdoptModelsStructure + "/io/upbound/aws/rds/v1beta1/clusteractivitystream.k":  "",
				kclAdoptModelsStructure + "/io/upbound/aws/rds/v1beta2/cluster.k":                "",
				kclAdoptModelsStructure + "/io/upbound/aws/rds/v1beta1/subnetgroup.k":            "",
				kclAdoptModelsStructure + "/io/upbound/aws/redshift/v1beta1/subnetgroup.k":       "",
			},
			expectedErrors: false,
		},
	}

	// Iterate over test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs() // Create an in-memory filesystem
			tt.setupFs(fs)            // Set up the initial filesystem structure

			// Run the transformation function
			err := transformStructureKcl(fs, tt.sourceDir, tt.targetDir)

			// Check if errors match expectations
			if tt.expectedErrors && err == nil {
				t.Fatalf("Expected an error but got none")
			} else if !tt.expectedErrors && err != nil {
				t.Fatalf("Did not expect an error, but got: %v", err)
			}

			// Validate the resulting file structure
			for expectedFile := range tt.expectedFiles {
				_, err := afero.ReadFile(fs, expectedFile)
				if err != nil {
					t.Fatalf("Expected file %s does not exist: %v", expectedFile, err)
				}
			}
		})
	}
}

func TestToKCLFileName(t *testing.T) {
	type testCase struct {
		input    string
		expected string
	}

	tests := map[string]testCase{
		"SimpleName": {
			input:    "Pod",
			expected: "Pod.k",
		},
		"DottedName": {
			input:    "io.k8s.api.core.v1.Pod",
			expected: "io/k8s/api/core/v1/Pod.k",
		},
		"TwoParts": {
			input:    "core.Pod",
			expected: "core/Pod.k",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := toKCLFileName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractSchemaName(t *testing.T) {
	type testCase struct {
		input    string
		expected string
	}

	tests := map[string]testCase{
		"EmptyString": {
			input:    "",
			expected: "",
		},
		"StandardReference": {
			input:    "#/components/schemas/Pod",
			expected: "Pod",
		},
		"ReferenceWithMoreParts": {
			input:    "#/components/schemas/io.k8s.api.core.v1.Pod",
			expected: "io.k8s.api.core.v1.Pod",
		},
		"SimplePath": {
			input:    "/Pod",
			expected: "Pod",
		},
		"NoSlashes": {
			input:    "Pod",
			expected: "Pod",
		},
		"TrailingSlash": {
			input:    "#/components/schemas/Pod/",
			expected: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := extractSchemaName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractSimpleName(t *testing.T) {
	type testCase struct {
		input    string
		expected string
	}

	tests := map[string]testCase{
		"EmptyString": {
			input:    "",
			expected: "",
		},
		"SimpleName": {
			input:    "Pod",
			expected: "Pod",
		},
		"KubernetesType": {
			input:    "io.k8s.api.apps.v1.Deployment",
			expected: "Deployment",
		},
		"TwoParts": {
			input:    "core.Pod",
			expected: "Pod",
		},
		"TrailingDot": {
			input:    "io.k8s.api.core.v1.",
			expected: "",
		},
		"SingleDot": {
			input:    ".",
			expected: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := extractSimpleName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestProcessSchemaReference(t *testing.T) {
	type testCase struct {
		ref               string
		currentSchemaName string
		expected          string
	}

	tests := map[string]testCase{
		"EmptyReference": {
			ref:               "",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "",
		},
		"IntOrStringReference": {
			ref:               "#/components/schemas/io.k8s.apimachinery.pkg.util.intstr.IntOrString",
			currentSchemaName: "io.k8s.api.core.v1.Service",
			expected:          "int | str",
		},
		"RawExtensionReference": {
			ref:               "#/components/schemas/io.k8s.apimachinery.pkg.runtime.RawExtension",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "any",
		},
		"SamePackageReference": {
			ref:               "#/components/schemas/io.k8s.api.core.v1.PodSpec",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "PodSpec",
		},
		"DifferentPackageReference": {
			ref:               "#/components/schemas/io.k8s.api.apps.v1.DeploymentSpec",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "appsV1.DeploymentSpec",
		},
		"MetaV1Reference": {
			ref:               "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "v1.ObjectMeta",
		},
		"SimpleReference": {
			ref:               "#/components/schemas/CustomResource",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "CustomResource",
		},
		"QuantityReference": {
			ref:               "#/components/schemas/io.k8s.apimachinery.pkg.api.resource.Quantity",
			currentSchemaName: "io.k8s.api.core.v1.ResourceRequirements",
			expected:          "str",
		},
		"QuantityReferenceWithDifferentPath": {
			ref:               "#/components/schemas/io.k8s.apimachinery.pkg.api.resource.Quantity",
			currentSchemaName: "io.k8s.api.apps.v1.Deployment",
			expected:          "str",
		},
		"TimeReference": {
			ref:               "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.Time",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "str",
		},
		"TimeReferenceWithDifferentPath": {
			ref:               "#/components/schemas/io.k8s.apimachinery.pkg.apis.meta.v1.Time",
			currentSchemaName: "io.k8s.api.apps.v1.Deployment",
			expected:          "str",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := processSchemaReference(tc.ref, tc.currentSchemaName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatTypeReference(t *testing.T) {
	type testCase struct {
		refName           string
		currentSchemaName string
		expected          string
	}

	tests := map[string]testCase{
		"EmptyRefName": {
			refName:           "",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "",
		},
		"SamePackageK8sAPI": {
			refName:           "io.k8s.api.core.v1.PodSpec",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "PodSpec",
		},
		"DifferentPackageK8sAPI": {
			refName:           "io.k8s.api.apps.v1.DeploymentSpec",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "appsV1.DeploymentSpec",
		},
		"MetaV1SamePackage": {
			refName:           "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			currentSchemaName: "io.k8s.apimachinery.pkg.apis.meta.v1.Status",
			expected:          "ObjectMeta",
		},
		"MetaV1DifferentPackage": {
			refName:           "io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "v1.ObjectMeta",
		},
		"RuntimeSamePackage": {
			refName:           "io.k8s.apimachinery.pkg.runtime.RawExtension",
			currentSchemaName: "io.k8s.apimachinery.pkg.runtime.Unknown",
			expected:          "RawExtension",
		},
		"RuntimeDifferentPackage": {
			refName:           "io.k8s.apimachinery.pkg.runtime.RawExtension",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "runtime.RawExtension",
		},
		"OtherApimachinerySamePackage": {
			refName:           "io.k8s.apimachinery.pkg.util.intstr.IntOrString",
			currentSchemaName: "io.k8s.apimachinery.pkg.util.intstr.Type",
			expected:          "IntOrString",
		},
		"NonK8sType": {
			refName:           "com.example.CustomType",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "CustomType",
		},
		"SimpleType": {
			refName:           "String",
			currentSchemaName: "io.k8s.api.core.v1.Pod",
			expected:          "String",
		},
		"CoreGroupType": {
			refName:           "io.k8s.api.core.v1.Secret",
			currentSchemaName: "io.k8s.api.apps.v1.Deployment",
			expected:          "coreV1.Secret",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := formatTypeReference(tc.refName, tc.currentSchemaName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateFromOpenAPIKCL(t *testing.T) {
	inputFS := afero.NewBasePathFs(afero.FromIOFS{FS: testdataJSONFS}, "testdata")
	schemaFS, err := kclGenerator{}.GenerateFromOpenAPI(t.Context(), inputFS, nil)
	assert.NilError(t, err)

	expectedFiles := []string{
		"models/io/k8s/api/authentication/v1/BoundObjectReference.k",
		"models/io/k8s/api/authentication/v1/TokenRequest.k",
		"models/io/k8s/api/authentication/v1/TokenRequestSpec.k",
		"models/io/k8s/api/authentication/v1/TokenRequestStatus.k",
		"models/io/k8s/api/autoscaling/v1/Scale.k",
		"models/io/k8s/api/autoscaling/v1/ScaleSpec.k",
		"models/io/k8s/api/autoscaling/v1/ScaleStatus.k",
		"models/io/k8s/api/core/v1/AWSElasticBlockStoreVolumeSource.k",
		"models/io/k8s/api/core/v1/Affinity.k",
		"models/io/k8s/api/core/v1/AppArmorProfile.k",
		"models/io/k8s/api/core/v1/AttachedVolume.k",
		"models/io/k8s/api/core/v1/AzureDiskVolumeSource.k",
		"models/io/k8s/api/core/v1/AzureFilePersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/AzureFileVolumeSource.k",
		"models/io/k8s/api/core/v1/Binding.k",
		"models/io/k8s/api/core/v1/CSIPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/CSIVolumeSource.k",
		"models/io/k8s/api/core/v1/Capabilities.k",
		"models/io/k8s/api/core/v1/CephFSPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/CephFSVolumeSource.k",
		"models/io/k8s/api/core/v1/CinderPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/CinderVolumeSource.k",
		"models/io/k8s/api/core/v1/ClientIPConfig.k",
		"models/io/k8s/api/core/v1/ClusterTrustBundleProjection.k",
		"models/io/k8s/api/core/v1/ComponentCondition.k",
		"models/io/k8s/api/core/v1/ComponentStatus.k",
		"models/io/k8s/api/core/v1/ComponentStatusList.k",
		"models/io/k8s/api/core/v1/ConfigMap.k",
		"models/io/k8s/api/core/v1/ConfigMapEnvSource.k",
		"models/io/k8s/api/core/v1/ConfigMapKeySelector.k",
		"models/io/k8s/api/core/v1/ConfigMapList.k",
		"models/io/k8s/api/core/v1/ConfigMapNodeConfigSource.k",
		"models/io/k8s/api/core/v1/ConfigMapProjection.k",
		"models/io/k8s/api/core/v1/ConfigMapVolumeSource.k",
		"models/io/k8s/api/core/v1/Container.k",
		"models/io/k8s/api/core/v1/ContainerImage.k",
		"models/io/k8s/api/core/v1/ContainerPort.k",
		"models/io/k8s/api/core/v1/ContainerResizePolicy.k",
		"models/io/k8s/api/core/v1/ContainerState.k",
		"models/io/k8s/api/core/v1/ContainerStateRunning.k",
		"models/io/k8s/api/core/v1/ContainerStateTerminated.k",
		"models/io/k8s/api/core/v1/ContainerStateWaiting.k",
		"models/io/k8s/api/core/v1/ContainerStatus.k",
		"models/io/k8s/api/core/v1/ContainerUser.k",
		"models/io/k8s/api/core/v1/DaemonEndpoint.k",
		"models/io/k8s/api/core/v1/DownwardAPIProjection.k",
		"models/io/k8s/api/core/v1/DownwardAPIVolumeFile.k",
		"models/io/k8s/api/core/v1/DownwardAPIVolumeSource.k",
		"models/io/k8s/api/core/v1/EmptyDirVolumeSource.k",
		"models/io/k8s/api/core/v1/EndpointAddress.k",
		"models/io/k8s/api/core/v1/EndpointPort.k",
		"models/io/k8s/api/core/v1/EndpointSubset.k",
		"models/io/k8s/api/core/v1/Endpoints.k",
		"models/io/k8s/api/core/v1/EndpointsList.k",
		"models/io/k8s/api/core/v1/EnvFromSource.k",
		"models/io/k8s/api/core/v1/EnvVar.k",
		"models/io/k8s/api/core/v1/EnvVarSource.k",
		"models/io/k8s/api/core/v1/EphemeralContainer.k",
		"models/io/k8s/api/core/v1/EphemeralVolumeSource.k",
		"models/io/k8s/api/core/v1/Event.k",
		"models/io/k8s/api/core/v1/EventList.k",
		"models/io/k8s/api/core/v1/EventSeries.k",
		"models/io/k8s/api/core/v1/EventSource.k",
		"models/io/k8s/api/core/v1/ExecAction.k",
		"models/io/k8s/api/core/v1/FCVolumeSource.k",
		"models/io/k8s/api/core/v1/FlexPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/FlexVolumeSource.k",
		"models/io/k8s/api/core/v1/FlockerVolumeSource.k",
		"models/io/k8s/api/core/v1/GCEPersistentDiskVolumeSource.k",
		"models/io/k8s/api/core/v1/GRPCAction.k",
		"models/io/k8s/api/core/v1/GitRepoVolumeSource.k",
		"models/io/k8s/api/core/v1/GlusterfsPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/GlusterfsVolumeSource.k",
		"models/io/k8s/api/core/v1/HTTPGetAction.k",
		"models/io/k8s/api/core/v1/HTTPHeader.k",
		"models/io/k8s/api/core/v1/HostAlias.k",
		"models/io/k8s/api/core/v1/HostIP.k",
		"models/io/k8s/api/core/v1/HostPathVolumeSource.k",
		"models/io/k8s/api/core/v1/ISCSIPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/ISCSIVolumeSource.k",
		"models/io/k8s/api/core/v1/ImageVolumeSource.k",
		"models/io/k8s/api/core/v1/KeyToPath.k",
		"models/io/k8s/api/core/v1/Lifecycle.k",
		"models/io/k8s/api/core/v1/LifecycleHandler.k",
		"models/io/k8s/api/core/v1/LimitRange.k",
		"models/io/k8s/api/core/v1/LimitRangeItem.k",
		"models/io/k8s/api/core/v1/LimitRangeList.k",
		"models/io/k8s/api/core/v1/LimitRangeSpec.k",
		"models/io/k8s/api/core/v1/LinuxContainerUser.k",
		"models/io/k8s/api/core/v1/LoadBalancerIngress.k",
		"models/io/k8s/api/core/v1/LoadBalancerStatus.k",
		"models/io/k8s/api/core/v1/LocalObjectReference.k",
		"models/io/k8s/api/core/v1/LocalVolumeSource.k",
		"models/io/k8s/api/core/v1/ModifyVolumeStatus.k",
		"models/io/k8s/api/core/v1/NFSVolumeSource.k",
		"models/io/k8s/api/core/v1/Namespace.k",
		"models/io/k8s/api/core/v1/NamespaceCondition.k",
		"models/io/k8s/api/core/v1/NamespaceList.k",
		"models/io/k8s/api/core/v1/NamespaceSpec.k",
		"models/io/k8s/api/core/v1/NamespaceStatus.k",
		"models/io/k8s/api/core/v1/Node.k",
		"models/io/k8s/api/core/v1/NodeAddress.k",
		"models/io/k8s/api/core/v1/NodeAffinity.k",
		"models/io/k8s/api/core/v1/NodeCondition.k",
		"models/io/k8s/api/core/v1/NodeConfigSource.k",
		"models/io/k8s/api/core/v1/NodeConfigStatus.k",
		"models/io/k8s/api/core/v1/NodeDaemonEndpoints.k",
		"models/io/k8s/api/core/v1/NodeFeatures.k",
		"models/io/k8s/api/core/v1/NodeList.k",
		"models/io/k8s/api/core/v1/NodeRuntimeHandler.k",
		"models/io/k8s/api/core/v1/NodeRuntimeHandlerFeatures.k",
		"models/io/k8s/api/core/v1/NodeSelector.k",
		"models/io/k8s/api/core/v1/NodeSelectorRequirement.k",
		"models/io/k8s/api/core/v1/NodeSelectorTerm.k",
		"models/io/k8s/api/core/v1/NodeSpec.k",
		"models/io/k8s/api/core/v1/NodeStatus.k",
		"models/io/k8s/api/core/v1/NodeSwapStatus.k",
		"models/io/k8s/api/core/v1/NodeSystemInfo.k",
		"models/io/k8s/api/core/v1/ObjectFieldSelector.k",
		"models/io/k8s/api/core/v1/ObjectReference.k",
		"models/io/k8s/api/core/v1/PersistentVolume.k",
		"models/io/k8s/api/core/v1/PersistentVolumeClaim.k",
		"models/io/k8s/api/core/v1/PersistentVolumeClaimCondition.k",
		"models/io/k8s/api/core/v1/PersistentVolumeClaimList.k",
		"models/io/k8s/api/core/v1/PersistentVolumeClaimSpec.k",
		"models/io/k8s/api/core/v1/PersistentVolumeClaimStatus.k",
		"models/io/k8s/api/core/v1/PersistentVolumeClaimTemplate.k",
		"models/io/k8s/api/core/v1/PersistentVolumeClaimVolumeSource.k",
		"models/io/k8s/api/core/v1/PersistentVolumeList.k",
		"models/io/k8s/api/core/v1/PersistentVolumeSpec.k",
		"models/io/k8s/api/core/v1/PersistentVolumeStatus.k",
		"models/io/k8s/api/core/v1/PhotonPersistentDiskVolumeSource.k",
		"models/io/k8s/api/core/v1/Pod.k",
		"models/io/k8s/api/core/v1/PodAffinity.k",
		"models/io/k8s/api/core/v1/PodAffinityTerm.k",
		"models/io/k8s/api/core/v1/PodAntiAffinity.k",
		"models/io/k8s/api/core/v1/PodCondition.k",
		"models/io/k8s/api/core/v1/PodDNSConfig.k",
		"models/io/k8s/api/core/v1/PodDNSConfigOption.k",
		"models/io/k8s/api/core/v1/PodIP.k",
		"models/io/k8s/api/core/v1/PodList.k",
		"models/io/k8s/api/core/v1/PodOS.k",
		"models/io/k8s/api/core/v1/PodReadinessGate.k",
		"models/io/k8s/api/core/v1/PodResourceClaim.k",
		"models/io/k8s/api/core/v1/PodResourceClaimStatus.k",
		"models/io/k8s/api/core/v1/PodSchedulingGate.k",
		"models/io/k8s/api/core/v1/PodSecurityContext.k",
		"models/io/k8s/api/core/v1/PodSpec.k",
		"models/io/k8s/api/core/v1/PodStatus.k",
		"models/io/k8s/api/core/v1/PodTemplate.k",
		"models/io/k8s/api/core/v1/PodTemplateList.k",
		"models/io/k8s/api/core/v1/PodTemplateSpec.k",
		"models/io/k8s/api/core/v1/PortStatus.k",
		"models/io/k8s/api/core/v1/PortworxVolumeSource.k",
		"models/io/k8s/api/core/v1/PreferredSchedulingTerm.k",
		"models/io/k8s/api/core/v1/Probe.k",
		"models/io/k8s/api/core/v1/ProjectedVolumeSource.k",
		"models/io/k8s/api/core/v1/QuobyteVolumeSource.k",
		"models/io/k8s/api/core/v1/RBDPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/RBDVolumeSource.k",
		"models/io/k8s/api/core/v1/ReplicationController.k",
		"models/io/k8s/api/core/v1/ReplicationControllerCondition.k",
		"models/io/k8s/api/core/v1/ReplicationControllerList.k",
		"models/io/k8s/api/core/v1/ReplicationControllerSpec.k",
		"models/io/k8s/api/core/v1/ReplicationControllerStatus.k",
		"models/io/k8s/api/core/v1/ResourceClaim.k",
		"models/io/k8s/api/core/v1/ResourceFieldSelector.k",
		"models/io/k8s/api/core/v1/ResourceHealth.k",
		"models/io/k8s/api/core/v1/ResourceQuota.k",
		"models/io/k8s/api/core/v1/ResourceQuotaList.k",
		"models/io/k8s/api/core/v1/ResourceQuotaSpec.k",
		"models/io/k8s/api/core/v1/ResourceQuotaStatus.k",
		"models/io/k8s/api/core/v1/ResourceRequirements.k",
		"models/io/k8s/api/core/v1/ResourceStatus.k",
		"models/io/k8s/api/core/v1/SELinuxOptions.k",
		"models/io/k8s/api/core/v1/ScaleIOPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/ScaleIOVolumeSource.k",
		"models/io/k8s/api/core/v1/ScopeSelector.k",
		"models/io/k8s/api/core/v1/ScopedResourceSelectorRequirement.k",
		"models/io/k8s/api/core/v1/SeccompProfile.k",
		"models/io/k8s/api/core/v1/Secret.k",
		"models/io/k8s/api/core/v1/SecretEnvSource.k",
		"models/io/k8s/api/core/v1/SecretKeySelector.k",
		"models/io/k8s/api/core/v1/SecretList.k",
		"models/io/k8s/api/core/v1/SecretProjection.k",
		"models/io/k8s/api/core/v1/SecretReference.k",
		"models/io/k8s/api/core/v1/SecretVolumeSource.k",
		"models/io/k8s/api/core/v1/SecurityContext.k",
		"models/io/k8s/api/core/v1/Service.k",
		"models/io/k8s/api/core/v1/ServiceAccount.k",
		"models/io/k8s/api/core/v1/ServiceAccountList.k",
		"models/io/k8s/api/core/v1/ServiceAccountTokenProjection.k",
		"models/io/k8s/api/core/v1/ServiceList.k",
		"models/io/k8s/api/core/v1/ServicePort.k",
		"models/io/k8s/api/core/v1/ServiceSpec.k",
		"models/io/k8s/api/core/v1/ServiceStatus.k",
		"models/io/k8s/api/core/v1/SessionAffinityConfig.k",
		"models/io/k8s/api/core/v1/SleepAction.k",
		"models/io/k8s/api/core/v1/StorageOSPersistentVolumeSource.k",
		"models/io/k8s/api/core/v1/StorageOSVolumeSource.k",
		"models/io/k8s/api/core/v1/Sysctl.k",
		"models/io/k8s/api/core/v1/TCPSocketAction.k",
		"models/io/k8s/api/core/v1/Taint.k",
		"models/io/k8s/api/core/v1/Toleration.k",
		"models/io/k8s/api/core/v1/TopologySpreadConstraint.k",
		"models/io/k8s/api/core/v1/TypedLocalObjectReference.k",
		"models/io/k8s/api/core/v1/TypedObjectReference.k",
		"models/io/k8s/api/core/v1/Volume.k",
		"models/io/k8s/api/core/v1/VolumeDevice.k",
		"models/io/k8s/api/core/v1/VolumeMount.k",
		"models/io/k8s/api/core/v1/VolumeMountStatus.k",
		"models/io/k8s/api/core/v1/VolumeNodeAffinity.k",
		"models/io/k8s/api/core/v1/VolumeProjection.k",
		"models/io/k8s/api/core/v1/VolumeResourceRequirements.k",
		"models/io/k8s/api/core/v1/VsphereVirtualDiskVolumeSource.k",
		"models/io/k8s/api/core/v1/WeightedPodAffinityTerm.k",
		"models/io/k8s/api/core/v1/WindowsSecurityContextOptions.k",
		"models/io/k8s/api/policy/v1/Eviction.k",
		"models/io/k8s/apimachinery/pkg/api/resource/Quantity.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/APIResource.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/APIResourceList.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/APIVersions.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/Condition.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/DeleteOptions.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/FieldsV1.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/LabelSelector.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/LabelSelectorRequirement.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/ListMeta.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/ManagedFieldsEntry.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/MicroTime.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/ObjectMeta.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/OwnerReference.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/Patch.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/Preconditions.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/ServerAddressByClientCIDR.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/Status.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/StatusCause.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/StatusDetails.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/Time.k",
		"models/io/k8s/apimachinery/pkg/apis/meta/v1/WatchEvent.k",
		"models/io/k8s/apimachinery/pkg/runtime/RawExtension.k",
		"models/io/k8s/apimachinery/pkg/util/intstr/IntOrString.k",
		"models/kcl.mod",
	}

	for _, path := range expectedFiles {
		exists, err := afero.Exists(schemaFS, path)
		assert.NilError(t, err)
		assert.Assert(t, exists, "expected model file %s does not exist", path)
	}
}
