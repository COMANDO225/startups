package validator

import (
	domainerr "facturacion-service/internal/shared/domain/errors"
)

// Validate implementa fiber.StructValidator. Al registrarlo en fiber.Config,
// cada c.Bind().Body(&req) valida automaticamente y el error viaja al
// global error handler ya formateado.
func (v *Validator) Validate(out any) error {
	details := v.Details(out)
	if len(details) == 0 {
		return nil
	}
	return domainerr.Validation("Los datos enviados contienen errores").WithDetails(details...)
}
