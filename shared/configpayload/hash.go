// Package configpayload defines the canonical content hash used by desired
// configuration storage and the HUB control protocol.
package configpayload

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

// Hash returns a stable SHA-256 hex digest for a JSON value. Whitespace and
// object key order do not affect the result.
func Hash(payload []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return "", errors.New("configuration payload contains multiple JSON values")
		}
		return "", err
	}
	value, err := canonicalize(value)
	if err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalize(value any) (any, error) {
	switch typed := value.(type) {
	case json.Number:
		normalized, err := canonicalNumber(string(typed))
		if err != nil {
			return nil, err
		}
		return json.Number(normalized), nil
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			value, err := canonicalize(typed[index])
			if err != nil {
				return nil, err
			}
			result[index] = value
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, nested := range typed {
			value, err := canonicalize(nested)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		return result, nil
	default:
		return value, nil
	}
}

func canonicalNumber(value string) (string, error) {
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = value[1:]
	}
	parts := strings.SplitN(value, "e", 2)
	if len(parts) == 1 {
		parts = strings.SplitN(value, "E", 2)
	}
	exponent := 0
	if len(parts) == 2 {
		parsed, err := strconv.Atoi(parts[1])
		if err != nil || parsed < -1000000 || parsed > 1000000 {
			return "", errors.New("JSON number exponent is out of range")
		}
		exponent = parsed
	}
	decimal := strings.SplitN(parts[0], ".", 2)
	integerDigits := decimal[0]
	fractionDigits := ""
	if len(decimal) == 2 {
		fractionDigits = decimal[1]
	}
	digits := integerDigits + fractionDigits
	leadingZeros := len(digits) - len(strings.TrimLeft(digits, "0"))
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return "0", nil
	}
	scientificExponent := len(integerDigits) + exponent - leadingZeros - 1
	digits = strings.TrimRight(digits, "0")
	mantissa := digits[:1]
	if len(digits) > 1 {
		mantissa += "." + digits[1:]
	}
	sign := ""
	if negative {
		sign = "-"
	}
	return sign + mantissa + "e" + strconv.Itoa(scientificExponent), nil
}
