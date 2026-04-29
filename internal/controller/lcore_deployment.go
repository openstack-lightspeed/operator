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
	"context"
	"fmt"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	apiv1beta1 "github.com/openstack-lightspeed/operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// buildLCorePodTemplateSpec builds the pod template spec for the LCore deployment.
// This function is used by CreateOrPatch to generate the desired pod spec.
func buildLCorePodTemplateSpec(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) (corev1.PodTemplateSpec, error) {
	// Build shared volumes
	volumes := []corev1.Volume{}

	// Llama Stack config volume (used by llama-stack container)
	llamaVol, llamaMount := buildLlamaStackConfigVolumeAndMount(VolumeDefaultMode)
	volumes = append(volumes, llamaVol)

	// LCore config volume (used by lightspeed-stack container)
	lcoreVol, lcoreMount := buildLCoreConfigVolumeAndMount(VolumeDefaultMode)
	volumes = append(volumes, lcoreVol)

	// Shared volumes - CA, postgres
	sharedMounts := []corev1.VolumeMount{}
	addOpenShiftCAVolumesAndMounts(&volumes, &sharedMounts, VolumeDefaultMode)
	addOpenShiftRootCAVolumesAndMounts(&volumes, &sharedMounts, VolumeDefaultMode)
	addPostgresCAVolumesAndMounts(&volumes, &sharedMounts)
	addUserCAVolumesAndMounts(&volumes, &sharedMounts, instance, VolumeDefaultMode)

	// Llama cache emptydir
	llamaCacheMounts := []corev1.VolumeMount{}
	addLlamaCacheVolumesAndMounts(&volumes, &llamaCacheMounts)

	// Shared RAG content volume
	ragMounts := []corev1.VolumeMount{}
	addRAGVolumesAndMounts(&volumes, &ragMounts)

	// Build env vars
	llamaEnvVars, err := buildLlamaStackEnvVars(h, ctx, instance)
	if err != nil {
		return corev1.PodTemplateSpec{}, fmt.Errorf("failed to build llama-stack env vars: %w", err)
	}
	lsEnvVars := buildLightspeedStackEnvVars()

	// Llama Stack container mounts: its config + shared + cache
	llamaStackMounts := []corev1.VolumeMount{llamaMount}
	llamaStackMounts = append(llamaStackMounts, sharedMounts...)
	llamaStackMounts = append(llamaStackMounts, llamaCacheMounts...)
	llamaStackMounts = append(llamaStackMounts, ragMounts...)

	llamaStackContainer := corev1.Container{
		Name:         "llama-stack",
		Image:        apiv1beta1.OpenStackLightspeedDefaultValues.LCoreImageURL,
		Command:      []string{"llama", "stack", "run", LlamaStackConfigMountPath},
		Ports:        []corev1.ContainerPort{{Name: "llama-stack", ContainerPort: LlamaStackContainerPort}},
		VolumeMounts: llamaStackMounts,
		Env:          llamaEnvVars,
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(LlamaStackContainerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Resources:       getResourcesOrDefault(nil, corev1.ResourceRequirements{}),
		ImagePullPolicy: corev1.PullIfNotPresent,
	}

	// Lightspeed Stack container mounts: its config + shared + TLS (only API container needs TLS)
	lightspeedStackMounts := []corev1.VolumeMount{lcoreMount}
	lightspeedStackMounts = append(lightspeedStackMounts, sharedMounts...)
	tlsMounts := []corev1.VolumeMount{}
	addTLSVolumesAndMounts(&volumes, &tlsMounts, VolumeDefaultMode)
	lightspeedStackMounts = append(lightspeedStackMounts, tlsMounts...)

	lightspeedStackContainer := corev1.Container{
		Name:            "lightspeed-service-api",
		Image:           apiv1beta1.OpenStackLightspeedDefaultValues.LCoreImageURL,
		Ports:           []corev1.ContainerPort{{Name: "https", ContainerPort: OpenStackLightspeedAppServerContainerPort}},
		VolumeMounts:    lightspeedStackMounts,
		Env:             lsEnvVars,
		LivenessProbe:   buildLightspeedStackLivenessProbe(),
		ReadinessProbe:  buildLightspeedStackReadinessProbe(),
		Resources:       getResourcesOrDefault(nil, corev1.ResourceRequirements{}),
		ImagePullPolicy: corev1.PullIfNotPresent,
	}

	containers := []corev1.Container{llamaStackContainer, lightspeedStackContainer}

	// Build configmap resource version annotations for change detection
	annotations, err := buildConfigMapAnnotations(h, ctx)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	annotations[RAGImageAnnotation] = instance.Spec.RAGImage
	annotations[ActiveOCPRAGVersionAnnotation] = instance.Status.ActiveOCPRAGVersion

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      generateAppServerSelectorLabels(),
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: OpenStackLightspeedAppServerServiceAccountName,
			InitContainers:     []corev1.Container{buildRAGInitContainer(instance)},
			Containers:         containers,
			Volumes:            volumes,
		},
	}, nil
}

