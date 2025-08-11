package appConfigLib

import (
	"context"
	"fmt"
	"hash/crc32"
	"sort"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/xetcd"

	"github.com/ruandao/distribute-im-pkg/config/basicConfig"

	"github.com/ruandao/distribute-im-pkg/lib"

	"github.com/ruandao/distribute-im-pkg/lib/xerr"
)

type FlyConfig struct {
	TrafficTags []string `mapstructure:"routeTags"`
	// 为什么需要从配置中读取？程序是知道自己依赖哪些服务的，但是编排程序的人不知道，所以需要强制二者一致，这样部署维护就更容易
	DepServices []string `mapstructure:"depServices"`
}
type FlyConfigWatch struct {
	flyConfig      *FlyConfig
	keyWatchRemove func()
}

type AppConfig struct {
	BConfig        *basicConfig.BConfig
	XContent       *xetcd.XContent
	flyConfigWatch *FlyConfigWatch
}

func (appConf *AppConfig) Watch() {
	if appConf.flyConfigWatch != nil {
		return
	}

	flyConfigWatch := &FlyConfigWatch{}
	parseFlyConfig := func(s string, err error) {
		if err != nil {
			logx.Errorf("%v", xerr.NewXError(err, fmt.Sprintf("can't get flyConfig for %v ", appConf.ConfigKeyPrefix())))
			flyConfigWatch.flyConfig = nil
		} else {
			flyConfig := &FlyConfig{}
			err = lib.ReadFromJSON([]byte(s), flyConfig)
			if err != nil {
				logx.Errorf("%v", xerr.NewXError(err, fmt.Sprintf("can't parse flyConfig for %v of val: %v", appConf.ConfigKeyPrefix(), s)))
			}
			flyConfigWatch.flyConfig = flyConfig
		}
	}
	
	flyConfigWatch.keyWatchRemove = appConf.XContent.KeyWatch(appConf.ConfigKeyPrefix(), func(s string, err error) {
		parseFlyConfig(s, err)
	})
	val, err := appConf.XContent.Get(appConf.ConfigKeyPrefix())
	parseFlyConfig(val, err)

	appConf.flyConfigWatch = flyConfigWatch
}

func (appConf *AppConfig) Close() {
	if appConf.flyConfigWatch != nil {
		appConf.flyConfigWatch.keyWatchRemove()
		appConf.flyConfigWatch = nil
	}
}

func (appConf *AppConfig) GetTrafficTags() []string {
	if appConf.flyConfigWatch != nil && appConf.flyConfigWatch.flyConfig != nil {
		return appConf.flyConfigWatch.flyConfig.TrafficTags
	}
	return nil
}

func (appConf *AppConfig) SplitIdToSeparateShare(ctx context.Context, bizName string, shareIdList []string) (map[xetcd.ShareName][]string, error) {
	shares, err := appConf.GetShares(ctx, bizName)
	if err != nil {
		return nil, err
	}
	m := make(map[xetcd.ShareName][]string)
	for _, shareId := range shareIdList {
		shareKey := ShareKeyFromId(shares, shareId)
		shareIdListPiece := m[shareKey]
		shareIdListPiece = append(shareIdListPiece, shareId)
		m[shareKey] = shareIdListPiece
	}
	return m, nil
}

func (appConf *AppConfig) GetShares(ctx context.Context, bizName string) ([]xetcd.ShareName, error) {
	routeShareConns, err := appConf.XContent.GetDepServicesCluster(bizName)
	if err != nil {
		return nil, err
	}

	var shares []xetcd.ShareName
	for shareKey := range routeShareConns.M {
		shares = append(shares, shareKey)
	}

	return shares, nil
}

func (appConf *AppConfig) GetShareKeyFromId(ctx context.Context, bizName string, shareId string) (xetcd.ShareName, error) {
	shares, err := appConf.GetShares(ctx, bizName)
	if err != nil {
		return "", err
	}
	return ShareKeyFromId(shares, shareId), nil
}

func ShareKeyFromId(shares []xetcd.ShareName, shareId string) xetcd.ShareName {
	if len(shares) == 0 {
		return ""
	}

	// 创建虚拟节点来提高哈希分布的均匀性
	type Node struct {
		Hash  uint32
		Share xetcd.ShareName
	}

	var hashRing []Node
	virtualNodes := 10 // 每个真实节点对应的虚拟节点数量

	// 将所有节点(包括虚拟节点)添加到哈希环
	for _, share := range shares {
		for i := 0; i < virtualNodes; i++ {
			hash := crc32.ChecksumIEEE([]byte(fmt.Sprintf("%s:%d", share, i)))
			hashRing = append(hashRing, Node{Hash: hash, Share: share})
		}
	}

	// 对哈希环进行排序
	sort.Slice(hashRing, func(i, j int) bool {
		return hashRing[i].Hash < hashRing[j].Hash
	})

	// 计算 shareId 的哈希值
	idHash := crc32.ChecksumIEEE([]byte(shareId))

	// 查找第一个大于等于 shareId 哈希值的节点
	for _, node := range hashRing {
		if node.Hash >= idHash {
			return node.Share
		}
	}

	// 如果没有找到(shareId的哈希值大于所有节点的哈希值)，则返回第一个节点
	return hashRing[0].Share
}

// 获取应用自身配置的键
func (appConf *AppConfig) ConfigKeyPrefix() string {
	configPath := fmt.Sprintf("/appConfig/%v/%v/%v/%v/config", appConf.BConfig.BusinessName, appConf.BConfig.Role, appConf.BConfig.ShareName, appConf.BConfig.Version)
	return configPath
}

// 生成存放应用状态的键
func (appConf *AppConfig) StateKeys() []string {
	// /appState/auth/mysql/db0/default/127.0.0.1:3306/state
	// `keyPrefix`/`shareName`/`routeTag`/`instanceIPPort`/state
	keys := []string{}
	for _, routeTag := range appConf.GetTrafficTags() {
		configPath := fmt.Sprintf("/appState/%v/%v/%v/%v/%v/state",
			appConf.BConfig.BusinessName,
			appConf.BConfig.Role,
			appConf.BConfig.ShareName,
			routeTag,
			appConf.BConfig.RegisterAddr(),
		)
		keys = append(keys, configPath)
	}
	return keys
}

var _appConfig *AppConfig

func NewAppConfig(basicConfig *basicConfig.BConfig, xContent *xetcd.XContent) *AppConfig {
	appConfig := &AppConfig{BConfig: basicConfig, XContent: xContent, flyConfigWatch: nil}
	appConfig.Watch()
	return appConfig
}
func RegisterAppConfig(appConfig *AppConfig) {
	_appConfig = appConfig
}
func GetAppConfig() *AppConfig {
	if _appConfig == nil {
		panic("plase do RegisterAppConfig first")
	}
	return _appConfig
}
