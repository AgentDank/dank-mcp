// Copyright 2025 Neomantra Corp

package ct

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////

// IsTraceMeasurement returns true if the string is considered a trace measure
// Examples are: "TRC" "<LOQ" and "<0.1"
func IsTraceMeasurement(str string) bool {
	if str == "TRC" || strings.Contains(str, "LOQ") ||
		strings.HasPrefix(str, "<") {
		return true
	}
	return false
}

// IsEmptyMeasurement returns true if the string is considered a trace measure
// Examples are: "" and "<0.1"
func IsEmptyMeasurement(str string) bool {
	if str == "" || str == "." || str == "-" ||
		strings.HasPrefix(str, "--") {
		return true
	}
	return false
}

// IsErrorMeasurement returns true if the string is considered to be erroneous
// Examples are:
func IsErrorMeasurement(str string) bool {
	// One entry had two decimal points (1.1.)... We skip those
	if strings.Count(str, ".") > 1 {
		return true
	}

	// Another error is commas and weird quotes
	if strings.ContainsAny(str, ",`/()") {
		return true
	}

	// Other specific ones have letters in the beginning
	if isLetter(str[0]) {
		return true
	}
	if str == "0<0.10" || strings.HasPrefix(str, "terpinolene: 1.22") || strings.HasPrefix(str, "a-Ocimene: 1.08") {
		return true
	}

	// All good
	return false
}

// isLetter returns true if c is a letter (a-z, A-Z)
func isLetter(c byte) bool {
	r := rune(c)
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

///////////////////////////////////////////////////////////////////////////////

var (
	measureEmptySentinel = 0.0          // Sentinel value for Empty, which is nil-value
	measureZeroSentinel  = math.NaN()   // Sentinel value for Zero, must use IsNan because NaN != NaN
	measureTraceSentinel = math.Inf(-1) // Sentinel value for Trace
	measureErrorSentinel = math.Inf(-1) // Sentinel value for Error
)

func measureSentinelize(amount float64) float64 {
	if amount == 0 {
		return measureZeroSentinel
	} else if amount < 0 {
		return measureTraceSentinel
	}
	return amount
}

///////////////////////////////////////////////////////////////////////////////

// Measure tracks a a measurement, with special flags for no-measurment and trace measurement
type Measure struct {
	amount float64 // amount is the amount of the measure, or sentinal values
}

// NewMeasure creates a new measure with the given amount.
// Any amount < 0, will be treated as a trace measurement.
// To create an "empty" Measure object, use nil-initialization Measure{} or NewEmptyMeasure
func NewMeasure(amount float64) Measure {
	return Measure{amount: measureSentinelize(amount)}
}

// NewEmptyMeasure creates a new "empty" measure.
// This may also be created through nil-initialization Measure{}
func NewEmptyMeasure() Measure {
	return Measure{amount: measureEmptySentinel}
}

// NewTraceMeasure creates a new "trace amount" measure.
func NewTraceMeasure() Measure {
	return Measure{amount: measureTraceSentinel}
}

// NewErrorMeasure creates a new "error" measure.
func NewErrorMeasure() Measure {
	return Measure{amount: measureTraceSentinel}
}

// IsEmpty returns true if the measure is empty (no measurement)
// Same as IsNil
func (m Measure) IsEmpty() bool {
	return m.amount == measureEmptySentinel
}

// IsNil returns true if the measure is empty (no measurement)
// Same as IsEmpty
func (m Measure) IsNil() bool {
	return m.amount == measureEmptySentinel
}

// IsTrace returns true if the measure is a trace amount
func (m Measure) IsTrace() bool {
	// trace is encoded as Inf
	return math.IsInf(m.amount, -1)
}

// IsEmpty returns true if the measure has an initialized, but zero value
func (m Measure) IsZero() bool {
	// zero is encoded as NaN
	return math.IsNaN(m.amount)
}

// Amount returns the value, isTrace, isEmpty
// If it is a trace amount, result will be 0 and isTrace will be true
// If it is an empty value, result will be 0 and isEmpty will be true
func (m Measure) Amount() (result float64, trace bool, empty bool) {
	if m.IsEmpty() {
		return 0, false, true
	} else if m.IsZero() {
		return 0, false, false
	} else if m.IsTrace() {
		return 0, true, false
	} else {
		return m.amount, false, false
	}
}

func (m Measure) IsEqual(other Measure) bool {
	return m.amount == other.amount
}

// IsValidPercent returns true if the measure is a valid percentage (0-100)
func (m Measure) IsValidPercent() bool {
	if m.IsZero() || m.IsEmpty() || m.IsTrace() {
		return true // these are valid values
	}
	// check if the amount is a valid percentage
	return m.amount >= 0 && m.amount <= 100
}

///////////////////////////////////////////////////////////////////////////////

// FromString modifies the given measure based on the passed string
// It undergoes some cleaning and Trace and Empty detection
func (m *Measure) FromString(str string) error {
	// check for a specific non-value records
	if IsEmptyMeasurement(str) {
		m.amount = measureEmptySentinel
		return nil
	}
	if IsErrorMeasurement(str) {
		m.amount = measureEmptySentinel
		return nil
	}
	if IsTraceMeasurement(str) {
		m.amount = measureTraceSentinel
		return nil
	}

	// There's a weird case where some start with ","... we just strip that
	str = strings.TrimPrefix(str, ",")

	// For cases with ">", we just strip that
	str = strings.TrimPrefix(str, ">")

	// Remove percentages, it is always percentages
	str = strings.TrimSuffix(str, "%")

	// convert to float
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}

	// set value
	m.amount = measureSentinelize(val)
	return nil
}

