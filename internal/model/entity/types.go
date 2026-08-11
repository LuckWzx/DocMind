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

// FlexString 兼容 JSON string 与 number 的字符串类型。
// 用途：请求 DTO 中可能收到数字或字符串形态的字段（如前端 agent_id 可能传数字主键 1 或 id_str），
// 反序列化时自动归一化为字符串；序列化时统一输出字符串。
type FlexString string

// UnmarshalJSON 兼容 string / number / null 三种形态
func (s *FlexString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*s = ""
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = FlexString(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err == nil {
		*s = FlexString(num.String())
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into FlexString", string(data))
}

// MarshalJSON 统一输出字符串形态
func (s FlexString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func jsonStructValue(value interface{}) (driver.Value, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}
