# Managed Certificates

Managed Certificates simplify user flow in managing HTTPS traffic.
Instead of manually acquiring an SSL certificate from a Certificate
Authority, configuring it on the load balancer and renewing it on time,
now it is only necessary to create a Managed Certificate
[Custom Resource object](https://kubernetes.io/docs/concepts/api-extension/custom-resources/)
and provide the domains for which you want to obtain a certificate.
The certificate will be auto-renewed when necessary.

For that to work you need to run your cluster on a platform with
[Google Cloud Load Balancer](https://github.com/kubernetes/ingress-gce),
that is a cluster in GKE or your own cluster in GCP.

In GKE all the components are already installed.
Follow the [how-to](https://cloud.google.com/kubernetes-engine/docs/how-to/managed-certs)
for more information. For a GCP setup follow the instructions below.

This feature status is GA.

# Installation

Managed Certificates consist of two parts:
* managed-certificate-controller which uses GCP Compute API to manage
  certificates securing your traffic,
* Managed Certificate CRD which is needed to tell the controller what
  domains you want to secure.

## Limitations

* Managed Certificates support multi-SAN non-wildcard certificates.
* Managed Certificates are compatible only with [GKE Ingress](https://github.com/kubernetes/ingress-gce).
* A single ManagedCertificate supports up to 100 domain names.
* A single Ingress supports up to 15 certificates, and all types of certificates
  count towards the limit.
* A GCP project supports up to
  [ssl_certificates](https://cloud.google.com/load-balancing/docs/quotas#ssl_certificates)
  quota of certificates.

## Prerequisites

1. You need to use a Kubernetes cluster with GKE-Ingress v1.5.1+.
    * Managed Certificates have been tested against Kubernetes v1.19.0.
    * Kubernetes v1.15+ most likely will work as well.
    * Kubernetes v1.13-v1.15 most likely will work if you enable the
      [CustomResourceWebhookConversion feature](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definition-versioning/),
      otherwise Managed Certificate CRD validation will not work properly.
1. You need to grant permissions to the controller so that it is allowed to use
   the GCP Compute API.
    * When creating the cluster, add scope *compute-rw* to the node where you will
      run the pod with managed-certificate-controller.
    * Alternatively:
        * [Create](https://cloud.google.com/kubernetes-engine/docs/how-to/access-scopes#service_account)
          a dedicated service account with minimal roles.
            ```console
            export NODE_SA_NAME=mcrt-controller-sa
            gcloud iam service-accounts create $NODE_SA_NAME --display-name "managed-certificate-controller service account"
            export NODE_SA_EMAIL=`gcloud iam service-accounts list --format='value(email)' \
            --filter='displayName:managed-certificate-controller'`

            export PROJECT=`gcloud config get-value project`
            gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL \
            --role roles/monitoring.metricWriter
            gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL \
            --role roles/monitoring.viewer
            gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL \
            --role roles/logging.logWriter
            ```
        * [Grant](https://cloud.google.com/kubernetes-engine/docs/how-to/access-scopes#additional_roles)
          additional role *roles/compute.loadBalancerAdmin* to your service
          account.
            ```console
            gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL \
            --role roles/compute.loadBalancerAdmin
            ```
        * [Export](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys)
          a service account key to a JSON file.
            ```console
            gcloud iam service-accounts keys create ./key.json --iam-account $NODE_SA_EMAIL
            ```
        * Create a Kubernetes Secret that holds the service account key stored
          in key.json.
            ```console
            kubectl create secret generic sa-key --from-file=./key.json
            ```
        * Mount the sa-key secret to managed-certificate-controller pod. In file deploy/managed-certificate-controller.yaml add:
            * Above section *volumeMounts*
                ```
                env:
                  - name: GOOGLE_APPLICATION_CREDENTIALS
                    value: "/etc/gcp/key.json"
                ```
            * In section *volumeMounts*
                ```
                - name: sa-key-volume
                  mountPath: /etc/gcp
                  readOnly: true
                ```
            * In section *volumes*
                ```
                - name: sa-key-volume
                  secret:
                    secretName: sa-key
                    items:
                    - key: key.json
                      path: key.json
                ```
1. Configure your domain example.com so that it points at the load balancer
created for your cluster by Ingress. If you add a CAA record to restrict the CAs that are allowed
to provision certificates for your domain, note that Managed Certificates currently support:
    * [Google Trust Services](http://pki.goog),
    * Let's Encrypt.
In the future additional CAs may be available and a CAA record may make it impossible
for you to take advantage of them.

## Steps

To install Managed Certificates in your own cluster in GCP, you need to:

1. Deploy the Managed Certificate CRD
    ```console
    $ kubectl create -f deploy/managedcertificates-crd.yaml
    ```
1. Deploy the managed-certificate-controller
   You may want to build your own managed-certificate-controller image and
   reference it in the deploy/managed-certificate-controller.yaml file. The default
   image is periodically built by a CI system and may not be stable. Alternatively
   you may use `gcr.io/gke-release/managed-certificate-controller:v1.0.5`
   which is deployed in GKE, however this README likely will not be kept up to date with
   future GKE updates, and so this image may become stale.
    ```console
    $ kubectl create -f deploy/managed-certificate-controller.yaml
    ```

# Usage

1. Create a Managed Certificate custom object, specifying up to 100 non-wildcard domains
not longer than 63 characters each, for which you want to obtain a certificate:
    ```yaml
    apiVersion: networking.gke.io/v1
    kind: ManagedCertificate
    metadata:
      name: example-certificate
    spec:
      domains:
      - example1.com
      - example2.com
    ```
2. Configure Ingress to use this custom object to terminate SSL connections:
    ```console
    $ kubectl annotate ingress [your-ingress-name] networking.gke.io/managed-certificates=example-certificate
    ```
If you need, you can specify multiple managed certificates here,
separating their names with commas.

# Clean up

You can do the below steps in any order to turn SSL off:

* Remove annotation from Ingress
    ```console
    $ kubectl annotate ingress [your-ingress-name] networking.gke.io/managed-certificates-
    ```
  (note the minus sign at the end of annotation name)
* Tear down the controller
    ```console
    $ kubectl delete -f deploy/managed-certificate-controller.yaml
    ```
* Tear down the Managed Certificate CRD
    ```console
    $ kubectl delete -f deploy/managedcertificates-crd.yaml
    ```

# Troubleshooting

1. Check Kubernetes events attached to ManagedCertificate and Ingress resources
   for information on temporary failures.

2. Use the same ManagedCertificate resource at every endpoint to which your domain resolves to.

   A real life example is when your example.com domain points at two IP
   addresses, one for IPv4 and one for IPv6. You [deploy two Ingress objects](https://github.com/kubernetes/ingress-gce/issues/87)
   to handle IPv4 and IPv6 traffic separately. If you create
   two separate ManagedCertificate resources and attach each of them to one of
   the Ingresses, one of the ManagedCertificate resources may not be
   provisioned. The reason is that the Certificate Authority is free to verify
   challenges on any of the IP addresses the domain resolves to.

3. Managed Certificates communicate with GKE Ingress using annotation
   kubernetes.io/pre-shared-cert. Problems may arise for instance if you:

   * forcibly keep clearing this annotation,
   * store a snapshot of Ingress, tear it down and restore Ingress from the
     snapshot. In the meantime an SslCertificate resource listed in the
     pre-shared-cert annotation may not be available any more. Ingress has
     all-or-nothing semantics and will not work if a certificate it references
     is missing.

# API changes

Managed Certificates support the following versions of Managed Certificate CRD,
the API: v1, v1beta2 and v1beta1.

You should use the v1 version. v1beta2 and v1beta1 are deprecated and will be
removed in the future.

v1beta2 supports more than one domain name per certificate (multi-SAN).

v1beta1 supports one domain name per certificate.
