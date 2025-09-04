package runner_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ruandao/distribute-im-pkg/lib/runner"
)

func TestGo(t *testing.T) {
	ch := make(chan int, 2)
	runner.Go(2, func() {
		ch <- 1
	})
	time.Sleep(time.Second)
	close(ch)
	sum := 0
	for i := range ch {
		sum += i
	}
	if sum != 2 {
		t.Errorf("Go(2, f) should run f twice")
	}
}

func TestRunAndWait(t *testing.T) {
	ch := make(chan int, 3)
	runner.RunAndWait(2, func() {
		ch <- 1
	})
	ch <- 3
	<-ch
	<-ch
	thirdElem := <-ch
	if thirdElem != 3 {
		t.Errorf("第三个元素应该是 3, 但是得到 %v\n", thirdElem)
	}
}

func TestGoWithWaitGroup(t *testing.T) {

	ch := make(chan int, 4)
	wg := &sync.WaitGroup{}
	runner.GoWithWaitGroup(wg, func() {
		ch <- 1
		time.Sleep(2 * time.Second)
		ch <- 3
	})
	go func() {
		time.Sleep(time.Second)
		ch <- 2
	}()
	wg.Wait()
	ch <- 4

	firstElem := <-ch
	secondElem := <-ch
	thirdElem := <-ch
	fourthElem := <-ch
	// if no wait, thirdElem = 2, fourthElem = 3
	// if has wait, thirdElem = 3, fourthElem = 4
	if thirdElem != 3 || fourthElem != 4 {
		t.Errorf("wg 没有Wait %v, %v, %v, %v", firstElem, secondElem, thirdElem, fourthElem)
	}
}

func TestRunForever(t *testing.T) {
	cnt := 0
	targetCnt := 10010
	runner.RunForever(context.Background(), "testing", func() (goon bool) {
		if cnt < targetCnt {
			cnt++
			return true
		}
		return false
	})
	if cnt < targetCnt {
		t.Errorf("RunForever 运行次数 %v 少于目标运行次数 %v", cnt, targetCnt)
	}
}

// 一般而言, RunAndClose 中， f 函数 产生的内容，会放入到 ch 中，
// 然后所有的 f 运行完了，就 close ch
func TestRunAndClose(t *testing.T) {
	ch := make(chan int, 3)
	runner.RunAndClose(ch, 2, func() {
		ch <- 2
	})
	_firstElem := <-ch
	_secondElem := <-ch
	_thirdElem, chIsOpen := <-ch
	if chIsOpen {
		fmt.Printf("%v %v %v\n", _firstElem, _secondElem, _thirdElem)
		t.Errorf("在第三次 <- ch 时, ch 应该是已经关闭了的\n")
	}
}

func TestGoAndClose(t *testing.T) {
	ch := make(chan int, 3)
	runner.GoAndClose(ch, 2, func() {
		ch <- 2
	})
	ch <- 1
	firstElem := <-ch
	<-ch
	_, open := <-ch
	_, close := <-ch
	if firstElem != 1 || open != true || close != false {
		t.Errorf("第一次获取值应该是 1 得到 %v, 第三次获取值，ch 应该是开放的，第四次获取值， ch 应该是关闭的", firstElem)
	}
}

// func TicketBatch[InputElem any](ctx context.Context, runnerName string,
// 	bufferSize int, duration time.Duration, batchSize int,
// 	inputCh chan InputElem ) (chan []InputElem) {

func TestTicketBatch(t *testing.T) {
	ch := make(chan int, 10)
	outCh := runner.TicketBatch(context.Background(), "testing", 3, time.Second, 4, ch)
	for i := range 10 {
		ch <- i
	}
	close(ch)
	firstGroup := <-outCh
	secondGroup := <-outCh
	thirdGroup, open3 := <-outCh
	fourthGroup, open4 := <-outCh
	if open3 != true || open4 != false || len(thirdGroup) != 2 || thirdGroup[0] != 8 || thirdGroup[1] != 9 {
		fmt.Printf("%v %v (%v %v) (%v %v)", firstGroup, secondGroup, thirdGroup, open3, fourthGroup, open4)
		t.Errorf("expect thirdGroup should be [8, 9]")
	}

}

// func MapBatchTicket[InputElem any, OutputElem any](ctx context.Context, runnerName string,
// 	inputCh chan InputElem, mapF func(elem InputElem) string,
// 	bufferSize int, duration time.Duration, batchSize int,
// 	elemTask func(inputElem []InputElem) OutputElem ) (chan OutputElem) {

type Item struct {
	key   string
	elems []int
}

