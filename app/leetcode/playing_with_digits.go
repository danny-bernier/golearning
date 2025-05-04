package leetcode

import (
	"math"
	"strconv"
)

func DigPow(n, p int) int {
	//get digits
	numString := strconv.Itoa(n)
	digits := make([]int, len(numString))
	for i, r := range numString {
		d, _ := strconv.Atoi(string(r))
		digits[i] = d
	}

	// rise to pow to get sum
	sum := 0.0
	for _, d := range digits {
		sum += math.Pow(float64(d), float64(p))
		p += 1
	}

	// check if there is a whole k value
	k := sum / float64(n)
	if k == math.Trunc(k) {
		return int(k)
	}
	return -1
}
