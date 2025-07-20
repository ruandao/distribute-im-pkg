package traffic

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
)

var trafficRouteVal *atomic.Value

func RegisterRouteVal(routeVal *atomic.Value) {
	trafficRouteVal = routeVal
}

func RouteKeyPrefix(tag string, business string) string {
	return fmt.Sprintf("/AppState/traffic/%v/%v/", tag, business)
}

// key: /AppState/traffic/${tag}/${BusinessName}/${ip:port}
// val: appState
type TrafficRoute map[string]string

func readEndpointsFrom(route map[string]string, prefix string) TrafficEndPoints {
	var endPoints []string
	for key := range route {
		if strings.HasPrefix(key, prefix) {
			ipport, found := strings.CutPrefix(key, prefix)
			if found {
				endPoints = append(endPoints, ipport)
			}
		}
	}
	return endPoints
}

func ReadRouteEndPoints(tag any, business string) TrafficEndPoints {
	if tag == "" {
		tag = "default"
	}
	sTag := tag.(string)
	logx.Infof("trafficRouteVal: %v\n", trafficRouteVal)
	route := trafficRouteVal.Load().(map[string]string)
	logx.Infof("route: %v\n", route)
	targetKeyPrefix := RouteKeyPrefix(sTag, business)
	endPoints := readEndpointsFrom(route, targetKeyPrefix)

	// the business don't have tag Node
	// then we will traffic to default Node
	if len(endPoints) == 0 && tag != "default" {
		targetKeyPrefix := RouteKeyPrefix("default", business)
		endPoints = readEndpointsFrom(route, targetKeyPrefix)
	}
	return endPoints
}
