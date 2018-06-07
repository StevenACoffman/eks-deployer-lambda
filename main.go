package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/heptio/authenticator/pkg/token"
)

var local = flag.Bool("l", true, "running locally?")

func GetToken(clusterId string) string {
	var tok string
	var err error
	gen, err := token.NewGenerator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get token: %v\n", err)
		os.Exit(1)
	}
	tok, err = gen.GetWithRole(clusterId, GetLambdaRoleArn())
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(tok)
	return tok
}

func GetLambdaRoleArn() string {
	stssvc := sts.New(session.New())
	identity, err := stssvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get identity %v\n", err)
		os.Exit(1)
	}

	return *identity.Arn

}

func GetClusterUrl(clusterId string) string {
	ekssvc := eks.New(session.New())
	cluster, err := ekssvc.DescribeCluster(&eks.DescribeClusterInput{Name: &clusterId})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not find cluster: %v\n", err)
		os.Exit(1)
	}
	return *cluster.Cluster.Endpoint
}

type ClusterId struct {
	ClusterId string `json:"clusterid"`
}

func HandleRequest(ctx context.Context, clusterId ClusterId) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/kube-system/configmaps", GetClusterUrl(clusterId.ClusterId))

	configmap := fmt.Sprintf("{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"aws-auth\",\"namespace\":\"kube-system\"},\"data\":{\"mapRoles\":\"- rolearn: %v\n  username: system:node:{{EC2PrivateDNSName}}\n  groups:\n    - system:bootstrappers\n    - system:nodes\"}}")

	client := &http.Client{}
	req, _ := http.NewRequest("POST", url, strings.NewReader(configmap))
	req.Header.Add("Bearer:", GetToken(clusterId.ClusterId))
	resp, err := client.Do(req)
	if err != nil {
		fmt.Sprintf("error POSTing configmap: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("error creating configmap %s; got HTTP %v status code", configmap, resp.StatusCode)
	}

	return nil
}

func main() {
	if *local {
		err := HandleRequest(context.TODO(), ClusterId{ClusterId: "test"})
		if err != nil {
			fmt.Println(err)
		}
	} else {
		lambda.Start(HandleRequest)
	}
}
