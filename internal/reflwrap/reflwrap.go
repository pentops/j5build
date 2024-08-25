package reflwrap

import (
	"fmt"
	"math"

	"github.com/pentops/j5/lib/j5reflect"
)

type unknownValue struct {
	typeName string
}

func (uv unknownValue) AsString() (string, error) {
	return "", fmt.Errorf("value is %s is not a string", uv.typeName)
}

func (uv unknownValue) AsBoolean() (bool, error) {
	return false, fmt.Errorf("value is %s is not a boolean", uv.typeName)
}

func (uv unknownValue) AsUint(size int) (uint64, error) {
	return 0, fmt.Errorf("value is %s is not a uint%d", uv.typeName, size)
}

func (uv unknownValue) AsInt(size int) (int64, error) {
	return 0, fmt.Errorf("value is %s is not a int%d", uv.typeName, size)
}

func (uv unknownValue) AsFloat(size int) (float64, error) {
	return 0, fmt.Errorf("value is %s is not a float%d", uv.typeName, size)
}

type IntValue struct {
	unknownValue
	val int64
}

func NewIntValue(val int64) j5reflect.ASTValue {
	return IntValue{
		unknownValue: unknownValue{typeName: "int"},
		val:          val,
	}
}
func (iv IntValue) AsInt(size int) (int64, error) {
	if size == 32 {
		if iv.val > math.MaxInt32 || iv.val < math.MinInt32 {
			return 0, fmt.Errorf("int32 overflow")
		}
	}
	return iv.val, nil
}
