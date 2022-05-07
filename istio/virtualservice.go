package istio

import (
	"encoding/json"
	"log"

	"golang.org/x/net/context"
	"istio.io/client-go/pkg/apis/networking/v1beta1"
	"istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Vs struct {
	Name      string
	Namespace string
	Gateways  []string
	Hosts     []string
	Http      []HttpRoute
}

type HttpRoute struct {
	Name  string
	Match []HttpMatchRequest
	Route []HttpRouteDestination
}

type HttpMatchRequest struct {
	Name   string
	Uri    string
	Header map[string]string
}

type HttpRouteDestination struct {
	Host   string
	SubSet string
	Port   uint32
	Weight int32
}

var ctx = context.Background()

func (vs *Vs) ListVs(cs *versioned.Clientset) []*v1beta1.VirtualServiceList {

	var vsListSlice []*v1beta1.VirtualServiceList
	vsList, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		log.Printf("List vs err: %v", err)
		return nil
	}
	vsListSlice = append(vsListSlice, vsList)

	return vsListSlice
}

func (vs *Vs) GetVs(cs *versioned.Clientset) *v1beta1.VirtualService {

	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Get(ctx, vs.Name, v1.GetOptions{})
	if err != nil {
		log.Printf("Get vs err: %v", err)
		return nil
	}

	return v

}

func (vs *Vs) AddVsRule(cs *versioned.Clientset, vsOri *v1beta1.VirtualService) *v1beta1.VirtualService {
	// some operation done there

	// update vs
	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vsOri, v1.UpdateOptions{})
	if err != nil {
		log.Printf("Get vs err: %v", err)
		return nil
	}
	return v
}

func (vs *Vs) DelVsRule(cs *versioned.Clientset, vsOri *v1beta1.VirtualService) *v1beta1.VirtualService {

	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vsOri, v1.UpdateOptions{})
	if err != nil {
		log.Printf("Del vs rule err: %v", err)

	}
	return v

}

func (vs *Vs) PatchVs(cs *versioned.Clientset, vsOri *v1beta1.VirtualService) *v1beta1.VirtualService {
	// some operation done there

	// convert vs to byte
	vsOriBytes, err := json.Marshal(vsOri)
	if err != nil {
		log.Panicf("vsOri json marshal err: %v", err)
	}

	// patch vs
	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Patch(ctx, vs.Name, types.MergePatchType, vsOriBytes, v1.PatchOptions{})
	if err != nil {
		log.Printf("Get vs err: %v", err)
		return nil
	}
	return v
}
