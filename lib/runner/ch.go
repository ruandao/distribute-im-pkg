package runner

func NilOrChan[T any](arr []T) chan T {
	if len(arr) == 0 {
		return nil
	}
	ch := make(chan T, len(arr))
	for _, item := range arr {
		ch <- item
	}
	close(ch)
	return ch
}
