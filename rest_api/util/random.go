package util

import (
	"strings"

	"github.com/Pallinder/go-randomdata"
)

func RandomName() string {
	a1 := randomdata.Adjective()
	a2 := randomdata.Adjective()
	n := randomdata.Noun()
	a1 = strings.ToUpper(a1[:1]) + a1[1:]
	a2 = strings.ToUpper(a2[:1]) + a2[1:]
	n = strings.ToUpper(n[:1]) + n[1:]
	return a1 + a2 + n
}
