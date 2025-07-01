// File: /models/types.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringSliceType is a custom type for handling JSON arrays of strings in database
type StringSliceType []string

// Value implements driver.Valuer interface for database storage
func (ss StringSliceType) Value() (driver.Value, error) {
	if ss == nil {
		return nil, nil
	}
	return json.Marshal(ss)
}

// Scan implements sql.Scanner interface for database retrieval
func (ss *StringSliceType) Scan(value interface{}) error {
	if value == nil {
		*ss = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, ss)
	case string:
		return json.Unmarshal([]byte(v), ss)
	default:
		return fmt.Errorf("cannot scan %T into StringSliceType", value)
	}
}

// GormDataType returns the data type for GORM
func (StringSliceType) GormDataType() string {
	return "json"
}

// MarshalJSON implements json.Marshaler interface
func (ss StringSliceType) MarshalJSON() ([]byte, error) {
	if ss == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(ss))
}

// UnmarshalJSON implements json.Unmarshaler interface
func (ss *StringSliceType) UnmarshalJSON(data []byte) error {
	var slice []string
	if err := json.Unmarshal(data, &slice); err != nil {
		return err
	}
	*ss = StringSliceType(slice)
	return nil
}
