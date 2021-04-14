module github.com/GoogleCloudPlatform/gke-managed-certs

go 1.14

require (
	cloud.google.com/go v0.81.0
	github.com/GoogleCloudPlatform/k8s-cloud-provider v1.14.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.5.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.20.0 // indirect
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/net v0.0.0-20210414194228-064579744ee0 // indirect
	golang.org/x/oauth2 v0.0.0-20210413134643-5e61552d6c78
	golang.org/x/sys v0.0.0-20210415045647-66c3f260301c // indirect
	golang.org/x/term v0.0.0-20210406210042-72f3dc4e9b72 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	google.golang.org/api v0.44.0
	google.golang.org/genproto v0.0.0-20210414175830-92282443c685 // indirect
	google.golang.org/grpc v1.37.0 // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver v0.0.0-20200509050320-4b87f5137e22
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v1.5.2
	k8s.io/cloud-provider v0.21.0 // indirect
	k8s.io/code-generator v0.20.0
	k8s.io/component-base v0.21.0
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20210323165736-1a6458611d18 // indirect
	k8s.io/legacy-cloud-providers v0.21.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.1 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.20.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.0
	k8s.io/client-go => k8s.io/client-go v0.20.0
	k8s.io/code-generator => k8s.io/code-generator v0.20.0
	k8s.io/component-base => k8s.io/component-base v0.20.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.20.0
)
