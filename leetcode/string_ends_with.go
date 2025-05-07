package leetcode

func solution(str, ending string) bool {
	if len(ending) > len(str) {
		return false
	}
	last := str[len(str)-len(ending):]
	return last == ending
}
