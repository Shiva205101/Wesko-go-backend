package validation

import (
	"errors"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	defaultValidator *validator.Validate
	once             sync.Once
)

func engine() *validator.Validate {
	once.Do(func() {
		defaultValidator = validator.New()
		defaultValidator.RegisterTagNameFunc(func(field reflect.StructField) string {
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				return field.Name
			}

			return name
		})
	})

	return defaultValidator
}

func Validate(value any) error {
	if err := engine().Struct(value); err != nil {
		var validationErrs validator.ValidationErrors
		if !errors.As(err, &validationErrs) {
			return err
		}

		result := make(Errors, len(validationErrs))
		for _, fieldErr := range validationErrs {
			result.add(fieldErr.Field(), messageForTag(fieldErr.Tag(), fieldErr.Param()))
		}

		return result
	}

	return nil
}

func IsValidationError(err error) bool {
	var validationErrs Errors
	return errors.As(err, &validationErrs)
}
