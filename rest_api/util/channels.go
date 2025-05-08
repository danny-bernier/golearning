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

func IsChannelClosed(ch chan struct{}) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}
