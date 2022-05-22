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
	StableHttpMatch       []networkingV1beta1.HTTPMatchRequest
	StableHttpDestination []networkingV1beta1.HTTPRouteDestination
	HttpMatch             []*networkingV1beta1.HTTPMatchRequest
	VirtualService        *v1beta1.VirtualService
}

var ctx = context.Background()

// get the vs.Name pointer
func (vs *Vs) GetVs(cs *versioned.Clientset) *v1beta1.VirtualService {

	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Get(ctx, vs.Name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Get vs err: %v", err)
		return nil
	}

	return v

}

// get vs httpRoute
func (vs *Vs) GetVsHttpRoute(cs *versioned.Clientset, rname string) (int, *networkingV1beta1.HTTPRoute) {
	v := vs.GetVs(cs)

	for index, j := range v.Spec.Http {
		if j.Name == rname {
			return index, j
		}
	}

	return -1, nil

}

// first cd add the default canary vs httpRoute
func (vs *Vs) AddCanaryVsHttpRoute(cs *versioned.Clientset) (*v1beta1.VirtualService, error) {

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

	stableUri := getVsMatchUri(stableRoute)

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

// del canary vs httpRoute, and check all canary version delete, and update it into vs.Name
func (vs *Vs) DelCanaryVsHttpRoute(cs *versioned.Clientset) (*v1beta1.VirtualService, error) {

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

	if err := vs.checkVsSubsetExist(vOri); err != nil {
		return nil, err
	}

	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vOri, metav1.UpdateOptions{})
	if err != nil {
		return nil, err

	}
	return v, nil

}

// update canary vs httpRoute when it had been put or post from frontend
func (vs *Vs) UpdateCanaryVsHttpRoute(cs *versioned.Clientset, rName string) *v1beta1.VirtualService {

	vOri := vs.VirtualService.DeepCopy()
	if vOri == nil {
		log.Panicf("vs.VertualService == nil, please run getVs() first")
	}

	// upate vs.HttpRoute.Name rule
	index, _ := vs.GetVsHttpRoute(cs, rName)

	vTargetHttp := vOri.Spec.Http[index]

	// if CanaryWeight has changed, then update weight, and remove matches not in stable match
	switch vs.CanaryWeightSwitch {
	case true:
		rIndex, rWeight := vs.getVsRouteWeight(cs, vTargetHttp.Route)
		if vs.CanaryWeight != rWeight && len(vTargetHttp.Match) == 2 {
			// mod canary httpRoute []destionationRoute weight
			vTargetHttp.Route[rIndex].Weight = vs.CanaryWeight
			vTargetHttp.Route[1-rIndex].Weight = 100 - vs.CanaryWeight

			// use stable httpRoute.Match replace of canary httpRoute.Match
			vTargetHttp.Match = vs.getVsStableMatch(cs)
		} else {
			log.Printf("The canary weight not changed, do nothing.")
		}
	default:
		// get stable route and check stable route has uri Match, if ok, add it to canary vsRoute match when canary vsRoute match no uri(if it has headers)
		_, vsStableRoute := vs.getVsStableRoute(cs)
		if getVsMatchUri(vsStableRoute) != nil {
			for _, j := range vs.HttpMatch {
				if j.Uri == nil {
					j.Uri = getVsMatchUri(vsStableRoute)
				}
			}
		}
		vTargetHttp.Match = vs.HttpMatch
	}

	// update vs
	v, err := cs.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vOri, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("Update vs err: %v", err)
		return nil
	}
	return v
}

// when delete canary rule, check all httpRouteDestinatio have no canary version
func (vs *Vs) checkVsSubsetExist(vOri *v1beta1.VirtualService) error {
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

// return RouteDestion index and weight
func (vs *Vs) getVsRouteWeight(cs *versioned.Clientset, hDest []*networkingV1beta1.HTTPRouteDestination) (int32, int32) {
	t := tools.ReplaceVersion(vs.Version)

	for i, j := range hDest {
		if j.Destination.Subset == t {
			return int32(i), j.Weight
		}
	}
	return -1, -1
}

// get stable httpRoute.Match, whether it have something or not, if sRoute.Match == nil, return nil
func (vs *Vs) getVsStableMatch(cs *versioned.Clientset) []*networkingV1beta1.HTTPMatchRequest {
	_, sRoute := vs.getVsStableRoute(cs)
	return sRoute.Match
}

// get stable route pointer and it's index in the vs
func (vs *Vs) getVsStableRoute(cs *versioned.Clientset) (int, *networkingV1beta1.HTTPRoute) {
	rName := fmt.Sprintf("%s-stable", vs.AppName)
	index, stableRoute := vs.GetVsHttpRoute(cs, rName)

	if stableRoute != nil {
		return index, stableRoute.DeepCopy()
	}

	return -1, nil
}

// if httpRoute.Match have uri, return this uri match
func getVsMatchUri(hRoute *networkingV1beta1.HTTPRoute) *networkingV1beta1.StringMatch {

	hc := hRoute.DeepCopy()
	if hc == nil || len(hc.Match) == 0 {
		return nil
	}

	for _, v := range hc.Match {
		if v.Uri != nil {
			return v.Uri
		}
	}
	return nil
}
