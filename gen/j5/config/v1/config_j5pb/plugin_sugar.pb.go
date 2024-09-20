// Code generated by protoc-gen-go-sugar. DO NOT EDIT.

package config_j5pb

import (
	driver "database/sql/driver"
	fmt "fmt"
)

type IsDockerRegistryAuth_Auth = isDockerRegistryAuth_Auth

// Plugin
const (
	Plugin_UNSPECIFIED Plugin = 0
	Plugin_PROTO       Plugin = 1
	Plugin_J5_CLIENT   Plugin = 2
)

var (
	Plugin_name_short = map[int32]string{
		0: "UNSPECIFIED",
		1: "PROTO",
		2: "J5_CLIENT",
	}
	Plugin_value_short = map[string]int32{
		"UNSPECIFIED": 0,
		"PROTO":       1,
		"J5_CLIENT":   2,
	}
	Plugin_value_either = map[string]int32{
		"UNSPECIFIED":        0,
		"PLUGIN_UNSPECIFIED": 0,
		"PROTO":              1,
		"PLUGIN_PROTO":       1,
		"J5_CLIENT":          2,
		"PLUGIN_J5_CLIENT":   2,
	}
)

// ShortString returns the un-prefixed string representation of the enum value
func (x Plugin) ShortString() string {
	return Plugin_name_short[int32(x)]
}
func (x Plugin) Value() (driver.Value, error) {
	return []uint8(x.ShortString()), nil
}
func (x *Plugin) Scan(value interface{}) error {
	var strVal string
	switch vt := value.(type) {
	case []uint8:
		strVal = string(vt)
	case string:
		strVal = vt
	default:
		return fmt.Errorf("invalid type %T", value)
	}
	val := Plugin_value_either[strVal]
	*x = Plugin(val)
	return nil
}
