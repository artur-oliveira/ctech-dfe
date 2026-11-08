package ciot

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	plateRe   = regexp.MustCompile(`^[A-Z]{3}[0-9]{4}$`)
	mercosul  = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z][0-9]{2}$`)
	nonDigits = regexp.MustCompile(`[^0-9]`)
)

const rntrcLen = 9

// NormalizeRNTRC keeps digits and left-pads 8-digit values with a zero (B60); other lengths are returned as is.
func NormalizeRNTRC(s string) string {
	d := nonDigits.ReplaceAllString(s, "")
	if len(d) == rntrcLen-1 {
		return "0" + d
	}
	return d
}

// NormalizePlate upper-cases and strips separators.
func NormalizePlate(s string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(s))
}

// ValidPlate accepts AAA9999 and Mercosul AAA9A99.
func ValidPlate(s string) bool { return plateRe.MatchString(s) || mercosul.MatchString(s) }

// NormalizeDoc strips punctuation from a CPF or CNPJ, keeping alphanumeric CNPJ characters upper-cased.
func NormalizeDoc(s string) string {
	return strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(s))
}

// FormatDate renders yyyy-MM-dd in Brasilia time.
func FormatDate(t time.Time) string { return t.In(Brasilia).Format(DateLayout) }

// FormatDateTime renders yyyy-MM-ddTHH:mm:ss without zone, in Brasilia time.
func FormatDateTime(t time.Time) string { return t.In(Brasilia).Format(DateTimeLayout) }

// ParseDate parses yyyy-MM-dd as midnight Brasilia.
func ParseDate(s string) (time.Time, error) { return time.ParseInLocation(DateLayout, s, Brasilia) }

// ParseDateTime parses yyyy-MM-ddTHH:mm:ss as Brasilia time.
func ParseDateTime(s string) (time.Time, error) {
	return time.ParseInLocation(DateTimeLayout, s, Brasilia)
}

// FormatMoney renders integer cents as text with a dot and 2 decimals (money is never a float).
func FormatMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// ParseMoney parses "123.45" (or "123.4", "123") into cents; more than 2 decimals is an error.
func ParseMoney(s string) (int64, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	if len(frac) > 2 || whole == "" && frac == "" {
		return 0, fmt.Errorf("ciot: invalid money %q", s)
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ciot: invalid money %q", s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("ciot: invalid money %q", s)
	}
	if strings.HasPrefix(strings.TrimSpace(s), "-") {
		return w*100 - f, nil
	}
	return w*100 + f, nil
}
