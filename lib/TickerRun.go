package lib

import (
	"context"
	"time"
)

func RunWithTicker(ctx context.Context, ticker time.Ticker, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				goon := taskF()
				if !goon {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}
func RunWithTickerDuration(ctx context.Context, tickerDuration time.Duration, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(tickerDuration)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				goon := taskF()
				if !goon {
					cancel()
					ticker.Stop()
					return
				}
			}
		}
	}()
	return ctx, cancel
}

func RunWithCancel(ctx context.Context, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				goon := taskF()
				if !goon {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}
