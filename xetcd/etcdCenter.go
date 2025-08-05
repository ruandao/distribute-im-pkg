package xetcd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ruandao/distribute-im-pkg/lib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	etcdLib "go.etcd.io/etcd/client/v3"
)

type Content struct {
	Endpoints []string
	cli       *etcdLib.Client
	c         sync.Map
	closeCh   chan struct{}
}

func (content *Content) connect(ctx context.Context) error {
	cli, err := etcdLib.New(etcdLib.Config{
		Endpoints:   content.Endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return lib.NewXError(err, "connect etcd failed")
	}
	content.cli = cli
	content.closeCh = make(chan struct{})

	gResp, err := cli.Get(ctx, "/", etcdLib.WithPrefix())
	if err != nil {
		return lib.NewXError(err, "Get Config Data Fail")
	}

	for _, kv := range gResp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)

		content.c.Store(key, value)
		logx.Info(fmt.Sprintf("update key: %v value: %v\n", key, value))
	}

	go func() {
		ch := cli.Watch(ctx, "/", etcdLib.WithPrefix())
		for {
			select {
			case kvChange, ok := <-ch:
				if !ok {
					break
				}
				for _, event := range kvChange.Events {
					switch event.Type {
					case etcdLib.EventTypePut:
						key := string(event.Kv.Key)
						value := string(event.Kv.Value)
						content.c.Store(key, value)
						logx.Info(fmt.Sprintf("update key: %v value: %v\n", key, value))
					case etcdLib.EventTypeDelete:
						key := string(event.Kv.Key)
						content.c.Delete(key)
						logx.Info(fmt.Sprintf("delete key: %v \n", key))
					}
				}
			case <-content.closeCh:
				return
			}
		}
	}()
	return nil
}
func (content *Content) Close() {
	close(content.closeCh)
	content.cli.Close()
}

func (content *Content) Range(fn func(key string, value string) bool) {
	content.c.Range(func(key, value any) bool {
		ret := fn(key.(string), value.(string))
		return ret
	})
}

type ShareDBConfig struct {
	RouteTag      string
	ShareInstance string
	ConnConfig    []lib.DBConfig
}


// RouteTag: ShareInstance: []DBConfig
func (content *Content) GetShareDBConfig(keyPrefix string) map[string]map[string]*ShareDBConfig {
	routeMap := make(map[string]map[string]*ShareDBConfig)
	found := false
	content.Range(func(key string, value string) bool {
		if strings.HasPrefix(key, keyPrefix) {
			// key
			// /appState/auth/mysql/default/db0/127.0.0.1:3306/state
			keyWithoutPrefix, _ := strings.CutPrefix(key, keyPrefix)
			// keyWithoutPrefix
			// /default/db0/127.0.0.1:3306/state
			pieces := strings.Split(keyWithoutPrefix, "/")
			tag, instance := pieces[1], pieces[2]
			
			var dbConfig *lib.DBConfig = &lib.DBConfig{}
			
			err := lib.ReadFromJSON([]byte(value), dbConfig)
			// 为了避免应用程序级联崩溃, 我们将在这里忽略错误
			if err != nil {
				fmt.Printf("parse key: %v\n", key)
				fmt.Printf("for value: %v\n", value)
				fmt.Printf("because err: %v\n", err)
				return true
			}
			// fmt.Printf("tag: %v instance: %v dbConfig: %v err: %v\n", tag, instance, dbConfig, err)
			found = true
			instanceMap := routeMap[tag]
			if instanceMap == nil {
				instanceMap = make(map[string]*ShareDBConfig)
			}
			defer func() {
				// fmt.Printf("routeMap: %v\n", routeMap)
				routeMap[tag] = instanceMap
			}()

			instanceOnSpecficShare := instanceMap[instance]
			if instanceOnSpecficShare == nil {
				instanceOnSpecficShare = &ShareDBConfig{
					RouteTag:      tag,
					ShareInstance: instance,
					ConnConfig:    nil,
				}
			}
			defer func() {
				// fmt.Printf("instanceMap: %v\n", instanceMap)
				// fmt.Printf("instance: %v\n", instance)
				// fmt.Printf("instanceOnSpecficShare: %v\n", instanceOnSpecficShare)
				// fmt.Printf("pieces: %v tag: %v instance: %v\n", pieces, tag, instance)
				instanceMap[instance] = instanceOnSpecficShare
			}()

			
			instanceOnSpecficShare.ConnConfig = append(instanceOnSpecficShare.ConnConfig, *dbConfig)
		}
		return true
	})
	// fmt.Printf("getShareDBConfig keyPrefix: %v value: %v\n", keyPrefix, routeMap)
	if !found {
		return nil
	}
	return routeMap
}
// if routeTag no match, it will downgrade to "default"
// if shareInstance no match, it will return nil
func (content *Content) GetShareDBInstancesConfig(keyPrefix string, routeTag string, shareInstance string) *ShareDBConfig {
	shareDBConfig := content.GetShareDBConfig(keyPrefix)
	if shareDBConfig == nil {
		return nil
	}
	dedicateRoute := shareDBConfig[routeTag]
	if dedicateRoute == nil {
		dedicateRoute = shareDBConfig["default"]
	}

	if dedicateRoute == nil {
		return  nil
	}

	dedicateInstance := dedicateRoute[shareInstance]
	return  dedicateInstance
}

func New(ctx context.Context, Endpoints []string) (*Content, error) {
	content := &Content{Endpoints: Endpoints}
	err := content.connect(ctx)
	if err != nil {
		return nil, err
	}
	return content, nil
}
