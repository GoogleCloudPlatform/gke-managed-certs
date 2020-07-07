module github.com/GoogleCloudPlatform/gke-managed-certs

go 1.13

require (
	cloud.google.com/go v0.49.0
	github.com/GoogleCloudPlatform/k8s-cloud-provider v0.0.0-20190822182118-27a4ced34534 // indirect
	github.com/evanphx/json-patch v4.2.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/json-iterator/go v1.1.8 // indirect
	github.com/onsi/ginkgo v1.11.0 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.7.0 // indirect
	github.com/prometheus/procfs v0.0.7 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.opencensus.io v0.22.2 // indirect
	golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975 // indirect
	golang.org/x/net v0.0.0-20191119073136-fc4aabc6c914 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20191120155948-bd437916bb0e // indirect
	google.golang.org/api v0.14.1-0.20191117000809-006ab4afe25e
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/genproto v0.0.0-20191115221424-83cc0476cb11 // indirect
	google.golang.org/grpc v1.26.0 // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.0.0-20200131232428-e3a917c59b04
	k8s.io/apiextensions-apiserver v0.0.0-00010101000000-000000000000
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v0.0.0-20200410023015-75e09fce8f36
	k8s.io/component-base v0.0.0-20200131233309-6d0e514d4f25
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6 // indirect
	k8s.io/legacy-cloud-providers v0.0.0-00010101000000-000000000000
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace k8s.io/api => k8s.io/api v0.0.0-20200131232428-e3a917c59b04

replace k8s.io/apimachinery => k8s.io/apimachinery v0.15.13-beta.0

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20200410023015-75e09fce8f36

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20200410193558-7927ede350fb

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20200201000119-7b8ede55b43a

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20200318010201-8546efc3bc75

replace k8s.io/component-base => k8s.io/component-base v0.0.0-20200131233309-6d0e514d4f25
