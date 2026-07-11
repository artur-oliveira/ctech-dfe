package nfes

import (
	"strings"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal {
	v, _ := decimal.NewFromString(s)
	return v
}

func q2(v decimal.Decimal) string { return v.StringFixed(2) }

func q4(v decimal.Decimal) string { return v.StringFixed(4) }

func dn(s string, n int32) decimal.Decimal {
	parts := strings.Split(s, ".")
	if len(parts) == 2 && len(parts[1]) <= int(n) {
		return d(s)
	}
	dec, _ := decimal.NewFromString(s)
	return dec.Round(n)
}
