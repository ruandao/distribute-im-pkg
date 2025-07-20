package lib

import (
	"sync"
)

type WaitGroupVal struct {
	wg  sync.WaitGroup
	val int
	sync.RWMutex
}

func NewWaitGroupVal() WaitGroupVal {
	return WaitGroupVal{}
}

func (wgv *WaitGroupVal) Add(v int) {
	wgv.RWMutex.Lock()
	defer wgv.RWMutex.Unlock()

	wgv.val += v
	wgv.wg.Add(v)
}

func (wgv *WaitGroupVal) Val() int {
	wgv.RWMutex.RLocker()
	defer wgv.RWMutex.RLocker().Unlock()
	return wgv.val
}

func (wgv *WaitGroupVal) Done() {
	wgv.RWMutex.Lock()
	defer wgv.RWMutex.Unlock()

	wgv.wg.Done()
	wgv.val -= 1
}

func (wgv *WaitGroupVal) Wait() {
	wgv.wg.Wait()
}
