package traffic

import (
	"context"
	"fmt"

	"github.com/ruandao/distribute-im-pkg/config/appConfigLib"
	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"
	"github.com/ruandao/distribute-im-pkg/xetcd"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TrafficConfig struct {
	TrafficIdKey TIDKey
	EndPoints    TrafficEndPoints
}

func NewConfig(tidKey string, TrafficEndPoints TrafficEndPoints) TrafficConfig {
	trafficConfig := TrafficConfig{TrafficIdKey: TIDKey(tidKey), EndPoints: TrafficEndPoints}
	return trafficConfig
}

func NewContext(trafficConfig TrafficConfig, ctx context.Context) context.Context {
	key := trafficConfig.TrafficIdKey
	if key == "" {
		key = "trafficIdKey"
	}

	tid := ctx.Value(key)
	if tid == nil {
		tid = NewTrafficID()
		ctx = context.WithValue(ctx, trafficConfig.TrafficIdKey, tid)
	}
	return ctx
}

func GetRPCClient(ctx context.Context, businessNode string) (*grpc.ClientConn, error) {
	reqTag := ctx.Value("TrafficTag")
	if reqTag == nil {
		reqTag = "default"
	}
	authEndPoints := ReadRouteEndPoints(reqTag, businessNode)
	trafficConfig := NewConfig("", authEndPoints)
	// ctx = NewContext(trafficConfig, ctx)
	for _, endPoint := range trafficConfig.EndPoints {
		dialOption := grpc.WithTransportCredentials(insecure.NewCredentials())
		conn, err := grpc.NewClient(endPoint, dialOption)
		if err != nil {
			logx.ErrorX("get client fail")(endPoint, err)
			continue
		}
		return conn, nil
	}
	return nil, xerr.NewXError(fmt.Errorf("not endpoint found for %v of %v node", businessNode, reqTag), "")
}

func GetRPCConnX(ctx context.Context, bizPrefix string, shareId string) (conn *grpc.ClientConn, err error) {
	routeTag := GetRouteTag(ctx)
	// shareKey :=
	shareKey, err := appConfigLib.GetAppConfig().GetShareKeyFromId(ctx, bizPrefix, shareId)
	if err != nil {
		return nil, xerr.NewXError(err, "获取shareKey失败")
	}
	shareConfig, err := xetcd.Get().GetDepServicesShareDBInstancesConfig(bizPrefix, shareKey, routeTag)
	if err != nil {
		return nil, err
	}
	shareConfig.ConnConfig.Range(func(key, value any) bool {
		ipport := key.(string)
		conn, err = GetRPCConn(ctx, ipport)
		if err != nil {
			logx.ErrorX("grpc 连接 ipport 失败")(ipport, err)
			return true
		}
		return false
	})
	return conn, err
}
func GetRPCConn(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	dialOption := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.NewClient(endpoint, dialOption)
	if err != nil {
		return nil, xerr.NewXError(err, "连接失败")
	}
	return conn, nil
}
