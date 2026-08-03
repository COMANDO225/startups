package pgconv

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TextToString converts pgtype.Text to string.
func TextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// StringToText converts string to pgtype.Text.
func StringToText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// NullableStringToText converts *string to pgtype.Text.
func NullableStringToText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// TextToNullableString converts pgtype.Text to *string.
func TextToNullableString(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// TimestamptzToTime converts pgtype.Timestamptz to time.Time.
func TimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// TimeToTimestamptz converts time.Time to pgtype.Timestamptz.
func TimeToTimestamptz(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// TimestamptzToNullableTime converts pgtype.Timestamptz to *time.Time.
func TimestamptzToNullableTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// NullableTimeToTimestamptz converts *time.Time to pgtype.Timestamptz.
func NullableTimeToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// Int4ToInt converts pgtype.Int4 to int32.
func Int4ToInt(i pgtype.Int4) int32 {
	if !i.Valid {
		return 0
	}
	return i.Int32
}

// IntToInt4 converts int32 to pgtype.Int4.
func IntToInt4(i int32) pgtype.Int4 {
	return pgtype.Int4{Int32: i, Valid: true}
}

// Int8ToInt64 converts pgtype.Int8 to int64.
func Int8ToInt64(i pgtype.Int8) int64 {
	if !i.Valid {
		return 0
	}
	return i.Int64
}

// Int64ToInt8 converts int64 to pgtype.Int8.
func Int64ToInt8(i int64) pgtype.Int8 {
	return pgtype.Int8{Int64: i, Valid: true}
}

// BoolToBool converts pgtype.Bool to bool.
func BoolToBool(b pgtype.Bool) bool {
	if !b.Valid {
		return false
	}
	return b.Bool
}

// BoolToPgBool converts bool to pgtype.Bool.
func BoolToPgBool(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}

// NotFoundOr returns true if the error is a "no rows" sentinel.
// Use this to distinguish "not found" from real DB errors in SQLC queries.
func NotFoundOr(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}
