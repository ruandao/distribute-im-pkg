package xetcd

import (
	"fmt"

	"github.com/ruandao/distribute-im-pkg/lib/xerr"
)

type RouteTag string
type ShareName string
type ShareConfig struct {
	RouteTag      RouteTag
	ShareInstance ShareName
	ConnConfig    []string
}

type RouteShareConns struct {
	// the M should be readonly, so we don't add lock to it
	// shareKey: routeTag: *ShareConfig
	M map[ShareName]map[RouteTag]*ShareConfig
}
func NewRouteShareConns(m map[ShareName]map[RouteTag]*ShareConfig) RouteShareConns {
	if m == nil {
		panic("m shouldn't be an nil")
	}
	return  RouteShareConns{M: m}
}

func (shareCluster *RouteShareConns)GetEffectTag(shareKey ShareName, routeTag RouteTag) RouteTag {
	dedicateRoute := shareCluster.M[shareKey][routeTag]
	if dedicateRoute == nil {
		// 一般而言,我们会将 default 路由,视为 测试节点,以避免当配置错误时,流量对生产环境产生影响
		return "default"
	}
	return routeTag
}

func (shareCluster *RouteShareConns) Get(shareKey ShareName, routeTag RouteTag) (shareConfig *ShareConfig, err error) {
	if shareCluster == nil {
		return nil, xerr.NewXError(fmt.Errorf("shareCluster for shareKey: %v routeTag: %v shouldn't be nil", shareKey, routeTag))
	}
	
	routeTag = shareCluster.GetEffectTag(shareKey, routeTag)
	dedicateInstance := shareCluster.M[shareKey][routeTag]
	if dedicateInstance == nil {
		return nil, xerr.NewXError(fmt.Errorf("couldn't found dedicateInstance for share: %v route: %v", shareKey, routeTag))
	}

	return dedicateInstance, nil
}
