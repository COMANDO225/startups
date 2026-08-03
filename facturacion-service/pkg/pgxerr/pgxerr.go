package pgxerr

import (
	"errors"

	domainerr "facturacion-service/internal/shared/domain/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Códigos de error de PostgreSQL relevantes.
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	UniqueViolation     = "23505"
	ForeignKeyViolation = "23503"
	CheckViolation      = "23514"
	ExclusionViolation  = "23P01"
)

// MapError convierte errores de pgx a errores de dominio.
//
// Orden de resolución:
//  1. nil → nil
//  2. pgx.ErrNoRows → notFound
//  3. Unique violation (23505) → Conflict con constraint name
//  4. FK violation (23503) → Validation con constraint name
//  5. Exclusion violation (23P01) → Conflict con constraint name
//  6. Check violation (23514) → Validation con constraint name
//  7. Otro error → Internal wrapeado
func MapError(err error, notFound *domainerr.Error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return mapPgError(pgErr)
	}

	return domainerr.Internal("error de base de datos").Wrap(err)
}

// MapWriteError convierte errores de escritura (INSERT/UPDATE/DELETE).
// No necesita notFound — las escrituras no retornan ErrNoRows.
func MapWriteError(err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return mapPgError(pgErr)
	}

	return domainerr.Internal("error de base de datos").Wrap(err)
}

func mapPgError(pgErr *pgconn.PgError) *domainerr.Error {
	switch pgErr.Code {
	case UniqueViolation:
		return domainerr.Conflict("El registro ya existe").
			WithCode("DUPLICATE_ENTRY").
			WithMeta("constraint", pgErr.ConstraintName)

	case ForeignKeyViolation:
		return domainerr.Validation("Referencia inválida: el recurso relacionado no existe").
			WithCode("INVALID_REFERENCE").
			WithMeta("constraint", pgErr.ConstraintName)

	case ExclusionViolation:
		return domainerr.Conflict("El recurso solicitado genera un conflicto").
			WithCode("EXCLUSION_CONFLICT").
			WithMeta("constraint", pgErr.ConstraintName)

	case CheckViolation:
		return domainerr.Validation("Los datos no cumplen las reglas de validación").
			WithCode("CHECK_VIOLATION").
			WithMeta("constraint", pgErr.ConstraintName)

	default:
		return domainerr.Internal("error de base de datos").Wrap(pgErr)
	}
}

// IsUniqueViolation verifica si el error es una violación de unicidad.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == UniqueViolation
}

// ConstraintName extrae el nombre del constraint del error, si existe.
func ConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}
