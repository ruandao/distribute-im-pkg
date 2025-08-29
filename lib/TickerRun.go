package lib

import (
	"context"
	"time"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
)

func RunForever(ctx context.Context, name string, taskF func() bool) {
	for {
		logx.DebugX(name)()
		goon := taskF()
		if !goon {
			return
		}
	}
}

func RunWithTicker(ctx context.Context, ticker time.Ticker, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go RunForever(ctx, "RunWithTicker", func() bool {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			goon := taskF()
			if !goon {
				cancel()
				return false
			}
		}
		return true
	})

	return ctx, cancel
}
func RunWithTickerDuration(ctx context.Context, tickerDuration time.Duration, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(tickerDuration)

	go RunForever(ctx, "RunWithTickerDuration", func() bool {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			goon := taskF()
			if !goon {
				cancel()
				ticker.Stop()
				return false
			}
		}
		return true
	})

	return ctx, cancel
}

func RunWithCancel(ctx context.Context, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go RunForever(ctx, "RunWithCancel", func() bool {
		select {
		case <-ctx.Done():
			return false
		default:
			goon := taskF()
			if !goon {
				cancel()
				return false
			}
		}
		return true
	})
	return ctx, cancel
}
