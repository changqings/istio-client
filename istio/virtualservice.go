package istio

import (
	"fmt"
	"istio-client/istioClient"
)

cs := istioClient.GetIstioClient()

type Vs struct {
	Name string
	Namespace string
	Gateway string
	Host string
	RouterName string
	RouteDrStableHost string
	RouteDrCanaryHost string
	WeightStable int64
	WeightCanary int64
	
}

func GetVsAll(){
	fmt.Printf("%v", cs)
}


