package validator

import (
	"reflect"
	"strings"

	domainerr "facturacion-service/internal/shared/domain/errors"
	"github.com/go-playground/validator/v10"
)

// Validator envuelve go-playground/validator con formato de error personalizado.
type Validator struct {
	validate *validator.Validate
}

// New crea una nueva instancia de Validator.
func New() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())

	// usar nombres de tags JSON para nombres de campo
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	return &Validator{validate: v}
}

// Details valida un struct y retorna los detalles de error formateados.
func (v *Validator) Details(s any) []domainerr.Detail {
	err := v.validate.Struct(s)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []domainerr.Detail{{
			Code:    "VALIDATION_ERROR",
			Message: err.Error(),
		}}
	}

	details := make([]domainerr.Detail, 0, len(validationErrors))
	for _, e := range validationErrors {
		detail := domainerr.Detail{
			Field:   e.Field(),
			Code:    tagToCode(e.Tag()),
			Message: formatMessage(e),
		}
		// omitir valor de campos sensibles (password, token, etc.)
		if !isSensitiveField(e.Field()) {
			detail.Value = e.Value()
		}
		details = append(details, detail)
	}

	return details
}

// RegisterValidation adds a custom validation function.
func (v *Validator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}

// tagToCode convierte tags del validador a códigos de error.
func tagToCode(tag string) string {
	codes := map[string]string{
		"required": "REQUIRED_FIELD",
		"email":    "INVALID_FORMAT",
		"min":      "MIN_LENGTH",
		"max":      "MAX_LENGTH",
		"len":      "INVALID_LENGTH",
		"eqfield":  "FIELD_MISMATCH",
		"numeric":  "INVALID_FORMAT",
		"alphanum": "INVALID_FORMAT",
		"url":      "INVALID_FORMAT",
		"uuid":     "INVALID_FORMAT",
		"oneof":    "INVALID_VALUE",
		"gte":      "MIN_VALUE",
		"lte":      "MAX_VALUE",
		"e164":     "INVALID_FORMAT",
	}

	if code, ok := codes[tag]; ok {
		return code
	}
	return "INVALID_VALUE"
}

// isSensitiveField verifica si un campo contiene datos sensibles
// que no deben exponerse en respuestas de error.
func isSensitiveField(name string) bool {
	lower := strings.ToLower(name)
	for _, pattern := range []string{"password", "token", "secret", "api_key", "apikey", "credential"} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// formatMessage crea un mensaje de error legible para el usuario.
func formatMessage(e validator.FieldError) string {
	field := e.Field()

	switch e.Tag() {
	case "required":
		return field + " is required"
	case "email":
		return "Invalid email format"
	case "min":
		return field + " must be at least " + e.Param() + " characters"
	case "max":
		return field + " must be at most " + e.Param() + " characters"
	case "len":
		return field + " must be exactly " + e.Param() + " characters"
	case "eqfield":
		return field + " must match " + e.Param()
	case "numeric":
		return field + " must contain only numbers"
	case "alphanum":
		return field + " must contain only letters and numbers"
	case "url":
		return field + " must be a valid URL"
	case "uuid":
		return field + " must be a valid UUID"
	case "oneof":
		return field + " must be one of: " + e.Param()
	case "gte":
		return field + " must be at least " + e.Param()
	case "lte":
		return field + " must be at most " + e.Param()
	case "e164":
		return field + " must be a valid phone number"
	default:
		return field + " is invalid"
	}
}
