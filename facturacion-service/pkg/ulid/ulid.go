package ulid

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	entropyPool = sync.Pool{
		New: func() any {
			return ulid.Monotonic(rand.Reader, 0)
		},
	}
)

// ID is a ULID wrapper that implements database and JSON interfaces.
type ID string

// New generates a new ULID using monotonic entropy.
func New() ID {
	entropy := entropyPool.Get().(io.Reader)
	defer entropyPool.Put(entropy)

	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return ID(id.String())
}

// Parse validates and returns a ULID from its string representation.
func Parse(s string) (ID, error) {
	parsed, err := ulid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid ULID %q: %w", s, err)
	}
	return ID(parsed.String()), nil
}

// IsZero returns true if the ID is empty.
func (id ID) IsZero() bool {
	return id == ""
}

// String returns the string representation.
func (id ID) String() string {
	return string(id)
}

// Time extracts the timestamp from the ULID.
func (id ID) Time() time.Time {
	parsed, err := ulid.Parse(string(id))
	if err != nil {
		return time.Time{}
	}
	return ulid.Time(parsed.Time())
}

// MarshalJSON implements json.Marshaler.
func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(id))
}

// UnmarshalJSON implements json.Unmarshaler.
func (id *ID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*id = ""
		return nil
	}
	parsed, err := Parse(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// Value implements driver.Valuer for database writes.
func (id ID) Value() (driver.Value, error) {
	if id.IsZero() {
		return nil, nil
	}
	return string(id), nil
}

// Scan implements sql.Scanner for database reads.
func (id *ID) Scan(src any) error {
	if src == nil {
		*id = ""
		return nil
	}
	switch v := src.(type) {
	case string:
		*id = ID(v)
	case []byte:
		*id = ID(v)
	default:
		return fmt.Errorf("cannot scan %T into ULID", src)
	}
	return nil
}
