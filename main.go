package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/heptio/authenticator/pkg/token"
)

var local = flag.Bool("l", false, "running locally?")
var noderolearn = flag.String("n", "", "worker node role arn")
var clusterid = flag.String("c", "", "clusterid")

type Client struct {
	Client eksiface.EKSAPI
}

type EKSData struct {
	ClusterId   string `json:"clusterid"`
	NodeRoleArn string `json:"noderolearn"`
}

func GetToken(clusterId string) string {
	var tok string
	var err error
	gen, err := token.NewGenerator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get token: %v\n", err)
		os.Exit(1)
	}
	tok, err = gen.Get(clusterId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get token: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "Token: %v\n", tok)
	return tok
}

func (c *Client) GetClusterUrl(clusterId string) string {
	cluster, err := c.Client.DescribeCluster(&eks.DescribeClusterInput{Name: &clusterId})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not find cluster: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "cluster endpoint: %v\n", *cluster.Cluster.Endpoint)
	return *cluster.Cluster.Endpoint
}

func (c *Client) GetClusterCA(clusterId string) string {
	cluster, err := c.Client.DescribeCluster(&eks.DescribeClusterInput{Name: &clusterId})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not find cluster: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "cluster ca data: %v\n", *cluster.Cluster.CertificateAuthority.Data)
	return *cluster.Cluster.CertificateAuthority.Data
}

func (c *Client) HandleRequest(ctx context.Context, data EKSData) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/kube-system/configmaps", c.GetClusterUrl(data.ClusterId))

	configmap := fmt.Sprintf(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"aws-auth","namespace":"kube-system"},"data":{"mapRoles":"- rolearn: %v\n  username: system:node:{{EC2PrivateDNSName}}\n  groups:\n    - system:bootstrappers\n    - system:nodes"}}`, data.NodeRoleArn)

	certpool := x509.NewCertPool()
	ca_data, _ := base64.StdEncoding.DecodeString(c.GetClusterCA(data.ClusterId))
	certpool.AppendCertsFromPEM(ca_data)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: certpool},
	}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("POST", url, strings.NewReader(configmap))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\b", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", GetToken(data.ClusterId)))
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error POSTing configmap: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("error creating configmap; got HTTP status code: %v\n", resp.StatusCode)
	}
	return nil
}

func main() {
	flag.Parse()

	c := &Client{}

	if *local {
		fmt.Println("Running locally...")
		session := session.Must(session.NewSessionWithOptions(session.Options{
			Profile:           "default",
			SharedConfigState: session.SharedConfigEnable,
		}))

		c.Client = eks.New(session)

		err := c.HandleRequest(context.TODO(), EKSData{ClusterId: *clusterid, NodeRoleArn: *noderolearn})

		if err != nil {
			fmt.Println(err)
		}

	} else {
		c.Client = eks.New(session.New())
		lambda.Start(c.HandleRequest)
	}
}
