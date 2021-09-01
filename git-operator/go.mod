module github.com/keptn-sandbox/keptn-git-toolbox/git-operator

go 1.16

require (
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v0.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)

replace github.com/go-git/go-git/v5 => github.com/yeahservice/go-git/v5 v5.4.2-aws-patch
replace github.com/keptn-sandbox/keptn-git-toolbox/git-operator/model => ./model
