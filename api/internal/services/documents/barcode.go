package documents

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"strconv"
	"strings"

	boombarcode "github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
)

const dataURIPNGPrefix = "data:image/png;base64,"

func code128DataURI(value string) (string, error) {
	if value == "" || digits(value) != value {
		return "", fmt.Errorf("CODE-128 value must be a non-empty numeric string")
	}
	code, err := code128.Encode(value)
	if err != nil {
		return "", fmt.Errorf("encode CODE-128: %w", err)
	}
	return imageDataURI(boombarcode.Scale(code, max(600, code.Bounds().Dx()), 80))
}

func qrDataURI(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("QR Code payload is empty")
	}
	code, err := qr.Encode(value, qr.M, qr.Auto)
	if err != nil {
		return "", fmt.Errorf("encode QR Code: %w", err)
	}
	return imageDataURI(boombarcode.Scale(code, 360, 360))
}

func dadosNFeCode(cuf, emissionType, document, totalCents string, hasICMS, hasICMSST bool, emissionDay string) string {
	body := fixedDigits(cuf, 2) + fixedDigits(emissionType, 1) + fixedDigits(document, 14) +
		fixedDigits(totalCents, 14) + presenceDigit(hasICMS) + presenceDigit(hasICMSST) + fixedDigits(emissionDay, 2)
	return body + mod11Digit(body)
}

func fixedDigits(value string, width int) string {
	value = digits(value)
	if len(value) > width {
		return value[len(value)-width:]
	}
	return strings.Repeat("0", width-len(value)) + value
}

func presenceDigit(present bool) string {
	if present {
		return "1"
	}
	return "2"
}

func mod11Digit(value string) string {
	total, weight := 0, 2
	for index := len(value) - 1; index >= 0; index-- {
		total += int(value[index]-'0') * weight
		weight++
		if weight > 9 {
			weight = 2
		}
	}
	digit := 11 - total%11
	if digit >= 10 {
		digit = 0
	}
	return strconv.Itoa(digit)
}

func imageDataURI(image boombarcode.Barcode, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("scale barcode: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image); err != nil {
		return "", fmt.Errorf("encode barcode PNG: %w", err)
	}
	return dataURIPNGPrefix + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
