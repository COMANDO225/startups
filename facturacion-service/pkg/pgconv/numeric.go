package pgconv

import "github.com/jackc/pgx/v5/pgtype"

// Los importes viajan como string en todo el servicio: float64 pierde centavos
// y SUNAT valida los montos al centimo.
func NumericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0.00"
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return "0.00"
	}
	s, _ := v.(string)
	return s
}

func StringToNumeric(s string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if s == "" {
		s = "0.00"
	}
	err := n.Scan(s)
	return n, err
}
