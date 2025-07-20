package middlewarelib

import (
	"context"
	"net/http"
)

func H(fArr ...HandF) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqCtx := r.Context()

		if len(fArr) == 1 {
			fArr[0](reqCtx, w, r, EmptyNextF)
			return
		} else {
			firstF := fArr[0]
			othersF := fArr[1:]
			firstF(reqCtx, w, r, func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				// 构建新请求，替换 context
				newReq, err := http.NewRequestWithContext(
					ctx,
					r.Method,
					r.URL.String(),
					r.Body,
				)
				if err != nil {
					http.Error(w, "Failed to create new request", http.StatusInternalServerError)
					return
				}

				// 复制 headers
				newReq.Header = make(http.Header)
				for key, values := range r.Header {
					newReq.Header[key] = values
				}
				H(othersF...)(w, newReq)
			})
		}

	}
}
