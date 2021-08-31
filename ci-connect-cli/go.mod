module github.com/keptn-sandbox/keptn-git-toolbox/ci-connect-cli

go 1.16

require (
	github.com/go-git/go-git/v5 v5.4.2
	github.com/golang/mock v1.4.1
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
	gotest.tools v2.2.0+incompatible
	helm.sh/helm/v3 v3.5.4
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v20.10.6+incompatible
	github.com/go-git/go-git/v5 => github.com/yeahservice/go-git/v5 v5.4.2-aws-patch
)
