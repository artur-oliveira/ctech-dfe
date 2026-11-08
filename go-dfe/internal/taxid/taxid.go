// Package taxid validates Brazilian CPF and CNPJ (numeric and alphanumeric, IN RFB 2229/2024) by check digits.
// It is the single implementation; validate and ciot delegate to it.
package taxid

import (
	"regexp"
	"strings"
)

var (
	cnpjRe = regexp.MustCompile(`^[A-Z0-9]{12}[0-9]{2}$`)
	cpfRe  = regexp.MustCompile(`^[0-9]{11}$`)
)

// asciiZero is subtracted from a character code to get its CNPJ check-digit value (alphanumeric rule).
const asciiZero = '0'

// CNPJ validates format, all-zero and both check digits (value = ASCII code minus 48).
func CNPJ(s string) bool {
	if !cnpjRe.MatchString(s) || strings.Trim(s, "0") == "" {
		return false
	}
	dv := func(n int, w []int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(s[i]-asciiZero) * w[i]
		}
		if r := sum % 11; r >= 2 {
			return 11 - r
		}
		return 0
	}
	d1 := dv(12, []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	d2 := dv(13, []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2})
	return int(s[12]-asciiZero) == d1 && int(s[13]-asciiZero) == d2
}

// CPF validates 11 digits, not all equal, and both check digits.
func CPF(s string) bool {
	if !cpfRe.MatchString(s) || strings.Count(s, s[:1]) == len(s) {
		return false
	}
	dv := func(n int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(s[i]-asciiZero) * (n + 1 - i)
		}
		if r := (sum * 10) % 11; r < 10 {
			return r
		}
		return 0
	}
	return int(s[9]-asciiZero) == dv(9) && int(s[10]-asciiZero) == dv(10)
}

// Doc validates a CPF (11 characters) or CNPJ (14) by length.
func Doc(s string) bool {
	if len(s) == 11 {
		return CPF(s)
	}
	return CNPJ(s)
}
