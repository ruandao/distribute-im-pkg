package traffic

import "context"

// 定义一个自定义类型替代空匿名结构体
type SymbolRouteTag struct{}

func GetRouteTag(ctx context.Context) string {
	tag := ctx.Value(SymbolRouteTag{})
	if tag == nil {
		return "default"
	}
	return tag.(string)
}

func TagRoute(ctx context.Context, routeTag string) context.Context {
	ctx = context.WithValue(ctx, SymbolRouteTag{}, routeTag)
	return ctx
}