// buildLCoreConfigVolumeAndMount returns the volume and mount for the lightspeed-stack config.
func buildLCoreConfigVolumeAndMount(volumeDefaultMode int32) (corev1.Volume, corev1.VolumeMount) {
	vol := corev1.Volume{
		Name: "lcore-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: LCoreConfigCmName,
				},
				DefaultMode: toPtr(volumeDefaultMode),
			},
		},
	}
	mount := corev1.VolumeMount{
		Name:      "lcore-config",
		MountPath: LCoreConfigMountPath,
		SubPath:   LCoreConfigFilename,
		ReadOnly:  true,
	}
	return vol, mount
}

// buildLlamaStackConfigVolumeAndMount returns the volume and mount for the llama-stack config.
func buildLlamaStackConfigVolumeAndMount(volumeDefaultMode int32) (corev1.Volume, corev1.VolumeMount) {
	vol := corev1.Volume{
		Name: "llama-stack-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: LlamaStackConfigCmName,
				},
				DefaultMode: toPtr(volumeDefaultMode),
			},
		},
	}
	mount := corev1.VolumeMount{
		Name:      "llama-stack-config",
		MountPath: LlamaStackConfigMountPath,
		SubPath:   LlamaStackConfigFilename,
		ReadOnly:  true,
	}
	return vol, mount
}

// addTLSVolumesAndMounts adds the service-ca TLS certificate volume and mount.
func addTLSVolumesAndMounts(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, volumeDefaultMode int32) {
	*volumes = append(*volumes, corev1.Volume{
		Name: "tls-certs",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  OpenStackLightspeedCertsSecretName,
				DefaultMode: toPtr(volumeDefaultMode),
			},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      "tls-certs",
		MountPath: OpenStackLightspeedAppCertsMountRoot + "/lightspeed-tls",
		ReadOnly:  true,
	})
}

// addOpenShiftCAVolumesAndMounts adds the OpenShift service-ca CA bundle volume and mount.
func addOpenShiftCAVolumesAndMounts(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, volumeDefaultMode int32) {
	*volumes = append(*volumes, corev1.Volume{
		Name: OpenShiftCAVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: OpenStackLightspeedCAConfigMap,
				},
				DefaultMode: toPtr(volumeDefaultMode),
			},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      OpenShiftCAVolumeName,
		MountPath: OpenStackLightspeedAppCertsMountRoot + "/openshift-ca",
		ReadOnly:  true,
	})
}

// addOpenShiftRootCAVolumesAndMounts adds the OpenShift cluster-wide root CA bundle.
func addOpenShiftRootCAVolumesAndMounts(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, volumeDefaultMode int32) {
	*volumes = append(*volumes, corev1.Volume{
		Name: "openshift-root-ca",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "kube-root-ca.crt",
				},
				DefaultMode: toPtr(volumeDefaultMode),
			},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      "openshift-root-ca",
		MountPath: OpenStackLightspeedAppCertsMountRoot + "/openshift-root-ca",
		ReadOnly:  true,
	})
}

// addPostgresCAVolumesAndMounts adds the Postgres CA certificate volume and mount.
func addPostgresCAVolumesAndMounts(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount) {
	*volumes = append(*volumes, getPostgresCAConfigVolume())
	*mounts = append(*mounts, getPostgresCAVolumeMount())
}