// AsSQL converts the measure to "NULL" or "<amount>".
// Trace is converted to NULL.
func (m Measure) AsSQL() string {
	if m.IsEmpty() || m.IsTrace() {
		return "NULL"
	}
	if m.IsZero() {
		return "0"
	}
	return fmt.Sprintf("%f", m.amount)
}

// AsCSV converts the measure to "" or "<amount>".
// Trace is converted to "".
func (m Measure) AsCSV() string {
	if m.IsEmpty() || m.IsTrace() {
		return ""
	}
	if m.IsZero() {
		return "0"
	}
	return fmt.Sprintf("%f", m.amount)
}

///////////////////////////////////////////////////////////////////////////////
// Marshalling

// MarshalJSON converts the measure to JSON, which is a string
func (m *Measure) MarshalJSON() ([]byte, error) {
	if m.IsEmpty() {
		return []byte{}, nil
	} else if m.IsZero() {
		return []byte("0"), nil
	} else if m.IsTrace() {
		return []byte("<0.01"), nil
	} else {
		return []byte(fmt.Sprintf("%f", m.amount)), nil
	}
}

// MarshalJSON converts the measure from JSON, which can be string or number
func (m *Measure) UnmarshalJSON(b []byte) error {
	// Handle JSON null value
	if bytes.Equal(b, []byte("null")) {
		m.amount = measureEmptySentinel
		return nil
	}

	// try to unmarshal as a string
	var str string
	var err error
	if err = json.Unmarshal(b, &str); err == nil {
		return m.FromString(str)
	}

	// now try to unmarshal as a number
	var val float64
	if err := json.Unmarshal(b, &val); err == nil {
		m.amount = measureSentinelize(val)
		return nil
	}
	return fmt.Errorf("failed to unmarshal measure: %w", err)
}

// Value implements the driver.Valuer interface for inserting into SQL
func (m Measure) Value() (driver.Value, error) {
	if m.IsTrace() || m.IsEmpty() {
		return nil, nil // Trace and Empty values are nil
	}

	if m.IsZero() {
		return 0, nil
	}
	return m.amount, nil
}

// UnmarshalCSV unmarshals the measure from a CSV string
func (m *Measure) UnmarshalCSV(value string) error {
	// Handle CSV empty string
	if value == "" {
		m.amount = measureEmptySentinel
	}
	return m.FromString(value)
}

// MarshalCSV marshals the measure to a CSV string
func (m Measure) MarshalCSV() (string, error) {
	return m.AsCSV(), nil
}
