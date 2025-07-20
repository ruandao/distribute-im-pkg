package confreadych

import "sync"

var Ch chan struct{}

func init() {
	Ch = make(chan struct{})
}

var closeOnce sync.Once

func Close() {
	closeOnce.Do(func() {
		close(Ch)
	})
}
