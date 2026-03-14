package validation

import (
	"errors"
	"strings"
)

var ErrValidationFailed = errors.New("validation failed")

type Errors map[string]string

func (e Errors) Error() string {
	return ErrValidationFailed.Error()
}

func (e Errors) Is(target error) bool {
	return target == ErrValidationFailed
}

func (e Errors) Messages() map[string]string {
	out := make(map[string]string, len(e))
	for field, message := range e {
		out[field] = message
	}

	return out
}

func (e Errors) add(field string, message string) {
	if field == "" || message == "" {
		return
	}

	e[field] = message
}

func messageForTag(tag string, param string) string {
	switch tag {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		return "must be at least " + param + " characters long"
	case "max":
		return "must be at most " + param + " characters long"
	case "len":
		return "must be exactly " + param + " characters long"
	case "numeric":
		return "must contain only digits"
	default:
		if tag == "" {
			return "is invalid"
		}

		return "failed " + strings.ReplaceAll(tag, "_", " ") + " validation"
	}
}
