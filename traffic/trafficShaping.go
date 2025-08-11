package traffic

import (
	"context"
	"fmt"

	"github.com/ruandao/distribute-im-pkg/lib/xerr"

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
			return nil, xerr.NewXError(err, fmt.Sprintf("get client for %v fail: %v", endPoint, err))
		}
		return conn, nil
	}
	return nil, xerr.NewXError(fmt.Errorf("not endpoint found for %v of %v node", businessNode, reqTag), "")
}
