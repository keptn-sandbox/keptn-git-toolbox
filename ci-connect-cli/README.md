# Keptn CI Connect CLI

Needed environment variables:
```
export GIT_REPO="https://github.com/<repo>"
export GIT_USER="<my github user>"
export GIT_TOKEN="<my github token>"
export WORKSPACE=<the workspace>
```

A configuration file is expected in the directory `$WORKSPACE/.keptn/ci_config.yaml` and might look as follows:

```
 services:
 - name: podtatoserver
   chart_base: helm/podtatoserver
 git_config:
   user_email: keptn@keptn.sh
   user_name: jenkins
```

The deployment can be triggered using:

```
./ci-connect-cli trigger-deployment --service podtatoservice
```

Example configurations for testing can be found in the `.keptn` and `helm` directories