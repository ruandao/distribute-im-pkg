package mdb

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

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

type IShareModel interface {
	TableName() string
	ShareId() string
}

// GetDB 时,需要确认连接哪一个业务的哪个分片(通过业务ID),需要传入的信息有:业务名,业务ID
// 为什么 redis 使用的时候,不需要提前确认分片,而数据库的需要:
// redis 除了少量的命令会涉及到较为复杂的key情况(譬如, keys), 其他的命令都是很简单的和key相关的指令
// mysql 的查询语句,很容易出现复杂的情况,难以隔离具体的业务意图直接在中间层提前计算出分片信息, 譬如 where ... in 语句,对应的字段是否是分片字段(同一个库中有多个表), 这个并不确定
// 所以我们并不直接屏蔽分片的感知,而是让调用方提前思考好具体分片依赖的信息,然后据此来获取实例的连接信息
// 路由标签,是一个动态信息,一般而言,调用方并不直接决定,so 通过一个context 传入即可
func GetDB(ctx context.Context, bizName string, model IShareModel) (*gorm.DB, error) {
	// we should calculate shareKey from shareId
	shares, err := appConfigLib.GetAppConfig().GetShares(ctx, bizName)
	if err != nil {
		return nil, err
	}
	shareKey := appConfigLib.ShareKeyFromId(shares, model.ShareId())
	db, err := GetDBByShareKey(ctx, bizName, shareKey, nil)
	if err != nil {
		return nil, err
	}

	if model != nil {
		db = db.Table(model.TableName())
	}
	// return db.Debug(),err
	return db,err
}

var dbMap sync.Map
var dbMapLocker sync.Mutex

func GetDBByShareKey(ctx context.Context, bizName string, shareKey xetcd.ShareName, _dbConfig *DBConfig) (*gorm.DB, error) {
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

	dbConfigDSN := dbConfig.getDSN()
	val, ok := dbMap.Load(dbConfigDSN)
	if !ok {
		dbMapLocker.Lock()
		defer dbMapLocker.Unlock()

		val, ok = dbMap.Load(dbConfigDSN)
		if ok {
			return val.(*gorm.DB), nil
		}
		// 连接数据库
		db, err := gorm.Open(mysql.Open(dbConfigDSN), &gorm.Config{})
		if err != nil {
			return nil, xerr.NewXError(err, "连接数据库失败")
		}
		dbMap.Store(dbConfigDSN, db)
		return db, nil
	}
	return val.(*gorm.DB), nil
}

func ForEachShare(ctx context.Context, bizName string, cb func(shareKey xetcd.ShareName, retryCnt int, db *gorm.DB, err error) (stopRetry bool)) error {
	shares, err := appConfigLib.GetAppConfig().GetShares(ctx, bizName)
	if err != nil {
		return err
	}
	for _, shareKey := range shares {
		var retryCnt = 0
		for {
			db, err := GetDBByShareKey(ctx, bizName, shareKey, nil)
			stopRetry := cb(shareKey, retryCnt, db, err)
			retryCnt++
			if stopRetry {
				break
			}
		}
	}
	return nil
}

func _getDBConfig(ctx context.Context, bizName string, shareKey xetcd.ShareName) (*DBConfig, error) {
	routeTag := traffic.GetRouteTag(ctx)
	instanceConfig, err := _xContent.GetDepServicesShareDBInstancesConfig(bizName, shareKey, routeTag)
	if err != nil {
		return nil, xerr.NewXError(err, fmt.Sprintf("can't not found config on %v %v %v", bizName, shareKey, routeTag))
	}
	conn := randx.SelectOne(instanceConfig.ConnConfig)
	var _dbConfig = &DBConfig{}
	err = json.Unmarshal([]byte(conn), _dbConfig)
	return _dbConfig, err
}

var _xContent *xetcd.XContent

func RegisterXContent(ctx context.Context, bizNameList []string, xContent *xetcd.XContent) {
	// 传入 bizNameList 的目的是, 当配置变化时,做一次连接测试,方便及时的把报错信息暴露出来
	_xContent = xContent

	for _, bizName := range bizNameList {
		xContent.ClusterWatch(bizName, func(rsc *xetcd.RouteShareConns, err error) {
			if err != nil {
				logx.Errorf("db config for %v no found, err: %v", bizName, err)
				return
			}
			for shareKey, shareInstance := range rsc.M {
				for routeTag, nodeConfig := range shareInstance {
					for _, conn := range nodeConfig.ConnConfig {
						dbConfig := &DBConfig{}
						err := json.Unmarshal([]byte(conn), dbConfig)
						if err != nil {
							msg := "数据库连接,解析失败:\n" +
								fmt.Sprintf("业务: %v 路由: %v 分片: %v\n", bizName, routeTag, shareKey) +
								fmt.Sprintf("值: %v\n", conn) +
								fmt.Sprintf("错误信息: %v\n", err)
							logx.Errorf("%s", msg)
							continue
						}
						ctx := traffic.TagRoute(ctx, routeTag)
						db, err := GetDBByShareKey(ctx, bizName, "", dbConfig)
						if err != nil {
							msg := "数据库连接失败:\n" +
								fmt.Sprintf("业务: %v 路由: %v 分片: %v\n", bizName, routeTag, shareKey) +
								fmt.Sprintf("值: %v\n", conn) +
								fmt.Sprintf("错误信息: %v\n", err)
							logx.Errorf("%s", msg)
							continue
						}
						err = SelectTest(db)
						if err != nil {
							msg := "数据库SelectTest失败:\n" +
								fmt.Sprintf("业务: %v 路由: %v 分片: %v\n", bizName, routeTag, shareKey) +
								fmt.Sprintf("值: %v\n", conn) +
								fmt.Sprintf("错误信息: %v\n", err)
							logx.Errorf("%s", msg)
							continue
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
