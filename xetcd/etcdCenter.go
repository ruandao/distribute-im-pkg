package xetcd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ruandao/distribute-im-pkg/lib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	bConfLib "github.com/ruandao/distribute-im-pkg/config/basicConfig"

	etcdLib "go.etcd.io/etcd/client/v3"
)

type XContent struct {
	Endpoints []string
	cli       *etcdLib.Client
	cancelF   func()

	c       sync.Map
	closeCh chan struct{}
	// string: []*ClusterItem
	clusterWatchMap XMap
	// string: []KVItem
	keyWatchMap XMap
}

func (content *XContent) connect(ctx context.Context) error {
	cli, err := etcdLib.New(etcdLib.Config{
		Endpoints:   content.Endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return xerr.NewXError(err, "connect etcd failed")
	}
	content.cli = cli
	content.closeCh = make(chan struct{})

	gResp, err := cli.Get(ctx, "/", etcdLib.WithPrefix())
	if err != nil {
		return xerr.NewXError(err, "Get Config Data Fail")
	}

	for _, kv := range gResp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)
		content.c.Store(key, value)
		logx.Info(fmt.Sprintf("Load key: %v value: %v", key, value))
	}

	go func() {
		ctx, cancelF := context.WithCancel(context.Background())
		content.cancelF = cancelF
		ch := cli.Watch(ctx, "/", etcdLib.WithPrefix())
		logx.DebugX("etcdWatch open")("")
		defer func() {
			logx.DebugX("etcdWatch closed")("")
		}()
		for {
			select {
			case kvChange, ok := <-ch:
				if !ok {
					break
				}

				notifyList := []string{}
				for _, event := range kvChange.Events {
					switch event.Type {
					case etcdLib.EventTypePut:
						key := string(event.Kv.Key)
						value := string(event.Kv.Value)
						pre, _ := content.c.Load(key)
						if pre != value {
							content.c.Store(key, value)
							notifyList = append(notifyList, key)
							logx.Info(fmt.Sprintf("update key: %v pre: %v new: %v", key, pre, value))
						}
					case etcdLib.EventTypeDelete:
						key := string(event.Kv.Key)
						content.c.Delete(key)
						afterDelVal, ok := content.c.Load(key)
						notifyList = append(notifyList, key)
						logx.Info(fmt.Sprintf("delete key: %v afterDelVal: %v ok: %v", key, afterDelVal, ok))
					}
				}

				notifySet := lib.NewXSet(notifyList)

				clusterSet := lib.NewXSet(content.clusterWatchMap.Keys())
				clusterNotifyList := clusterSet.Intersect(notifySet, func(selfElem, argElem string) bool {
					return strings.HasPrefix(argElem, selfElem)
				})

				kvSet := lib.NewXSet(content.keyWatchMap.Keys())
				kvNotifyList := kvSet.Intersect(notifySet, func(selfElem, argElem string) bool {
					return selfElem == argElem
				})

				for _, key := range clusterNotifyList {
					_updateList, _ := content.clusterWatchMap.Load(key)
					if _updateList == nil {
						continue
					}
					cluster, err := content.GetDepServicesCluster(key)
					for _, updateItem := range _updateList.([]*ClusterItem) {
						updateItem.changeFunc(cluster, err)
					}
				}

				for _, key := range kvNotifyList {
					_updateList, _ := content.clusterWatchMap.Load(key)
					if _updateList == nil {
						continue
					}
					retV, err := content.Get(key)
					for _, updateItem := range _updateList.([]*KVItem) {
						updateItem.changeFunc(retV, err)
					}
				}

			case <-content.closeCh:
				return
			}
		}
	}()
	return nil
}
func (content *XContent) Close() {
	if content.cancelF != nil {
		content.cancelF()
		close(content.closeCh)
		content.cli.Close()
	}
}

func (content *XContent) Range(fn func(key string, value string) bool) {
	content.c.Range(func(key, value any) bool {
		ret := fn(key.(string), value.(string))
		return ret
	})
}
func (content *XContent) Get(key string) (retV string, err error) {
	ret, found := content.c.Load(key)
	retV = ""
	if found {
		retV = ret.(string)
		return retV, nil
	}
	return retV, xerr.NewXError(fmt.Errorf("couldn't found content on %v", key))
}
func (content *XContent) PutLease(ctx context.Context, key string, val string, seconds int64) error {
	leaseResp, err := content.cli.Grant(ctx, seconds)
	if err != nil {
		xerr := xerr.NewXError(err, "Get Grant Fail")
		return xerr
	}
	leaseID := leaseResp.ID

	_, err = content.cli.Put(ctx, key, val, etcdLib.WithLease(leaseID))
	if err != nil {
		xerr := xerr.NewXError(err, fmt.Sprintf("set key %s fail: %v", key, err))
		return xerr
	}

	return nil
}

