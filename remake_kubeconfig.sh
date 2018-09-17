#!/usr/bin/env bash

#!/bin/bash
set -e
set -o pipefail


# Add user to k8s using service account, no RBAC (must create RBAC after this script)
if [[ -z "$1" ]] || [[ -z "$2" ]] || [[ -z "$3" ]]; then
 echo "usage: $0 <service_account_name> <namespace> <cluster_name>"
 exit 1
fi


SERVICE_ACCOUNT_NAME=$1
NAMESPACE="$2"
CLUSTER_NAME="$3"
KUBECFG_FILE_NAME="/tmp/kube/k8s-${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-${CLUSTER_NAME}-conf"
TARGET_FOLDER="/tmp/kube"


create_target_folder() {
    echo -n "Creating target directory to hold files in ${TARGET_FOLDER}..."
    mkdir -p "${TARGET_FOLDER}"
    echo -n "Deleting any old possibly existing file ${KUBECFG_FILE_NAME}"
    rm -f "${KUBECFG_FILE_NAME}"
    printf "done"
}

get_user_token_from_secret() {
# Alternative temporary credentials
#    USER_TOKEN="$(AWS_PROFILE=admin; aws-iam-authenticator \
#    token -i ${CLUSTER_NAME} --token-only)"
    echo "\\nGetting service account user token from parameterstore /core/${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-${CLUSTER_NAME}-bearer-token"

    USER_TOKEN="$(aws --region 'us-east-1' \
    ssm get-parameters \
    --with-decryption \
    --names '/core/'"${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-${CLUSTER_NAME}"'-bearer-token' \
    --query "Parameters[*].{Name:Name,Value:Value}" | jq .[].Value -r )"

    printf "done"
}


set_kube_config_values() {

    echo "Cluster name: ${CLUSTER_NAME}"

    CA_DATA=$(aws eks describe-cluster --region us-east-1 --name $CLUSTER_NAME --query cluster.certificateAuthority.data --output text)
    ENDPOINT=$(aws eks describe-cluster --region us-east-1 --name $CLUSTER_NAME --query cluster.endpoint --output text)
    echo "Endpoint: ${ENDPOINT}"


    # Set up the config
    echo -e "\\nPreparing k8s-${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-conf"
    echo -n "Setting a cluster entry in kubeconfig..."
    kubectl config set-cluster "${CLUSTER_NAME}" \
    --kubeconfig="${KUBECFG_FILE_NAME}" \
    --server="${ENDPOINT}"

    # https://github.com/kubernetes/kubectl/issues/501
    kubectl config --kubeconfig="${KUBECFG_FILE_NAME}" \
    set "clusters.${CLUSTER_NAME}.certificate-authority-data" \
    "${CA_DATA}"


    echo -n "Setting token credentials entry in kubeconfig..."
    kubectl config set-credentials \
    "${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-${CLUSTER_NAME}" \
    --kubeconfig="${KUBECFG_FILE_NAME}" \
    --token="${USER_TOKEN}"


    echo -n "Setting a context entry in kubeconfig..."
    kubectl config set-context \
    "${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-${CLUSTER_NAME}" \
    --kubeconfig="${KUBECFG_FILE_NAME}" \
    --cluster="${CLUSTER_NAME}" \
    --user="${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-${CLUSTER_NAME}" \
    --namespace="${NAMESPACE}"


    echo -n "Setting the current-context in the kubeconfig file..."
    kubectl config use-context "${SERVICE_ACCOUNT_NAME}-${NAMESPACE}-${CLUSTER_NAME}" \
    --kubeconfig="${KUBECFG_FILE_NAME}"
}


create_target_folder
get_user_token_from_secret
set_kube_config_values


echo -e "\\nAll done! Test with:"
echo "KUBECONFIG=${KUBECFG_FILE_NAME} kubectl get pods"
echo "you should not have any permissions by default - you have just created the authentication part"
echo "You will need to create RBAC permissions"
KUBECONFIG=${KUBECFG_FILE_NAME} kubectl get pods
