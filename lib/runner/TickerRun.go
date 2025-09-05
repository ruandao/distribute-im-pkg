package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ruandao/distribute-im-pkg/lib/logx"
	"github.com/ruandao/distribute-im-pkg/lib/xerr"
)

func Go(n int, f func()) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	for range n {
		GoWithWaitGroup(wg, f)
	}
	return wg
}
func RunAndWait(n int, f func()) {
	wg := Go(n, f)
	wg.Wait()
}

func Keys[T any](m map[string]T) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func Arr2Ch[T any](arr []T) chan T {
	ch := make(chan T, len(arr))
	for _, key := range arr {
		ch <- key
	}
	close(ch)
	return ch
}

type KeyArrItem[T any] struct {
	Key string
	Arr []T
}

func MapReduce[InputElem any, OutputElem any](ctx context.Context, runnerName string,
	inputCh chan InputElem, mapF func(elem InputElem) string,
	routineSize int,
	elemTask func(key string, inputElem []InputElem) OutputElem) chan OutputElem {

	outputCh := make(chan OutputElem)
	m := make(map[string][]InputElem)
	GoAndClose(outputCh, 1, func() {
		for elem := range inputCh {
			key := mapF(elem)
			arr := m[key]
			arr = append(arr, elem)
			m[key] = arr
		}
		tmpCh := make(chan KeyArrItem[InputElem], len(m))
		for key, arr := range m {
			tmpCh <- KeyArrItem[InputElem]{Key: key, Arr: arr}
		}
		close(tmpCh)
		RangeCh(tmpCh, routineSize, func(item KeyArrItem[InputElem]) {
			outputCh <- elemTask(item.Key, item.Arr)
		})
	})
	return outputCh
}

// don't forget to close inputCh
func MapBatchTicket[InputElem any, OutputElem any](ctx context.Context, runnerName string,
	inputCh chan InputElem, mapF func(elem InputElem) string,
	bufferSize int, duration time.Duration, batchSize int,
	elemTask func(key string, inputElem []InputElem) OutputElem) chan OutputElem {

	if bufferSize <= 0 {
		err := xerr.NewError(fmt.Sprintf("bufferSize should large than 0, but get %v", bufferSize))
		panic(err)
	}
	outputCh := make(chan OutputElem)
	m := make(map[string]chan InputElem)
	GoAndClose(outputCh, 1, func() {
		wg := &sync.WaitGroup{}
		for elem := range inputCh {
			key := mapF(elem)
			ch := m[key]
			if ch == nil {
				ch = make(chan InputElem, bufferSize)
				m[key] = ch
				GoWithWaitGroup(wg, func() {
					batchCh := TicketBatch(ctx, runnerName, 1, duration, batchSize, ch)
					for item := range batchCh {
						newV := elemTask(key, item)
						// fmt.Printf("key: %v newV: %v <- item: %v\n", key, newV, item)
						outputCh <- newV
					}
				})
			}
			// fmt.Printf("currentKey: %v elem: %v keys: %v\n", key, elem, Keys(m))
			ch <- elem
		}

		for _, ch := range m {
			close(ch)
		}
		wg.Wait()
	})
	return outputCh
}

func TicketBatch[InputElem any](ctx context.Context, runnerName string,
	bufferSize int, duration time.Duration, batchSize int,
	inputCh chan InputElem) chan []InputElem {

	ch := make(chan []InputElem, bufferSize)
	ticket := time.NewTicker(duration)
	var buf []InputElem

	GoAndClose(ch, 1, func() {
		RunForever(ctx, "TicketBatch"+runnerName, func(runCnt int) (goon bool) {
			select {
			case item, open := <-inputCh:
				if !open {
					ch <- buf
					return false
				}
				buf = append(buf, item)
				// 定量
				if len(buf) == batchSize {
					ch <- buf
					buf = nil
				}
			case <-ticket.C:
				// 定时, 可以为nil，表示，这个期间没有新的数据
				ch <- buf
				buf = nil
			}

			return true
		})
	})

	return ch
}

func GoRangeCh[T any](ch chan T, n int, f func(item T)) {
	go RangeCh(ch, n, f)
}
func RangeCh[T any](ch chan T, n int, f func(item T)) {
	wg := Go(n, func() {
		for item := range ch {
			f(item)
		}
	})
	wg.Wait()
}

// 一般而言, RunAndClose 中， f 函数 产生的内容，会放入到 ch 中，
// 然后所有的 f 运行完了，就 close ch
func RunAndClose[T any](ch chan T, n int, f func()) {
	wg := Go(n, f)
	wg.Wait()
	close(ch)
}
func GoAndGetResult[T any](bufferSize int, routineSize int, f func(result chan T)) chan T {
	ch := make(chan T, bufferSize)
	GoAndClose(ch, routineSize, func() {
		f(ch)
	})
	return ch
}

// input -> [concurrencySize process by f] -> output
func GoPipe[OutputElem any, InputElem any](bufferSize int, routineSize int, inputCh chan InputElem, f func(inputElem InputElem, resultCh chan OutputElem)) chan OutputElem {
	ch := make(chan OutputElem, bufferSize)
	GoAndClose(ch, routineSize, func() {
		for inputElem := range inputCh {
			f(inputElem, ch)
		}
	})
	return ch
}

func GoAndClose[T any](ch chan T, n int, f func()) {
	go RunAndClose(ch, n, f)
}

func GoCh(f func()) chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		f()
	}()
	return ch
}

func GoWithWaitGroup(wg *sync.WaitGroup, f func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
}

func RunForever(ctx context.Context, name string, taskF func(runCnt int) (goon bool)) {
	cnt := 0
	for {
		cnt++
		if cnt%1000 == 0 {
			logx.DebugX(name)(cnt)
		}
		if ctx.Err() != nil {
			return
		}
		goon := taskF(cnt)
		if !goon {
			return
		}
	}
}

func RunWithTicker(ctx context.Context, runName string, ticker time.Ticker, taskF func() bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go RunForever(ctx, fmt.Sprintf("%v.RunWithTicker", runName), func(runCnt int) bool {
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
func RunWithTickerDuration(ctx context.Context, runName string, tickerDuration time.Duration, taskF func() (goon bool)) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(tickerDuration)

	go RunForever(ctx, fmt.Sprintf("%v.RunWithTickerDuration", runName), func(runCnt int) (goon bool) {
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

func RunWithCancel(ctx context.Context, runName string, taskF func(runCnt int) bool) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	go RunForever(ctx, fmt.Sprintf("%v.RunWithCancel", runName), func(runCnt int) bool {
		select {
		case <-ctx.Done():
			return false
		default:
			goon := taskF(runCnt)
			if !goon {
				cancel()
				return false
			}
		}
		return true
	})
	return ctx, cancel
}
