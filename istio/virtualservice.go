package istio

import (
	"errors"
	"fmt"
	tools "istio-client/utils"
	"log"
	"strings"

	"golang.org/x/net/context"
	networkingV1beta1 "istio.io/api/networking/v1beta1"
	"istio.io/client-go/pkg/apis/networking/v1beta1"
	"istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Vs struct {
	Name                  string
	Namespace             string
	Version               string
	AppName               string
	CanaryWeight          int32
	CanaryWeightSwitch    bool
	StablHttpMatch        []networkingV1beta1.HTTPMatchRequest
	StableHttpDestination []networkingV1beta1.HTTPRouteDestination
	HttpMatch             []*networkingV1beta1.HTTPMatchRequest
	VirtualService        *v1beta1.VirtualService
}

var ctx = context.Background()

func (vs *Vs) ListVs(cs *versioned.Clientset) []*v1beta1.VirtualServiceList {

	var vsListSlice []*v1beta1.VirtualServiceList
	vsList, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("List vs err: %v", err)
		return nil
	}
	vsListSlice = append(vsListSlice, vsList)

	return vsListSlice
}

func (vs *Vs) GetVs(cs *versioned.Clientset) *v1beta1.VirtualService {

	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Get(ctx, vs.Name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Get vs err: %v", err)
		return nil
	}

	return v

}
func (vs *Vs) GetVsRule(cs *versioned.Clientset, rname string) (int, *networkingV1beta1.HTTPRoute) {
	v := vs.GetVs(cs)

	for index, j := range v.Spec.Http {
		if j.Name == rname {
			return index, j
		}
	}

	return -1, nil

}

func (vs *Vs) AddVsRule(cs *versioned.Clientset) (*v1beta1.VirtualService, error) {

	// get ori vs
	vs.VirtualService = vs.GetVs(cs)
	// some operation done there
	index, stableRoute := vs.getVsStableRoute(cs)
	if stableRoute == nil || index == -1 {
		return nil, fmt.Errorf("can't get stable route, please check")
	}

	defaultHttpMatch := &networkingV1beta1.HTTPMatchRequest{
		Name: fmt.Sprintf("%s-%s", vs.AppName, tools.ReplaceVersion(vs.Version)),
		Headers: map[string]*networkingV1beta1.StringMatch{
			"x-weike-forward": {
				MatchType: &networkingV1beta1.StringMatch_Exact{
					Exact: vs.Version,
				},
			},
		},
	}

	defaultHttpRoute := []*networkingV1beta1.HTTPRouteDestination{
		{
			Destination: &networkingV1beta1.Destination{
				Host:   fmt.Sprintf("%s-canary.%s.svc.cluster.local", vs.AppName, vs.Namespace),
				Subset: tools.ReplaceVersion(vs.Version),
			},
			Weight: 100,
		},
		{
			Destination: &networkingV1beta1.Destination{
				Host:   fmt.Sprintf("%s.%s.svc.cluster.local", vs.AppName, vs.Namespace),
				Subset: "stable",
			},
			Weight: 0,
		},
	}

	stableUri := checkVsMatchUri(stableRoute)

	if stableUri != nil {
		defaultHttpMatch.Uri = stableUri
	}

	canaryHr := &networkingV1beta1.HTTPRoute{}

	canaryHr.Match[0] = defaultHttpMatch
	canaryHr.Route = defaultHttpRoute

	vs.VirtualService.Spec.Http = append(vs.VirtualService.Spec.Http[:index], append([]*networkingV1beta1.HTTPRoute{canaryHr}, vs.VirtualService.Spec.Http[index:]...)...)

	// update vs
	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vs.VirtualService, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("Get vs err: %v", err)
		return nil, nil
	}
	return v, nil
}

