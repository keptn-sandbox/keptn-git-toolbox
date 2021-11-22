module github.com/keptn-sandbox/keptn-git-toolbox/git-operator

go 1.16

require (
	github.com/fluxcd/pkg/apis/meta v0.10.0
	github.com/fluxcd/pkg/untar v0.1.0
	github.com/fluxcd/source-controller/api v0.18.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v0.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.5
)

replace github.com/go-git/go-git/v5 => github.com/yeahservice/go-git/v5 v5.4.2-aws-patch

replace github.com/keptn-sandbox/keptn-git-toolbox/git-operator/model => ./model
