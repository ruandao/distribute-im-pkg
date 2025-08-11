package traffic

import (
	"context"

	"github.com/ruandao/distribute-im-pkg/xetcd"
)

// 定义一个自定义类型替代空匿名结构体
type SymbolRouteTag struct{}

func GetRouteTag(ctx context.Context) xetcd.RouteTag {
	tag := ctx.Value(SymbolRouteTag{})
	if tag == nil {
		return "default"
	}
	return tag.(xetcd.RouteTag)
}

func TagRoute(ctx context.Context, routeTag xetcd.RouteTag) context.Context {
	ctx = context.WithValue(ctx, SymbolRouteTag{}, routeTag)
	return ctx
}
