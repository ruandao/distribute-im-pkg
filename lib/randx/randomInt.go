package randx

import (
	"math/rand"
	"time"
)

var _rand *rand.Rand

func init() {
	// 程序启动时设置一次种子即可
	_rand = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func RandomInt(n int) int {
	if n <= 0 {
		return 0
	}
	return _rand.Intn(n)
}

func SelectOne[T any](arr []T) T {
	return RandomOne(arr)
}


func RandomOne[T any](arr []T) T {
	arrLen := len(arr)
	return arr[RandomInt(arrLen)]
}