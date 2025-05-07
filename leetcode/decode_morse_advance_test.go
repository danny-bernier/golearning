package leetcode

import (
	"testing"
)

func TestSampleRate(t *testing.T) {
	morse := "1100110011001100000011000000111111001100111111001111110000000000000011001111110011111100111111000000110011001111110000001111110011001100000011"
	result := findSampleRate(morse)
	if result != 2 {
		t.Errorf("Expected: %d Actual: %d", 2, result)
	}

	morse = "111000111000111000000000111"
	result = findSampleRate(morse)
	if result != 3 {
		t.Errorf("Expected: %d Actual: %d", 3, result)
	}
}

func TestNormalizeMorse(t *testing.T) {
	morse := "1100110011001100000011000000111111001100111111001111110000000000000011001111110011111100111111000000110011001111110000001111110011001100000011"
	expected := "10101010001000111010111011100000001011101110111000101011100011101010001"

	sampleRate := findSampleRate(morse)
	result := normalizeMorse(morse, sampleRate)

	if result != expected {
		t.Errorf("Expected: %s Actual: %s", expected, result)
	}
}

func TestDecodeBits(t *testing.T) {
	morse := "1100110011001100000011000000111111001100111111001111110000000000000011001111110011111100111111000000110011001111110000001111110011001100000011"
	expected := "···· · −·−−   ·−−− ··− −·· ·"

	result := DecodeBits(morse)

	if result != expected {
		t.Errorf("Expected: %s Actual: %s", expected, result)
	}
}


func TestDecodeMorse(t *testing.T) {
	morse := "1100110011001100000011000000111111001100111111001111110000000000000011001111110011111100111111000000110011001111110000001111110011001100000011"
	expected := "HEY JUDE"

	result := DecodeMorse(DecodeBits(morse))

	if result != expected {
		t.Errorf("Expected: %s Actual: %s", expected, result)
	}
}