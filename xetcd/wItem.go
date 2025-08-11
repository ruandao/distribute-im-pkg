package xetcd

type ClusterChangeCB func(routeShareConns *RouteShareConns, err error)
type ClusterItem struct {
	keyPrefix  string
	changeFunc ClusterChangeCB
}

type KVChangeCB func(content string, err error)
type KVItem struct {
	key string
	changeFunc KVChangeCB
}
