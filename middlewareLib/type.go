package middlewarelib

import (
	"context"
	"net/http"
)

type NextF func(ctx context.Context, w http.ResponseWriter, r *http.Request)
type HandF func(ctx context.Context, w http.ResponseWriter, r *http.Request, nextF NextF)

var EmptyNextF = func(ctx context.Context, w http.ResponseWriter, r *http.Request) {}
