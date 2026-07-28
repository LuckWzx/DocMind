package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap 通用 JSON 配置载体
type JSONMap map[string]interface{}

// Value 实现 driver.Valuer
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// Scan 实现 sql.Scanner
func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = JSONMap{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*m = JSONMap{}
			return nil
		}
		return json.Unmarshal(v, m)
	case string:
		if v == "" {
			*m = JSONMap{}
			return nil
		}
		return json.Unmarshal([]byte(v), m)
	default:
		return fmt.Errorf("unsupported JSONMap scan type: %T", value)
	}
}

// SystemSetting 系统级配置
type SystemSetting struct {
	BaseEntity
	Key   string  `gorm:"type:varchar(128);uniqueIndex;not null;comment:配置键" json:"key"`
	Value JSONMap `gorm:"type:json;comment:配置值" json:"value"`
}

// TableName 指定表名
func (SystemSetting) TableName() string {
	return "system_settings"
}
