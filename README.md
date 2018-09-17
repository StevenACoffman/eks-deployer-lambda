## EKS Deployer Lambda

This lamdba will make a deployment into an EKS environment. It is intended for use in or after an AWS CodeBuild or AWS CodePipeline job.

You will need to edit the serverless.yml file and config directory files to reflect your AWS account, subnets, etc.

Running the [kubernetes_add_service_account_kubeconfig.sh](./kubernetes_add_service_account_kubeconfig.sh) script created rbac permissions (with cluster admin permissions!), and exports everything necessary for authentication to the cluster.


### Why do I care?

The most interesting piece here is how authentication to the EKS kubernetes cluster is handled.
EKS supports either IAM authN or a service account bearer token.

This repo is an example of using a kubernetes service account and exporting the bearer token out of the cluster

Alternatively, you could try using [aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator) as a library you can use AWS IAM credentials to authenticate to an EKS Kubernetes cluster and receive a bearer token.

The second option (as [in this example](https://github.com/chankh/eksutil)) was originally how this repo worked, but the first proved simpler and more robust.

If you want to go the other route, `kubectl edit -n kube-system configmap/aws-auth` and add the iam role of the lambda to the `mapRoles` section with a rolearn, username, and groups.
However, using the  [aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator) as a library is also providing a _temporary_ bearer token, and [occasionally failed](https://github.com/kubernetes-sigs/aws-iam-authenticator/issues/133).
The kubernetes rbac still needed to be manually applied. It just seemed complicated and slow. Maybe if you have a federated kubernetes this might be more worth pursuing.

Instead, 
### kubernetes_add_service_account_kubeconfig.sh

Three things are required for authentication:
+ Cluster Certificate Authority Data : `aws eks describe-cluster --region us-east-1 --name $CLUSTERNAME --query cluster.certificateAuthority.data`
+ Host : `aws eks describe-cluster --region us-east-1 --name $CLUSTERNAME --query cluster.endpoint`
+ Bearer Token : `kubectl get secret "${SECRET_NAME}" --namespace "${NAMESPACE}" -o json | jq -r '.data["token"]' | base64 -D`

Running this script will:
+ add serviceaccount
+ apply cluster admin rolebinding
+ export a kubeconfig wired up to authenticate using that service account's bearer token for testing
+ save the bearer token to AWS Parameter store for use by the lambda

```bash
./kubernetes-add-service-account-kubeconfig.sh

```

### How could anyone else have made this project from scratch?

```
cd $GOPATH/src/github.com/ithaka/continuous-deployment/go/
export APPNAME="pullrequest-clone-pipeline"
mkdir $APPNAME
cd $APPNAME
echo "10.8.0" > .nvmrc
nvm install
npm init -f
npm install serverless --save-dev
npm install serverless-pseudo-parameters --save-dev
npx serverless create -t aws-go-dep --name $APPNAME
```

### Getting Started

Use the AWS cli to add the parameters to SSM parameter store:

```
aws ssm put-parameter --name '/core/ithaka-cypress-github-otp' --value "$GITHUB_OAUTH_TOKEN" --type SecureString --region us-east-1
aws ssm put-parameter --name '/core/github-pr-webhook-secret' --value "$(ruby -rsecurerandom -e 'puts SecureRandom.hex(20)')" --type SecureString --region us-east-1
```

**You will need Serverless framework version 1.22.0 or above.**

### Retrieve CloudFormation

```
aws cloudformation get-template --stack-name eks-deployer-lambda-test
```

./kubernetes_add_service_account_kubeconfig.sh eks-deployer-lambda default
