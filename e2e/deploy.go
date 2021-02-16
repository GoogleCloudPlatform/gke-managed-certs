/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	utilserrors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
)

const (
	clusterRoleBindingName = "managed-certificate-role-binding"
	clusterRoleName        = "managed-certificate-role"
	deploymentName         = "managed-certificate-controller"
	gcpSecretName          = "managed-certificate-gcp-sa"
	secretKey              = "key.json"
	serviceAccountName     = "managed-certificate-account"
)

// Deploys Managed Certificate CRD
func deployCRD(ctx context.Context) error {
	domainRegex := `^(([a-z0-9]+|[a-z0-9][-a-z0-9]*[a-z0-9])\.)+[a-z][-a-z0-9]*[a-z0-9]$`
	var maxDomains1 int64 = 1
	var maxDomains100 int64 = 100
	var maxDomainLength int64 = 63
	crd := apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "managedcertificates.networking.gke.io",
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: "networking.gke.io",
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: false,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextv1.JSONSchemaProps{
								"status": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"certificateStatus": {Type: "string"},
										"domainStatus": {
											Type: "array",
											Items: &apiextv1.JSONSchemaPropsOrArray{
												Schema: &apiextv1.JSONSchemaProps{
													Type:     "object",
													Required: []string{"domain", "status"},
													Properties: map[string]apiextv1.JSONSchemaProps{
														"domain": {Type: "string"},
														"status": {Type: "string"},
													},
												},
											},
										},
										"certificateName": {Type: "string"},
										"expireTime":      {Type: "string", Format: "date-time"},
									},
								},
								"spec": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"domains": {
											Type:     "array",
											MaxItems: &maxDomains1,
											Items: &apiextv1.JSONSchemaPropsOrArray{
												Schema: &apiextv1.JSONSchemaProps{
													Type:      "string",
													MaxLength: &maxDomainLength,
													Pattern:   domainRegex,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Name:    "v1beta2",
					Served:  true,
					Storage: false,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextv1.JSONSchemaProps{
								"status": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"certificateStatus": {Type: "string"},
										"domainStatus": {
											Type: "array",
											Items: &apiextv1.JSONSchemaPropsOrArray{
												Schema: &apiextv1.JSONSchemaProps{
													Type:     "object",
													Required: []string{"domain", "status"},
													Properties: map[string]apiextv1.JSONSchemaProps{
														"domain": {Type: "string"},
														"status": {Type: "string"},
													},
												},
											},
										},
										"certificateName": {Type: "string"},
										"expireTime":      {Type: "string", Format: "date-time"},
									},
								},
								"spec": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"domains": {
											Type:     "array",
											MaxItems: &maxDomains100,
											Items: &apiextv1.JSONSchemaPropsOrArray{
												Schema: &apiextv1.JSONSchemaProps{
													Type:      "string",
													MaxLength: &maxDomainLength,
													Pattern:   domainRegex,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextv1.JSONSchemaProps{
								"status": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"certificateStatus": {Type: "string"},
										"domainStatus": {
											Type: "array",
											Items: &apiextv1.JSONSchemaPropsOrArray{
												Schema: &apiextv1.JSONSchemaProps{
													Type:     "object",
													Required: []string{"domain", "status"},
													Properties: map[string]apiextv1.JSONSchemaProps{
														"domain": {Type: "string"},
														"status": {Type: "string"},
													},
												},
											},
										},
										"certificateName": {Type: "string"},
										"expireTime":      {Type: "string", Format: "date-time"},
									},
								},
								"spec": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"domains": {
											Type:     "array",
											MaxItems: &maxDomains100,
											Items: &apiextv1.JSONSchemaPropsOrArray{
												Schema: &apiextv1.JSONSchemaProps{
													Type:      "string",
													MaxLength: &maxDomainLength,
													Pattern:   domainRegex,
												},
											},
										},
									},
								},
							},
						},
					},
					AdditionalPrinterColumns: []apiextv1.CustomResourceColumnDefinition{
						{
							Name:     "Age",
							Type:     "date",
							JSONPath: ".metadata.CreationTimestamp",
						},
						{
							Name:        "Status",
							Type:        "string",
							Description: "Status of the managed certificate",
							JSONPath:    ".status.certificateStatus",
						},
					},
				},
			},
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:     "managedcertificates",
				Singular:   "managedcertificate",
				Kind:       "ManagedCertificate",
				ShortNames: []string{"mcrt"},
			},
			Scope: apiextv1.NamespaceScoped,
		},
	}
	if err := utilserrors.IgnoreNotFound(clients.CustomResource.Delete(ctx, crd.Name, metav1.DeleteOptions{})); err != nil {
		return err
	}
	if _, err := clients.CustomResource.Create(ctx, &crd, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Created custom resource definition %s", crd.Name)

	if err := utils.Retry(func() error {
		crd, err := clients.CustomResource.Get(ctx, crd.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("ManagedCertificate CRD not yet established: %v", err)
		}

		for _, c := range crd.Status.Conditions {
			if c.Type == apiextv1.Established && c.Status == apiextv1.ConditionTrue {
				return nil
			}
		}

		return errors.New("ManagedCertificate CRD not yet established")
	}); err != nil {
		return err
	}

	return nil
}

