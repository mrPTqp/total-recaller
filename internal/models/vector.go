package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// Vector represents a PostgreSQL vector type
type Vector []float32

// Value implements driver.Valuer interface for database storage
func (v Vector) Value() (driver.Value, error) {
	if len(v) == 0 {
		return nil, nil
	}

	// Convert to PostgreSQL vector format: "[1.0,2.0,3.0]"
	values := make([]string, len(v))
	for i, val := range v {
		values[i] = fmt.Sprintf("%f", val)
	}

	return fmt.Sprintf("[%s]", strings.Join(values, ",")), nil
}

// Scan implements sql.Scanner interface for database retrieval
func (v *Vector) Scan(value any) error {
	if value == nil {
		*v = nil
		return nil
	}

	var str string
	switch val := value.(type) {
	case string:
		str = val
	case []byte:
		str = string(val)
	default:
		return fmt.Errorf("cannot scan %T into Vector", value)
	}

	if str == "" || str == "[]" {
		*v = nil
		return nil
	}

	// Remove brackets and parse values
	str = strings.Trim(str, "[]")
	if str == "" {
		*v = nil
		return nil
	}

	parts := strings.Split(str, ",")
	result := make([]float32, len(parts))

	for i, part := range parts {
		var val float64
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%f", &val); err != nil {
			return fmt.Errorf("failed to parse vector value: %v", err)
		}
		result[i] = float32(val)
	}

	*v = result
	return nil
}

func (v Vector) MarshalJSON() ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	return json.Marshal([]float32(v))
}

func (v *Vector) UnmarshalJSON(data []byte) error {
	var values []float32
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	*v = Vector(values)
	return nil
}

// FromFloat32Slice creates Vector from []float32
func FromFloat32Slice(values []float32) Vector {
	if values == nil {
		return nil
	}
	result := make([]float32, len(values))
	copy(result, values)
	return Vector(result)
}
