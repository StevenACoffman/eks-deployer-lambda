package main

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
)

var local = flag.Bool("l", false, "running locally?")

type Client struct {
	Client eksiface.EKSAPI
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

func (c *Client) HandleRequest(ctx context.Context) error {

	certpool := x509.NewCertPool()
	clusterId := os.Getenv("EKS_CLUSTER_NAME")
	clusterCA := c.GetClusterCA(clusterId)
	clusterCAData, _ := base64.StdEncoding.DecodeString(clusterCA)
	clusterUrl := c.GetClusterUrl(clusterId)

	certpool.AppendCertsFromPEM(clusterCAData)

	bearerToken := os.Getenv("EKS_BEARER_TOKEN")


	kubeConfig := rest.Config{
		Host:        clusterUrl,
		BearerToken: string(bearerToken),
		TLSClientConfig: rest.TLSClientConfig{
			CAData: clusterCAData,
		},
	}
	clientset, err := kubernetes.NewForConfig(&kubeConfig)
	if err != nil {
		panic(err)
	}

	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "demo",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "web",
							Image: "nginx:1.12",
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	// Create Deployment
	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

	return nil
}

func main() {
	flag.Parse()

	c := &Client{}

	if *local {
		fmt.Println("Running locally...")
		sess := session.Must(session.NewSessionWithOptions(session.Options{
			Profile:           "default",
			SharedConfigState: session.SharedConfigEnable,
		}))

		c.Client = eks.New(sess)

		err := c.HandleRequest(context.TODO())

		if err != nil {
			fmt.Println(err)
		}

	} else {
		c.Client = eks.New(session.Must(session.NewSession()))
		lambda.Start(c.HandleRequest)
	}
}

func int32Ptr(i int32) *int32 { return &i }