package util

import (
	"strings"

	"github.com/Pallinder/go-randomdata"
)

func RandomName(numA int) string {
	noun := capitalizeFirst(randomdata.Noun())
	if numA <= 0 {
		return noun
	}

	adjectiveSet := make(map[string]struct{})
	adjectives := make([]string, numA-1)
	for i := 0; i < numA-1; i++ {
		a := randomdata.Adjective()
		if _, ok := adjectiveSet[a]; !ok {
			adjectiveSet[a] = struct{}{}
			adjectives[i] = capitalizeFirst(a)
			i += 1
		}
	}
	return strings.Join(adjectives, "") + noun
}

func RandomNames(w int, n int) []string {
	nameSet := make(map[string]struct{})
	names := make([]string, n)

	for i := 0; i < n; {
		name := RandomName(w)
		if _, ok := nameSet[name]; !ok {
			nameSet[name] = struct{}{}
			names[i] = name
			i += 1
		}
	}
	return names
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
