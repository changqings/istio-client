package istioClient

import (
	"fmt"
	"os"
	"path/filepath"

	versioned "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetIstioClient() *versioned.Clientset {
	var kubeconfig string
	var config *rest.Config

	var err error
	if os.Getenv("KUBECONFIG") != "" {
		kubeconfig = filepath.Join(os.Getenv("KUBECONFIG"))
	} else if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		kubeconfig = ""
	}

	if fileExist(kubeconfig) {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			fmt.Println("从环境变量或用户目录下获取 kubeconfig 文件不可用，请检查")
			panic(err.Error())
		}
	} else {
		// create in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			fmt.Println("Please check your local ~/.kube/config or ServiceAccount!!")
			panic(err.Error())
		}
	}
	// create istio clientset
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset

}

func fileExist(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}
