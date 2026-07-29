package entity

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSON GORM 兼容的 JSON 类型，用于存储灵活 JSON 字段
type JSON json.RawMessage

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSON: value is not []byte")
	}
	*j = append((*j)[0:0], bytes...)
	return nil
}

// Value 实现 driver.Valuer 接口
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

func scanJSONStruct(value interface{}, target interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return json.Unmarshal(v, target)
	case string:
		if v == "" {
			return nil
		}
		return json.Unmarshal([]byte(v), target)
	default:
		return fmt.Errorf("failed to scan JSON struct: unsupported type %T", value)
	}
}

func jsonStructValue(value interface{}) (driver.Value, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}
