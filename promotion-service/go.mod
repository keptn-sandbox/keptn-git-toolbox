module github.com/keptn-sandbox/keptn-git-toolbox/promotion-service

go 1.13

require (
	github.com/cloudevents/sdk-go/v2 v2.3.1
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-openapi/spec v0.19.5 // indirect
	github.com/go-openapi/strfmt v0.19.5 // indirect
	github.com/go-openapi/validate v0.19.8 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/keptn/go-utils v0.8.3
	github.com/keptn/kubernetes-utils v0.8.1
	github.com/spf13/afero v1.6.0
	github.com/stretchr/testify v1.8.0
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools v2.2.0+incompatible
	helm.sh/helm/v3 v3.10.3
	k8s.io/apimachinery v0.25.2
)

replace github.com/go-git/go-git/v5 => github.com/yeahservice/go-git/v5 v5.4.2-aws-patch
