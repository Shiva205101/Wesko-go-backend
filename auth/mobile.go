package auth

import (
	"strings"
	"unicode"
)

func NormalizeIndianMobile(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", ErrInvalidMobile
	}

	var digits strings.Builder
	for _, r := range trimmed {
		switch {
		case r == '+':
			if digits.Len() > 0 {
				return "", ErrInvalidMobileFormat
			}
		case unicode.IsDigit(r):
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return "", ErrInvalidMobileFormat
		}
	}

	number := digits.String()
	switch {
	case len(number) == 10:
	case len(number) == 11 && strings.HasPrefix(number, "0"):
		number = number[1:]
	case len(number) == 12 && strings.HasPrefix(number, "91"):
		number = number[2:]
	default:
		return "", ErrInvalidMobileFormat
	}

	if len(number) != 10 || number[0] < '6' || number[0] > '9' {
		return "", ErrInvalidMobileFormat
	}

	return "+91" + number, nil
}

func normalizeEmail(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func normalizeUsername(input string) string {
	return strings.TrimSpace(input)
}