// addLlamaCacheVolumesAndMounts adds an emptydir volume for llama-stack cache.
func addLlamaCacheVolumesAndMounts(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount) {
	*volumes = append(*volumes, corev1.Volume{
		Name: "llama-cache",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      "llama-cache",
		MountPath: "/tmp/llama-stack",
	})
}

// addRAGVolumesAndMounts adds an emptyDir volume shared by init and app containers for RAG content.
func addRAGVolumesAndMounts(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount) {
	*volumes = append(*volumes, corev1.Volume{
		Name: RAGVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      RAGVolumeName,
		MountPath: RAGVolumeMountPath,
	})
}

// buildRAGInitContainer returns an init container that copies vector DB content from the RAG image.
func buildRAGInitContainer(instance *apiv1beta1.OpenStackLightspeed) corev1.Container {
	return corev1.Container{
		Name:            "rag-init",
		Image:           instance.Spec.RAGImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"sh", "-c",
			fmt.Sprintf(
				"mkdir -p %s && cp -a %s/. %s",
				RAGInitCopyDestinationPath,
				"/rag",
				RAGInitCopyDestinationPath,
			),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      RAGVolumeName,
				MountPath: RAGInitVolumeMountPath,
			},
		},
	}
}

// addUserCAVolumesAndMounts adds user-provided additional CA certificate volume and mount
// if instance.Spec.TLSCACertBundle is set.
func addUserCAVolumesAndMounts(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, instance *apiv1beta1.OpenStackLightspeed, volumeDefaultMode int32) {
	if instance.Spec.TLSCACertBundle == "" {
		return
	}
	*volumes = append(*volumes, corev1.Volume{
		Name: AdditionalCAVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: instance.Spec.TLSCACertBundle,
				},
				DefaultMode: toPtr(volumeDefaultMode),
			},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      AdditionalCAVolumeName,
		MountPath: OpenStackLightspeedAppCertsMountRoot + "/additional-ca",
		ReadOnly:  true,
	})
}

// buildAdditionalCAEnvVars returns REQUESTS_CA_BUNDLE and SSL_CERT_FILE env vars
// pointing to the additional CA cert file, if an additional CA configmap is configured.
func buildAdditionalCAEnvVars(instance *apiv1beta1.OpenStackLightspeed) []corev1.EnvVar {
	if instance.Spec.TLSCACertBundle == "" {
		return nil
	}
	certPath := OpenStackLightspeedAppCertsMountRoot + "/additional-ca/" + AdditionalCACertFile
	return []corev1.EnvVar{
		{
			Name:  "REQUESTS_CA_BUNDLE",
			Value: certPath,
		},
		{
			Name:  "SSL_CERT_FILE",
			Value: certPath,
		},
	}
}