func (content *XContent) GetSelfConfigX(bConfig bConfLib.BConfig) (string, bool) {
	return content.GetSelfConfig(bConfig.BusinessName, bConfig.Role, bConfig.Version, bConfig.ShareName)
}
func (content *XContent) GetSelfConfig(bizName string, role string, version string, share_name string) (string, bool) {
	// /appConfig/业务/职能/版本号/数据分片/config
	configPath := fmt.Sprintf("/appConfig/%v/%v/%v/%v/config", bizName, role, version, share_name)
	var ret string
	found := false
	content.Range(func(key, value string) bool {
		if key == configPath {
			ret = value
			found = true

			return false
		}
		return true
	})
	return ret, found
}

// RouteTag: ShareInstance: []DBConfig
func (content *XContent) GetDepServicesCluster(keyPrefix string) (*RouteShareConns, error) {
	routeMap := make(map[ShareName]map[RouteTag]*ShareConfig)
	found := false
	content.Range(func(key string, value string) bool {
		if strings.HasPrefix(key, keyPrefix) {
			// key
			// /appState/auth/mysql/db0/default/127.0.0.1:3306/state
			// `keyPrefix`/`shareName`/`routeTag`/`instanceIPPort`/state
			keyWithoutPrefix, _ := strings.CutPrefix(key, keyPrefix)
			// keyWithoutPrefix
			// /db0/default/127.0.0.1:3306/state
			pieces := strings.Split(keyWithoutPrefix, "/")
			shareKey, tag, ipport := ShareName(pieces[1]), RouteTag(pieces[2]), pieces[3]

			// fmt.Printf("tag: %v instance: %v dbConfig: %v err: %v\n", tag, instance, dbConfig, err)
			found = true
			shareMap := routeMap[shareKey]
			if shareMap == nil {
				shareMap = make(map[RouteTag]*ShareConfig)
			}
			defer func() {
				// fmt.Printf("routeMap: %v\n", routeMap)
				routeMap[shareKey] = shareMap
			}()

			instanceOnSpecficShare := shareMap[tag]
			if instanceOnSpecficShare == nil {
				instanceOnSpecficShare = &ShareConfig{
					RouteTag:      tag,
					ShareInstance: shareKey,
					ConnConfig:    XMap{},
				}
			}
			defer func() {
				// fmt.Printf("instanceMap: %v\n", instanceMap)
				// fmt.Printf("instance: %v\n", instance)
				// fmt.Printf("instanceOnSpecficShare: %v\n", instanceOnSpecficShare)
				// fmt.Printf("pieces: %v tag: %v instance: %v\n", pieces, tag, instance)
				shareMap[tag] = instanceOnSpecficShare
			}()

			instanceOnSpecficShare.ConnConfig.Store(ipport, value)
		}
		return true
	})
	// fmt.Printf("getShareDBConfig keyPrefix: %v value: %v\n", keyPrefix, routeMap)
	if !found {
		return nil, xerr.NewXError(fmt.Errorf("couldn't found route config on %v", keyPrefix))
	}
	cluster := NewRouteShareConns(routeMap)
	return &cluster, nil
}

// if routeTag no match, it will downgrade to "default"
// if shareInstance no match, it will return nil
func (content *XContent) GetDepServicesShareDBInstancesConfig(keyPrefix string, shareKey ShareName, routeTag RouteTag) (*ShareConfig, error) {
	shareCluster, err := content.GetDepServicesCluster(keyPrefix)
	if err != nil {
		return nil, err
	}
	shareConf, err := shareCluster.Get(shareKey, routeTag)
	// logx.DebugX("GetDepServicesShareDBInstancesConfig")(shareConf, err)
	return shareConf, err
}

