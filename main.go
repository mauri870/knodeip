package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig string
	namespace  string
	podName    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "knodeip",
		Short: "Query the external ip addresses of a pod's node",
		Run:   root}
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "",
		"Path to the kubeconfig file. If empty will use the in-cluster mode.")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "",
		"Cluster namespace. If empty will try to guess it from the environment.")
	rootCmd.PersistentFlags().StringVar(&podName, "pod", "", "Pod name to query. If empty will try to guess it from the environment.")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}

}

func root(cmd *cobra.Command, args []string) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	// If no namespace is provided, try to retrieve it assuming it's running in a pod,
	// otherwise use the default.
	if namespace == "" {
		contents, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			namespace = "default"
		} else {
			namespace = string(contents)
		}
	}

	// If podName is not supplied try to get the pod from the hostname.
	if podName == "" {
		podName, err = os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
	}

	nodeName, err := getPodNodeName(clientset, podName, namespace, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s, did you mean --pod?\n", err)
		return
	}

	addrs, err := getNodeExternalIPs(clientset, nodeName, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	for _, addr := range addrs {
		fmt.Println(addr)
	}
}

func getPodNodeName(clientset *kubernetes.Clientset, name string, namespace string, opts metav1.GetOptions) (string, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(name, opts)
	if err != nil {
		return "", err
	}

	return pod.Spec.NodeName, nil
}

func getNodeExternalIPs(clientset *kubernetes.Clientset, name string, opts metav1.GetOptions) ([]string, error) {
	node, err := clientset.CoreV1().Nodes().Get(name, opts)
	if err != nil {
		return nil, err
	}

	var addrs []string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeExternalIP {
			addrs = append(addrs, addr.Address)
		}
	}

	return addrs, nil
}
