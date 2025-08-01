package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	confreadych "github.com/ruandao/distribute-im-pkg/config/confReadyCh"
	lib "github.com/ruandao/distribute-im-pkg/lib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	etcdLib "go.etcd.io/etcd/client/v3"
)

type IAppState interface {
	// seem we don't need any limit now
}

func NewAppState(appState IAppState) atomic.Value {
	var appStateVal atomic.Value
	appStateVal.Store(appState)
	waitFirstSyncCh := make(chan struct{})
	waitFirstSyncChOnce := sync.Once{}
	go func() {
		logx.Info("NewAppState")
		<-confreadych.Ch

		var xerr error
		defer func() {
			logx.Infof("[Sync:False] err: %v\n", xerr)
		}()

		logx.Infof("[Sync:True] err: %v\n", xerr)
		for {
			xerr = registerState(appStateVal)
			if xerr != lib.NilXerr {
				return
			}
			waitFirstSyncChOnce.Do(func() {
				close(waitFirstSyncCh)
			})

			conf := readConf()
			time.Sleep(time.Second * time.Duration(conf.Lease) / 3)
		}
	}()
	logx.Info("appState register wait")
	<-waitFirstSyncCh
	logx.Info("appState register done")
	config := readConf()
	watchRoute(context.Background(), config)
	logx.Info("route watch done")
	return appStateVal
}

func registerState(appStateVal atomic.Value) error {
	ctx := context.Background()

	conf := readConf()
	etcdCli, err := etcdLib.New(etcdLib.Config{
		Endpoints:   strings.Split(conf.EtcdAddrs, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		xerr := lib.NewXError(err, "Connect Etcd Failed")

		return xerr
	}
	defer etcdCli.Close()

	leaseResp, err := etcdCli.Grant(ctx, conf.Lease)
	if err != nil {
		xerr := lib.NewXError(err, "Get Grant Fail")
		return xerr
	}
	leaseID := leaseResp.ID

	stateVal := WriteIntoJSONIndent(appStateVal.Load())
	for _, key := range conf.AppStatePaths() {
		_, err := etcdCli.Put(ctx, key, stateVal, etcdLib.WithLease(leaseID))
		if err != nil {
			xerr := lib.NewXError(err, fmt.Sprintf("set key %s fail: %v", key, err))
			return xerr
		}
		// logx.Infof("register key: %v val: %v", key, stateVal)
	}
	return nil
}
