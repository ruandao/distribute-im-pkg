package xetcd

import "sync"

type XMap struct {
	sync.Map
}

func (xMap *XMap) Keys() []string {
	var keys []string
	xMap.Range(func(key, value any) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}
