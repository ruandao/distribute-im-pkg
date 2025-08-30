package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
)

func RunForever(ctx context.Context, name string, taskF func() bool) {
	cnt := 0
	for {
		cnt++
		if cnt %1000 == 0 {
			logx.DebugX(name)()
		}
		
		goon := taskF()
		if !goon {
			return
		}
	}
}

func RunWithTicker(ctx context.Context, runName string, ticker time.Ticker, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go RunForever(ctx, fmt.Sprintf("%v.RunWithTicker", runName), func() bool {
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
func RunWithTickerDuration(ctx context.Context, runName string, tickerDuration time.Duration, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(tickerDuration)

	go RunForever(ctx, fmt.Sprintf("%v.RunWithTickerDuration", runName), func() bool {
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

func RunWithCancel(ctx context.Context, runName string, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go RunForever(ctx, fmt.Sprintf("%v.RunWithCancel", runName), func() bool {
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
