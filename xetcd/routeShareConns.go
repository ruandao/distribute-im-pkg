package xetcd

type ShareConfig struct {
	RouteTag      string
	ShareInstance string
	ConnConfig    []string
}

type RouteShareConns struct {
	// the M should be readonly, so we don't add lock to it
	// routeTag: shareKey: *ShareConfig
	M map[string]map[string]*ShareConfig
}

func (shareCluster *RouteShareConns)GetEffectTag(routeTag string) string {
	dedicateRoute := shareCluster.M[routeTag]
	if dedicateRoute == nil {
		// 一般而言,我们会将 default 路由,视为 测试节点,以避免当配置错误时,流量对生产环境产生影响
		return "default"
	}
	return routeTag
}

func (shareCluster *RouteShareConns) Get(routeTag string, shareKey string) *ShareConfig {
	if shareCluster == nil {
		return nil
	}
	
	routeTag = shareCluster.GetEffectTag(routeTag)
	dedicateRoute := shareCluster.M[routeTag]
	if dedicateRoute == nil {
		return nil
	}

	dedicateInstance := dedicateRoute[shareKey]
	return dedicateInstance
}