func (content *XContent) KeyWatch(key string, changeFunc KVChangeCB) func() {
	cnt := 10
	for cnt > 0 {
		cnt--
		wItem := &KVItem{key: key, changeFunc: changeFunc}
		removeF := func() {
			content.KeyWatchRemove(wItem)
		}
		pre, loaded := content.keyWatchMap.LoadOrStore(key, []*KVItem{wItem})
		if !loaded {
			logx.Infof("KeyWatch LoadOrStore: store %v pre: %v\n", key, pre)
			return removeF
		}
		logx.Infof("KeyWatch %v pre: %v\n", key, pre)
		var preList []*KVItem = nil
		if pre != nil {
			preList = pre.([]*KVItem)
		}
		new := append(preList, wItem)
		swapped := content.keyWatchMap.CompareAndSwap(key, pre, new)
		logx.Infof("KeyWatch %v pre: %v new: %v swapped: %v\n", key, pre, new, swapped)
		if swapped {
			return removeF
		}
	}
	return nil
}

func (content *XContent) KeyWatchRemove(wItem *KVItem) {
	logx.Infof("KeyWatchRemove %v\n", wItem)
	cnt := 10
	for cnt > 0 {
		cnt--
		pre, _ := content.keyWatchMap.Load(wItem.key)
		logx.Infof("KeyWatchRemove key: %v pre: %v wItem: %v\n", wItem.key, pre, wItem)
		if pre == nil {
			return
		}
		newList := []*KVItem{}
		for _, item := range pre.([]*KVItem) {
			if item == wItem {
				continue
			}
			newList = append(newList, item)
		}
		updateSuccess := false
		if len(newList) == 0 {
			updateSuccess = content.keyWatchMap.CompareAndDelete(wItem.key, pre)
		} else {
			updateSuccess = content.keyWatchMap.CompareAndSwap(wItem.key, pre, newList)
		}
		if updateSuccess {
			return
		}
	}
}

func (content *XContent) ClusterWatch(keyPrefix string, changeFunc ClusterChangeCB) func() {
	cnt := 10
	for cnt > 0 {
		cnt--
		wItem := &ClusterItem{keyPrefix: keyPrefix, changeFunc: changeFunc}
		removeF := func() {
			content.ClusterWatchRemove(wItem)
		}
		pre, loaded := content.clusterWatchMap.LoadOrStore(keyPrefix, []*ClusterItem{wItem})
		if !loaded {
			return removeF
		}
		logx.Infof("ClusterWatch %v pre: %v\n", keyPrefix, pre)
		var preList []*ClusterItem = nil
		if pre != nil {
			preList = pre.([]*ClusterItem)
		}
		new := append(preList, wItem)
		swapped := content.clusterWatchMap.CompareAndSwap(keyPrefix, pre, new)
		logx.Infof("ClusterWatch %v pre: %v new: %v swapped: %v map: %v\n", keyPrefix, pre, new, swapped, &content.clusterWatchMap)
		if swapped {
			return removeF
		}
	}
	return nil
}

func (content *XContent) ClusterWatchRemove(wItem *ClusterItem) {
	for {
		pre, _ := content.clusterWatchMap.Load(wItem.keyPrefix)
		if pre == nil {
			return
		}
		newList := []*ClusterItem{}
		for _, item := range pre.([]*ClusterItem) {
			if item == wItem {
				continue
			}
			newList = append(newList, item)
		}
		updateSuccess := false
		if len(newList) == 0 {
			updateSuccess = content.clusterWatchMap.CompareAndDelete(wItem.keyPrefix, pre)
		} else {
			updateSuccess = content.clusterWatchMap.CompareAndSwap(wItem.keyPrefix, pre, newList)
		}
		if updateSuccess {
			return
		}
	}
}

func New(bConfig *bConfLib.BConfig) (*XContent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer func() {
		cancel()
	}()
	content := &XContent{Endpoints: strings.Split(bConfig.EtcdAddrs, ",")}
	logx.Infof("etcd connect start\n")
	logx.Infof("etcd connect endpoints: %v\n", strings.Split(bConfig.EtcdAddrs, ","))
	err := content.connect(ctx)
	if err != nil {
		return nil, err
	}
	logx.Infof("etcd connect done \n")
	return content, nil
}

var _xContent *XContent

func Register(xContent *XContent) {
	_xContent = xContent
}

func Get() *XContent {
	if _xContent == nil {
		panic(xerr.NewError("never register XContent"))
	}
	return _xContent
}
