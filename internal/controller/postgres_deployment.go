/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"path"
	"strconv"

	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// buildPostgresPodTemplateSpec builds the pod template spec for the Postgres deployment.
// If configMapChanged is true, it adds a force-reload timestamp to trigger pod restart.
func buildPostgresPodTemplateSpec() corev1.PodTemplateSpec {
	// Build volumes and volume mounts
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	restrictedMode := VolumeRestrictedMode
	defaultMode := VolumeDefaultMode

	// TLS certs volume (auto-provisioned by service-ca via the Service annotation)
	volumes = append(volumes, corev1.Volume{
		Name: "secret-" + PostgresCertsSecretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  PostgresCertsSecretName,
				DefaultMode: &restrictedMode,
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "secret-" + PostgresCertsSecretName,
		MountPath: OpenStackLightspeedAppCertsMountRoot,
		ReadOnly:  true,
	})

	// Bootstrap script volume
	volumes = append(volumes, corev1.Volume{
		Name: "secret-" + PostgresBootstrapSecretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  PostgresBootstrapSecretName,
				DefaultMode: &restrictedMode,
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "secret-" + PostgresBootstrapSecretName,
		MountPath: PostgresBootstrapVolumeMountPath,
		SubPath:   PostgresExtensionScript,
		ReadOnly:  true,
	})

	// Postgres config volume
	volumes = append(volumes, corev1.Volume{
		Name: PostgresConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: PostgresConfigMapName},
				DefaultMode:          &defaultMode,
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      PostgresConfigMapName,
		MountPath: PostgresConfigVolumeMountPath,
		SubPath:   PostgresConfigKey,
	})

	// TODO: CRITICAL - Replace EmptyDir with a PVC. With EmptyDir all conversation
	// history is lost if the pod is rescheduled or the OCP control plane goes down.
	volumes = append(volumes, corev1.Volume{
		Name: PostgresDataVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      PostgresDataVolume,
		MountPath: PostgresDataVolumeMountPath,
	})

	// Postgres CA volume
	volumes = append(volumes, getPostgresCAConfigVolume())
	volumeMounts = append(volumeMounts, getPostgresCAVolumeMountWithPath(path.Join(OpenStackLightspeedAppCertsMountRoot, PostgresCAVolume)))

	// Var run volume (writable runtime directory)
	volumes = append(volumes, corev1.Volume{
		Name: PostgresVarRunVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      PostgresVarRunVolumeName,
		MountPath: PostgresVarRunVolumeMountPath,
	})

	// Tmp volume (writable temp directory)
	volumes = append(volumes, corev1.Volume{
		Name: TmpVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      TmpVolumeName,
		MountPath: TmpVolumeMountPath,
	})

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      generatePostgresSelectorLabels(),
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            PostgresDeploymentName,
					Image:           apiv1beta1.OpenStackLightspeedDefaultValues.PostgresImageURL,
					ImagePullPolicy: corev1.PullAlways,
					Ports: []corev1.ContainerPort{
						{
							Name:          "server",
							ContainerPort: PostgresServicePort,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &[]bool{false}[0],
						ReadOnlyRootFilesystem:   &[]bool{true}[0],
					},
					VolumeMounts: volumeMounts,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("30m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "POSTGRESQL_USER",
							Value: PostgresDefaultUser,
						},
						{
							Name:  "POSTGRESQL_DATABASE",
							Value: PostgresDefaultDbName,
						},
						{
							Name:  "POSTGRESQL_SHARED_BUFFERS",
							Value: PostgresSharedBuffers,
						},
						{
							Name:  "POSTGRESQL_MAX_CONNECTIONS",
							Value: strconv.Itoa(PostgresMaxConnections),
						},
						{
							Name: "POSTGRESQL_ADMIN_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: PostgresSecretName},
									Key:                  OpenStackLightspeedComponentPasswordFileName,
								},
							},
						},
						{
							Name: "POSTGRESQL_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: PostgresSecretName},
									Key:                  OpenStackLightspeedComponentPasswordFileName,
								},
							},
						},
					},
				},
			},
			Volumes: volumes,
		},
	}
}
