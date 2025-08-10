package xetcd

type ClusterChangeCB func(*RouteShareConns)
type ClusterItem struct {
	keyPrefix  string
	changeFunc ClusterChangeCB
}

type KVChangeCB func(*string)
type KVItem struct {
	key string
	changeFunc KVChangeCB
}
