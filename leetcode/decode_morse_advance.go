// https://www.codewars.com/kata/54b72c16cd7f5154e9000457
package leetcode

import (
	"fmt"
	"math"
	"strings"
)

// You may get original char by morse code like this: MORSE_CODE[char]

/*
"Dot" – is 1 time unit long.
"Dash" – is 3 time units long.
Pause between dots and dashes in a character – is 1 time unit long.
Pause between characters inside a word – is 3 time units long.
Pause between words – is 7 time units long.
*/

func DecodeBits(bits string) (morseCode string) {
	bits = strings.Trim(bits, "0")
	sampleRate := findSampleRate(bits)
	fmt.Printf("sampleRate: %d\n", sampleRate)
	normalMorse := normalizeMorse(bits, sampleRate)
	fmt.Printf("normalMorse: %s\n", normalMorse)

	i := 0
	for i < len(normalMorse) {
		if i+6 < len(normalMorse) && normalMorse[i:i+7] == "0000000" {
			morseCode += "   "
			i += 7
		} else if i+2 < len(normalMorse) && (normalMorse[i:i+3] == "000" || normalMorse[i:i+3] == "111") {
			if normalMorse[i:i+3] == "000" {
				morseCode += " "
			} else {
				morseCode += "−"
			}
			i += 3
		} else {
			if normalMorse[i:i+1] == "1" {
				morseCode += "·"
			}
			i += 1
		}
	}

	return
}

func DecodeMorse(morseCode string) string {
	fmt.Printf("Decoing morse: %s\n", morseCode)

	words := strings.Split(morseCode, "   ")
	fmt.Printf("words: %s\n", strings.Join(words, ", "))
	
	for i, w := range(words) {
		rawLetters := strings.Split(w, " ")
		var word string
		for _, l := range(rawLetters) {
			word += morseCodes(l)
		}
		fmt.Printf("decoded raw: %s into word: %s\n", w, word)
		words[i] = word
	}

	return strings.Join(words, " ")
}

func findSampleRate(bits string) int {
	fmt.Printf("Processing %d long string: %s\n", len(bits), bits)
	shortest := math.MaxInt
	for _, gb := range(groupBits(bits)) {
		if len(gb) < shortest {
			shortest = len(gb)
		}
	}
	return shortest
}

func groupBits(bits string) []string {
    var groups []string
    currentGroup := string(bits[0])

    for i := 1; i < len(bits); i++ {
        if bits[i] == bits[i-1] {
            currentGroup += string(bits[i])
        } else {
            groups = append(groups, currentGroup)
            currentGroup = string(bits[i])
        }
    }
    groups = append(groups, currentGroup)
    return groups
}

func normalizeMorse(bits string, sampleRate int) (normalMorse string) {
	for i := 0; i < len(bits); i += sampleRate {
		normalMorse += bits[i : i+1]
	}
	return
}

func morseCodes(morse string) string {
    morseToLetter := map[string]string{
        // Letters
        "·−":    "A",
        "−···":  "B",
        "−·−·":  "C",
        "−··":   "D",
        "·":     "E",
        "··−·":  "F",
        "−−·":   "G",
        "····":  "H",
        "··":    "I",
        "·−−−":  "J",
        "−·−":   "K",
        "·−··":  "L",
        "−−":    "M",
        "−·":    "N",
        "−−−":   "O",
        "·−−·":  "P",
        "−−·−":  "Q",
        "·−·":   "R",
        "···":   "S",
        "−":     "T",
        "··−":   "U",
        "···−":  "V",
        "·−−":   "W",
        "−··−":  "X",
        "−·−−":  "Y",
        "−−··":  "Z",

        // Numbers
        "·−−−−": "1",
        "··−−−": "2",
        "···−−": "3",
        "····−": "4",
        "·····": "5",
        "−····": "6",
        "−−···": "7",
        "−−−··": "8",
        "−−−−·": "9",
        "−−−−−": "0",

        // Punctuation
        "·−·−·−": ".",
        "−−··−−": ",",
        "··−−··":  "?",
        "−·−·−−": "!",
        "−··−·":  "/",
        "−−·−−":  "(",
        "−−·−−·": ")",
        "·−··−·": "&",
        "−−−···": ":",
        "−·−·−·": ";",
        "−···−":  "=",
        "·−·−·":  "+",
        "−····−": "-",
        "··−−·−": "_",
        "·−···":  "\"",
        "···−··−": "'",
        "·−−−−·": "$",
        "·−·−−·": "@",
    }

    if letter, exists := morseToLetter[morse]; exists {
        return letter
    }
    return ""
}