// buildLlamaStackEnvVars builds environment variables for llama-stack,
// primarily provider API keys read from Kubernetes secrets.
func buildLlamaStackEnvVars(h *common_helper.Helper, ctx context.Context, instance *apiv1beta1.OpenStackLightspeed) ([]corev1.EnvVar, error) {
	envVars := []corev1.EnvVar{}

	{
		provider := buildProvider(instance)
		if provider.CredentialsSecret == "" {
			return envVars, nil
		}

		envVarName := providerNameToEnvVarName(provider.Name)

		if provider.Type == AzureOpenAIType {
			// Azure supports both API key and client credentials authentication.
			// Read the secret to determine which fields are present.
			secret := &corev1.Secret{}
			err := h.GetClient().Get(ctx, types.NamespacedName{
				Name:      provider.CredentialsSecret,
				Namespace: h.GetBeforeObject().GetNamespace(),
			}, secret)
			if err != nil {
				return nil, fmt.Errorf("failed to get Azure provider secret %s: %w", provider.CredentialsSecret, err)
			}

			// API key (always include - required by LiteLLM's Pydantic validation)
			if _, ok := secret.Data["apitoken"]; ok {
				envVars = append(envVars, corev1.EnvVar{
					Name: envVarName + "_API_KEY",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: provider.CredentialsSecret,
							},
							Key: "apitoken",
						},
					},
				})
			} else {
				// Provide an empty default so the env var exists
				envVars = append(envVars, corev1.EnvVar{
					Name:  envVarName + "_API_KEY",
					Value: "",
				})
			}

			// Client credentials fields for Azure AD authentication
			for _, field := range []struct {
				secretKey string
				envSuffix string
			}{
				{"client_id", "_CLIENT_ID"},
				{"tenant_id", "_TENANT_ID"},
				{"client_secret", "_CLIENT_SECRET"},
			} {
				if _, ok := secret.Data[field.secretKey]; ok {
					envVars = append(envVars, corev1.EnvVar{
						Name: envVarName + field.envSuffix,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: provider.CredentialsSecret,
								},
								Key: field.secretKey,
							},
						},
					})
				} else {
					envVars = append(envVars, corev1.EnvVar{
						Name:  envVarName + field.envSuffix,
						Value: "",
					})
				}
			}
		} else {
			// Non-Azure providers: single API_KEY from the "apitoken" key
			envVars = append(envVars, corev1.EnvVar{
				Name: envVarName + "_API_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: provider.CredentialsSecret,
						},
						Key: "apitoken",
					},
				},
			})

			// For vLLM providers, also set the URL environment variable
			// The vLLM adapter checks for VLLM_URL as a fallback if URL is not in config
			if provider.Type == "rhoai_vllm" || provider.Type == "rhelai_vllm" {
				if provider.URL != "" {
					envVars = append(envVars, corev1.EnvVar{
						Name:  "VLLM_URL",
						Value: provider.URL,
					})
				}
			}
		}
	}

	// Postgres password for ${env.POSTGRES_PASSWORD} substitution in llama-stack config
	envVars = append(envVars, buildPostgresPasswordEnvVar())

	// Logging configuration
	envVars = append(envVars, corev1.EnvVar{
		Name:  "LLAMA_STACK_LOGGING",
		Value: "all=info",
	})

	// Additional CA env vars
	envVars = append(envVars, buildAdditionalCAEnvVars(instance)...)

	return envVars, nil
}

// buildPostgresPasswordEnvVar returns the POSTGRES_PASSWORD env var sourced from the postgres secret.
func buildPostgresPasswordEnvVar() corev1.EnvVar {
	return corev1.EnvVar{
		Name: "POSTGRES_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: PostgresSecretName,
				},
				Key: OpenStackLightspeedComponentPasswordFileName,
			},
		},
	}
}

// buildLightspeedStackEnvVars builds environment variables for the lightspeed-stack container.
func buildLightspeedStackEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "LOG_LEVEL",
			Value: "INFO",
		},
		buildPostgresPasswordEnvVar(),
	}
}

// buildLightspeedStackLivenessProbe returns the liveness probe for the lightspeed-stack container.
func buildLightspeedStackLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(OpenStackLightspeedAppServerContainerPort),
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// buildLightspeedStackReadinessProbe returns the readiness probe for the lightspeed-stack container.
func buildLightspeedStackReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(OpenStackLightspeedAppServerContainerPort),
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// buildConfigMapAnnotations builds annotations with configmap resource versions
// so that changes to the configmaps trigger a deployment rollout.
func buildConfigMapAnnotations(h *common_helper.Helper, ctx context.Context) (map[string]string, error) {
	annotations := make(map[string]string)

	lcoreVersion, err := getConfigMapResourceVersion(ctx, h, LCoreConfigCmName, h.GetBeforeObject().GetNamespace())
	if err != nil {
		// ConfigMap may not exist yet during initial creation
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get LCore configmap resource version: %w", err)
		}
	} else {
		annotations[LCoreConfigMapResourceVersionAnnotation] = lcoreVersion
	}

	llamaVersion, err := getConfigMapResourceVersion(ctx, h, LlamaStackConfigCmName, h.GetBeforeObject().GetNamespace())
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get Llama Stack configmap resource version: %w", err)
		}
	} else {
		annotations[LlamaStackConfigMapResourceVersionAnnotation] = llamaVersion
	}

	return annotations, nil
}
