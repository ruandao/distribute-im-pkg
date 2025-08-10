package mdb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ruandao/distribute-im-pkg/config/appConfigLib"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/randx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	"github.com/ruandao/distribute-im-pkg/xetcd"

	"github.com/ruandao/distribute-im-pkg/traffic"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DBConfig struct {
	User     string
	Password string
	Addr     string
	DBName   string
}

func (dbConfig DBConfig) getDSN() string {
	// dsn := "user:password@tcp(127.0.0.1:3306)/test_db?charset=utf8mb4&parseTime=True&loc=Local"
	return fmt.Sprintf("%v:%v@tcp(%v)/%v?charset=utf8mb4&parseTime=True&loc=Local", dbConfig.User, dbConfig.Password, dbConfig.Addr, dbConfig.DBName)
}

func DBConfigFrom(val string) (*DBConfig, error) {
	dbConfig := &DBConfig{}
	err := json.Unmarshal([]byte(val), dbConfig)
	if err != nil {
		return nil, err
	}
	return dbConfig, nil
}

// GetDB 时,需要确认连接哪一个业务的哪个分片(通过业务ID),需要传入的信息有:业务名,业务ID
// 为什么 redis 使用的时候,不需要提前确认分片,而数据库的需要:
// redis 除了少量的命令会涉及到较为复杂的key情况(譬如, keys), 其他的命令都是很简单的和key相关的指令
// mysql 的查询语句,很容易出现复杂的情况,难以隔离具体的业务意图直接在中间层提前计算出分片信息, 譬如 where ... in 语句,对应的字段是否是分片字段(同一个库中有多个表), 这个并不确定
// 所以我们并不直接屏蔽分片的感知,而是让调用方提前思考好具体分片依赖的信息,然后据此来获取实例的连接信息
// 路由标签,是一个动态信息,一般而言,调用方并不直接决定,so 通过一个context 传入即可
func GetDB(ctx context.Context, bizName string, shareId string) (*gorm.DB, error) {
	// we should calculate shareKey from shareId
	shares := appConfigLib.GetAppConfig().GetShares(ctx, bizName)
	shareKey := appConfigLib.ShareKeyFromId(shares, shareId)
	return GetDBByShareKey(ctx, bizName, shareKey, nil)
}

func GetDBByShareKey(ctx context.Context, bizName string, shareKey string, _dbConfig *DBConfig) (*gorm.DB, error) {
	// 我们将根据 shareId 计算出 shareKey
	// shareKey := "db0"
	dbConfig := _dbConfig
	if dbConfig == nil {
		var err error
		dbConfig, err = _getDBConfig(ctx, bizName, shareKey)
		if err != nil {
			return nil, err
		}
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dbConfig.getDSN()), &gorm.Config{})
	if err != nil {
		return nil, xerr.NewXError(err, "连接数据库失败")
	}
	return db, nil
}

func ForEachShare(ctx context.Context, bizName string, cb func(shareKey string, retryCnt int, db *gorm.DB) (stopRetry bool)) {
	shares := appConfigLib.GetAppConfig().GetShares(ctx, bizName)
	for _, shareKey := range shares {
		var retryCnt = 0
		for {
			db, _ := GetDBByShareKey(ctx, bizName, shareKey, nil)
			stopRetry := cb(shareKey, retryCnt, db)
			retryCnt++
			if stopRetry {
				break
			}
		}
	}
}

func _getDBConfig(ctx context.Context, bizName string, shareKey string) (*DBConfig, error) {
	routeTag := traffic.GetRouteTag(ctx)
	instanceConfig := _xContent.GetDepServicesShareDBInstancesConfig(bizName, routeTag, shareKey)
	conn := instanceConfig.ConnConfig[randx.RandomInt(len(instanceConfig.ConnConfig))]
	var _dbConfig = &DBConfig{}
	err := json.Unmarshal([]byte(conn), _dbConfig)
	return _dbConfig, err
}

var _xContent *xetcd.XContent

func RegisterC(ctx context.Context, bizNameList []string, xContent *xetcd.XContent) {
	// 传入 bizNameList 的目的是, 当配置变化时,做一次连接测试,方便及时的把报错信息暴露出来
	_xContent = xContent

	for _, bizName := range bizNameList {
		xContent.ClusterWatch(bizName, func(rsc *xetcd.RouteShareConns) {
			for routeTag, shareInstance := range rsc.M {
				for shareKey, nodeConfig := range shareInstance {
					for _, conn := range nodeConfig.ConnConfig {
						dbConfig := &DBConfig{}
						err := json.Unmarshal([]byte(conn), dbConfig)
						if err != nil {
							msg := "数据库连接,解析失败:\n" +
								fmt.Sprintf("业务: %v 路由: %v 分片: %v\n", bizName, routeTag, shareKey) +
								fmt.Sprintf("错误信息: %v\n", err)
							logx.Errorf("%s", msg)
						}
						ctx := traffic.TagRoute(ctx, routeTag)
						db, err := GetDBByShareKey(ctx, bizName, "", dbConfig)
						if err != nil {
							msg := "数据库连接失败:\n" +
								fmt.Sprintf("业务: %v 路由: %v 分片: %v\n", bizName, routeTag, shareKey) +
								fmt.Sprintf("错误信息: %v\n", err)
							logx.Errorf("%s", msg)
						}
						err = SelectTest(db)
						if err != nil {
							msg := "数据库SelectTest失败:\n" +
								fmt.Sprintf("业务: %v 路由: %v 分片: %v\n", bizName, routeTag, shareKey) +
								fmt.Sprintf("错误信息: %v\n", err)
							logx.Errorf("%s", msg)
						}
					}
				}
			}
		})
	}
}

func SelectTest(db *gorm.DB) error {
	var result int
	err := db.Raw("SELECT 1").Scan(&result).Error
	return err
}
