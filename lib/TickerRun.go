package lib

import (
	"context"
	"time"
)

func TickerRun(ctx context.Context, ticker *time.Ticker, f func()) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				f()
			}
		}
	}()
	return ctx, cancel
}
