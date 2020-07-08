module github.com/GoogleCloudPlatform/gke-managed-certs

go 1.14

require (
	cloud.google.com/go v0.49.0
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/golang/protobuf v1.4.0 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/onsi/ginkgo v1.11.0 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.4.1 // indirect
	github.com/prometheus/procfs v0.0.11 // indirect
	go.opencensus.io v0.22.2 // indirect
	golang.org/x/net v0.0.0-20191119073136-fc4aabc6c914 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f // indirect
	google.golang.org/api v0.14.1-0.20191117000809-006ab4afe25e
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/genproto v0.0.0-20191115221424-83cc0476cb11 // indirect
	google.golang.org/grpc v1.26.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.0.0-20200429002227-34b661834646
	k8s.io/apiextensions-apiserver v0.0.0-20200509050320-4b87f5137e22
	k8s.io/apimachinery v0.16.13-rc.0
	k8s.io/client-go v0.0.0-20200429002950-b063729e49a6
	k8s.io/code-generator v0.16.13-rc.0
	k8s.io/component-base v0.0.0-20200429004915-f49ada33f0e4
	k8s.io/klog v1.0.0
	k8s.io/legacy-cloud-providers v0.0.0-20200627034416-356b6cbdf5f8
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20200429002227-34b661834646
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20200509050320-4b87f5137e22
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.13-rc.0
	k8s.io/client-go => k8s.io/client-go v0.0.0-20200429002950-b063729e49a6
	k8s.io/code-generator => k8s.io/code-generator v0.16.13-rc.0
	k8s.io/component-base => k8s.io/component-base v0.0.0-20200429004915-f49ada33f0e4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20200627034416-356b6cbdf5f8
)
