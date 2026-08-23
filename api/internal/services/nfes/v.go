package nfes

import (
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		slog.Warn("NF-e decimal parse failed", "err", err)
	}
	return v
}

func q2(v decimal.Decimal) string { return v.StringFixed(2) }

func q4(v decimal.Decimal) string { return v.StringFixed(4) }

func dn(s string, n int32) decimal.Decimal {
	parts := strings.Split(s, ".")
	if len(parts) == 2 && len(parts[1]) <= int(n) {
		return d(s)
	}
	dec, err := decimal.NewFromString(s)
	if err != nil {
		slog.Warn("NF-e rounded decimal parse failed", "err", err)
	}
	return dec.Round(n)
}
