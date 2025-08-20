package lib

import (
	"context"
	"time"
)

func TickerRun(ctx context.Context, ticker *time.Ticker, ticketF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				goon := ticketF()
				if !goon {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}
