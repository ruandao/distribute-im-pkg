package middlewarelib

import (
	"context"
	"net/http"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
)

// 认证中间件
var LogReqHandler HandF = func(ctx context.Context, w http.ResponseWriter, r *http.Request, nextF NextF) {
	logx.Infof("path: %v\n", r.URL.Path)
	nextF(ctx, w, r)
}