// Deploys Managed Certificate controller with all related objects
func deployController(ctx context.Context, gcpServiceAccountJson, registry, tag string) error {
	if err := deleteController(ctx); err != nil {
		return err
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: gcpSecretName},
		StringData: map[string]string{secretKey: gcpServiceAccountJson},
	}
	if _, err := clients.Secret.Create(ctx, &secret, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Created secret %s", gcpSecretName)

	serviceAccount := corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName}}
	if _, err := clients.ServiceAccount.Create(ctx, &serviceAccount, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Created service account %s", serviceAccountName)

	clusterRole := rbacv1beta1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
		Rules: []rbacv1beta1.PolicyRule{
			{
				APIGroups: []string{"networking.gke.io"},
				Resources: []string{"managedcertificates"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps", "endpoints", "events"},
				Verbs:     []string{"*"},
			},
		},
	}
	if _, err := clients.ClusterRole.Create(ctx, &clusterRole, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Created cluster role %s", clusterRoleName)

	clusterRoleBinding := rbacv1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName},
		Subjects:   []rbacv1beta1.Subject{{Namespace: "default", Name: serviceAccountName, Kind: "ServiceAccount"}},
		RoleRef: rbacv1beta1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
	}
	if _, err := clients.ClusterRoleBinding.Create(ctx, &clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Created cluster role binding %s", clusterRoleBindingName)

	appCtrl := map[string]string{"app": deploymentName}
	image := fmt.Sprintf("%s/managed-certificate-controller:%s", registry, tag)
	fileOrCreate := corev1.HostPathFileOrCreate

	sslCertsVolume := "ssl-certs"
	sslCertsVolumePath := "/etc/ssl/certs"

	usrShareCaCertsVolume := "usrsharecacerts"
	usrShareCaCertsVolumePath := "/usr/share/ca-certificates"

	logFileVolume := "logfile"
	logFileVolumePath := "/var/log/managed_certificate_controller.log"

	saKeyVolume := "sa-key-volume"
	saKeyVolumePath := "/etc/gcp"

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: appCtrl},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: appCtrl},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					RestartPolicy:      corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           image,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      sslCertsVolume,
									MountPath: sslCertsVolumePath,
									ReadOnly:  true,
								},
								{
									Name:      usrShareCaCertsVolume,
									MountPath: usrShareCaCertsVolumePath,
									ReadOnly:  true,
								},
								{
									Name:      logFileVolume,
									MountPath: logFileVolumePath,
									ReadOnly:  false,
								},
								{
									Name:      saKeyVolume,
									MountPath: saKeyVolumePath,
									ReadOnly:  true,
								},
							},
							Args: []string{
								"--logtostderr=false",
								"--alsologtostderr",
								fmt.Sprintf("--log_file=%s", logFileVolumePath),
							},
							Env: []corev1.EnvVar{
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: fmt.Sprintf("%s/%s", saKeyVolumePath, secretKey),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: sslCertsVolume,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: sslCertsVolumePath,
								},
							},
						},
						{
							Name: usrShareCaCertsVolume,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: usrShareCaCertsVolumePath,
								},
							},
						},
						{
							Name: logFileVolume,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: logFileVolumePath,
									Type: &fileOrCreate,
								},
							},
						},
						{
							Name: saKeyVolume,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: gcpSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  secretKey,
											Path: secretKey,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if _, err := clients.Deployment.Create(ctx, &deployment, metav1.CreateOptions{}); err != nil {
		return err
	}
	klog.Infof("Created deployment %s", deploymentName)

	return nil
}

// Deletes Managed Certificate controller and all related objects
func deleteController(ctx context.Context) error {
	if err := utilserrors.IgnoreNotFound(clients.Secret.Delete(ctx, gcpSecretName, metav1.DeleteOptions{})); err != nil {
		return err
	}
	klog.Infof("Deleted secret %s", gcpSecretName)

	if err := utilserrors.IgnoreNotFound(clients.ServiceAccount.Delete(ctx, serviceAccountName, metav1.DeleteOptions{})); err != nil {
		return err
	}
	klog.Infof("Deleted service account %s", serviceAccountName)

	if err := utilserrors.IgnoreNotFound(clients.ClusterRole.Delete(ctx, clusterRoleName, metav1.DeleteOptions{})); err != nil {
		return err
	}
	klog.Infof("Deleted cluster role %s", clusterRoleName)

	if err := utilserrors.IgnoreNotFound(clients.ClusterRoleBinding.Delete(ctx, clusterRoleBindingName, metav1.DeleteOptions{})); err != nil {
		return err
	}
	klog.Infof("Deleted cluster role binding %s", clusterRoleBindingName)

	if err := utilserrors.IgnoreNotFound(clients.Deployment.Delete(ctx, deploymentName, metav1.DeleteOptions{})); err != nil {
		return err
	}
	klog.Infof("Deleted deployment %s", deploymentName)

	return nil
}
