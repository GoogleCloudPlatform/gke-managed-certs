module github.com/GoogleCloudPlatform/gke-managed-certs

go 1.14

require (
	cloud.google.com/go v0.56.0
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/golang/protobuf v1.4.0 // indirect
	github.com/google/go-cmp v0.5.0
	github.com/google/uuid v1.1.1
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/procfs v0.0.11 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f // indirect
	google.golang.org/api v0.29.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.18.12
	k8s.io/apiextensions-apiserver v0.0.0-20200509050320-4b87f5137e22
	k8s.io/apimachinery v0.18.12
	k8s.io/client-go v0.18.12
	k8s.io/code-generator v0.18.12
	k8s.io/component-base v0.18.12
	k8s.io/klog v1.0.0
	k8s.io/legacy-cloud-providers v0.0.0-20200627034416-356b6cbdf5f8
)

replace (
	k8s.io/api => k8s.io/api v0.18.12
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.12
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.12
	k8s.io/client-go => k8s.io/client-go v0.18.12
	k8s.io/code-generator => k8s.io/code-generator v0.18.12
	k8s.io/component-base => k8s.io/component-base v0.18.12
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.12
)
