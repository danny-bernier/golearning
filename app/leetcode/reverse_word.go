package leetcode

import (
	"strings"
)

func ReverseWords(str string) string {
	words := strings.Split(str, " ")
	for i, word := range words {
		words[i] = reverseWord(word)
	}
	return strings.Join(words, " ")
}

func reverseWord(str string) string {
	runes := []rune(str)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
