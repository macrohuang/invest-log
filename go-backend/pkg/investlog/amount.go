package investlog

import (
	"database/sql/driver"
	"strconv"

	"github.com/shopspring/decimal"
)

// Amount wraps decimal.Decimal for monetary values.
// JSON marshaling outputs a float64 number (compatible with frontend),
// while internal arithmetic uses precise decimal operations.
type Amount struct {
	decimal.Decimal
}

// MarshalJSON outputs as a JSON number (not a string).
func (a Amount) MarshalJSON() ([]byte, error) {
	f, _ := a.Round(4).Float64()
	return []byte(strconv.FormatFloat(f, 'f', -1, 64)), nil
}

// UnmarshalJSON accepts both JSON numbers and quoted strings.
func (a *Amount) UnmarshalJSON(data []byte) error {
	return a.Decimal.UnmarshalJSON(data)
}

// Scan implements sql.Scanner, reading float64 from SQLite REAL columns.
func (a *Amount) Scan(src any) error {
	if src == nil {
		a.Decimal = decimal.Zero
		return nil
	}
	switch v := src.(type) {
	case float64:
		a.Decimal = decimal.NewFromFloat(v)
		return nil
	case int64:
		a.Decimal = decimal.NewFromInt(v)
		return nil
	case string:
		d, err := decimal.NewFromString(v)
		if err != nil {
			return err
		}
		a.Decimal = d
		return nil
	}
	return a.Decimal.Scan(src)
}

// Value implements driver.Valuer for database writes.
func (a Amount) Value() (driver.Value, error) {
	f, _ := a.Round(4).Float64()
	return f, nil
}

// NewAmount creates an Amount from a float64.
func NewAmount(f float64) Amount {
	return Amount{decimal.NewFromFloat(f)}
}

// NewAmountFromInt creates an Amount from an int64.
func NewAmountFromInt(i int64) Amount {
	return Amount{decimal.NewFromInt(i)}
}

// amountPtr returns a pointer to an Amount.
func amountPtr(v Amount) *Amount {
	return &v
}

// scanNullAmount scans a nullable float64 from SQL into a *Amount.
// Returns nil if src is nil/NULL.
func scanNullFloat(src any) (*Amount, error) {
	if src == nil {
		return nil, nil
	}
	var a Amount
	if err := a.Scan(src); err != nil {
		return nil, err
	}
	return &a, nil
}