func (vs *Vs) DelVsRule(cs *versioned.Clientset) (*v1beta1.VirtualService, error) {

	vOri := vs.VirtualService
	if vOri == nil {
		log.Panicf("vs.VertualService == nil, please run getVs() first")
	}

	for i, j := range vOri.Spec.Http {
		if j.Name == vs.AppName+"-"+"stable" {
			return nil, errors.New("can not delete stable vs rule")
		}
		if j.Name == vs.AppName+"-"+tools.ReplaceVersion(vs.Version) {
			vOri.Spec.Http = append(vOri.Spec.Http[:i], vOri.Spec.Http[i+1:]...)
			break
		}
	}

	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vOri, metav1.UpdateOptions{})
	if err != nil {
		return nil, err

	}
	return v, nil

}

func (vs *Vs) UpdateVsRule(cs *versioned.Clientset, rName string) *v1beta1.VirtualService {

	vOri := vs.VirtualService.DeepCopy()
	if vOri == nil {
		log.Panicf("vs.VertualService == nil, please run getVs() first")
	}

	// upate vs.HttpRoute.Name rule
	index, _ := vs.GetVsRule(cs, rName)

	vTargetHttp := vOri.Spec.Http[index]
	vTargetHttp.Match = vs.HttpMatch

	rIndex, rWeight := vs.getVsRouteWeight(cs, vTargetHttp.Route)

	if len(vTargetHttp.Route) != 1 && rWeight != 0 || len(vTargetHttp.Route) != 1 && rWeight == 100 {

	} else if vs.CanaryWeight != 100 && vs.CanaryWeight != rWeight && len(vTargetHttp.Match) == 2 {
		if rIndex == 0 {
			vTargetHttp.Route[0].Weight = vs.CanaryWeight
			vTargetHttp.Route[1].Weight = 100 - vs.CanaryWeight
		} else {
			vTargetHttp.Route[1].Weight = vs.CanaryWeight
			vTargetHttp.Route[0].Weight = 100 - vs.CanaryWeight
		}
	}

	// update vs
	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vOri, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("Update vs err: %v", err)
		return nil
	}
	return v
}

func (vs *Vs) CheckVsSubsetExist(vOri *v1beta1.VirtualService) error {
	// check all canary version not used
	sub := strings.ReplaceAll(vs.Version, ".", "-")
	for _, m := range vOri.Spec.Http {
		for _, n := range m.Route {
			if n.Destination.Subset == sub {
				log.Printf("Not all subset = %s delete, please check", sub)
				return errors.New("check all subset delete error")
			}
		}
	}
	return nil

}

func (vs *Vs) getVsRouteWeight(cs *versioned.Clientset, hDest []*networkingV1beta1.HTTPRouteDestination) (int32, int32) {
	t := tools.ReplaceVersion(vs.Version)

	for i, j := range hDest {
		if j.Destination.Subset == t {
			return int32(i), j.Weight
		}
	}
	return -1, -1
}

func (vs *Vs) addVsStableDestination(cs *versioned.Clientset, hDest []*networkingV1beta1.HTTPRouteDestination) []*networkingV1beta1.HTTPRouteDestination {
	hDestStable := &networkingV1beta1.HTTPRouteDestination{
		Destination: &networkingV1beta1.Destination{
			Host:   fmt.Sprintf("%s.%s.svc.cluster.local", vs.AppName, vs.Namespace),
			Subset: "stable",
		},
		Weight: 0,
	}

	hDest[0].Weight = 100

	hDest = append(hDest, hDestStable)
	return hDest
}

func (vs *Vs) getVsStableRoute(cs *versioned.Clientset) (int, *networkingV1beta1.HTTPRoute) {
	rName := fmt.Sprintf("%s,%s.svc.cluster.local", vs.AppName, vs.Namespace)
	index, stableRoute := vs.GetVsRule(cs, rName)

	if stableRoute != nil {
		return index, stableRoute.DeepCopy()
	}

	return -1, nil
}

func checkVsMatchUri(hRoute *networkingV1beta1.HTTPRoute) *networkingV1beta1.StringMatch {

	hc := hRoute.DeepCopy()
	if hc == nil {
		return nil
	}

	if len(hc.Match) == 0 {
		return nil
	}

	for _, v := range hc.Match {
		if v.Uri != nil {
			return v.Uri
		}
	}
	return nil
}
