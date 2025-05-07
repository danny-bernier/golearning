package util

func SafeClose[T any](ch chan T) {
	if ch == nil {
		return
	}
	_, ok := <-ch
	if ok {
		close(ch)
	}
}
