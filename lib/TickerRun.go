package lib

import (
	"context"
	"time"
)

func TickerRun(ctx context.Context, ticker *time.Ticker, ticketF func()) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ticketF()
			}
		}
	}()
	return ctx, cancel
}