func TestMapBatchTicket(t *testing.T) {
	inputCh := make(chan int, 20)
	itemBufferSize := 10
	ticketDuration := time.Second
	batchSize := -1
	outCh := runner.MapBatchTicket(context.Background(), "testing", inputCh, func(elem int) string {
		return fmt.Sprintf("%v", elem%3)
	}, itemBufferSize, ticketDuration, batchSize, func(key string, inputElem []int) Item {
		return Item{key: key, elems: inputElem}
	})
	for i := range 40 {
		inputCh <- i
	}
	close(inputCh)
	m := make(map[string][]int)
	for item := range outCh {
		m[item.key] = append(m[item.key], item.elems...)
	}
	if len(m) != 3 {
		t.Errorf("m should contain keys: [0, 1, 2] but get: %v len: %v", runner.Keys(m), len(m))
	}
	if fmt.Sprintf("%v", m["0"]) != "[0 3 6 9 12 15 18 21 24 27 30 33 36 39]" {
		t.Errorf("m[0] should be %v but get: %v\n", "[0 3 6 9 12 15 18 21 24 27 30 33 36 39]", m["0"])
	}
	if fmt.Sprintf("%v", m["1"]) != "[1 4 7 10 13 16 19 22 25 28 31 34 37]" {
		t.Errorf("m[1] should be %v but get: %v\n", "[1 4 7 10 13 16 19 22 25 28 31 34 37]", m["1"])
	}
	if fmt.Sprintf("%v", m["2"]) != "[2 5 8 11 14 17 20 23 26 29 32 35 38]" {
		t.Errorf("m[2] should be %v but get: %v\n", "[2 5 8 11 14 17 20 23 26 29 32 35 38]", m["2"])
	}

}

// func GoAndGetResult[T any](bufferSize int, routineSize int, f func(result chan T)) (chan T) {
// 	ch := make(chan T, bufferSize)
// 	GoAndClose(ch, routineSize, func() {
// 		f(ch)
// 	})
// 	return ch
// }
// // input -> [concurrencySize process by f] -> output
// func GoPipe[OutputElem any, InputElem any](bufferSize int, routineSize int, inputCh chan InputElem, f func(inputElem InputElem, resultCh chan OutputElem)) (chan OutputElem) {
// 	ch := make(chan OutputElem, bufferSize)
// 	GoAndClose(ch, routineSize, func() {
// 		for inputElem := range inputCh {
// 			f(inputElem, ch)
// 		}
// 	})
// 	return ch
// }

// func GoCh(f func()) (chan struct{}) {
// 	ch := make(chan struct{})
// 	go func() {
// 		defer close(ch)
// 		f()
// 	}()
// 	return ch
// }

// func RunWithTicker(ctx context.Context, runName string, ticker time.Ticker, taskF func() bool) (context.Context, context.CancelFunc) {
// 	ctx, cancel := context.WithCancel(ctx)

// 	go RunForever(ctx, fmt.Sprintf("%v.RunWithTicker", runName), func() bool {
// 		select {
// 		case <-ctx.Done():
// 			return false
// 		case <-ticker.C:
// 			goon := taskF()
// 			if !goon {
// 				cancel()
// 				return false
// 			}
// 		}
// 		return true
// 	})

// 	return ctx, cancel
// }
// func RunWithTickerDuration(ctx context.Context, runName string, tickerDuration time.Duration, taskF func() (goon bool)) (context.Context, context.CancelFunc) {
// 	ctx, cancel := context.WithCancel(ctx)
// 	ticker := time.NewTicker(tickerDuration)

// 	go RunForever(ctx, fmt.Sprintf("%v.RunWithTickerDuration", runName), func() (goon bool) {
// 		select {
// 		case <-ctx.Done():
// 			return false
// 		case <-ticker.C:
// 			goon := taskF()
// 			if !goon {
// 				cancel()
// 				ticker.Stop()
// 				return false
// 			}
// 		}
// 		return true
// 	})

// 	return ctx, cancel
// }

// func RunWithCancel(ctx context.Context, runName string, taskF func() bool) (context.Context, context.CancelFunc) {
// 	ctx, cancel := context.WithCancel(ctx)

// 	go RunForever(ctx, fmt.Sprintf("%v.RunWithCancel", runName), func() bool {
// 		select {
// 		case <-ctx.Done():
// 			return false
// 		default:
// 			goon := taskF()
// 			if !goon {
// 				cancel()
// 				return false
// 			}
// 		}
// 		return true
// 	})
// 	return ctx, cancel
// }
