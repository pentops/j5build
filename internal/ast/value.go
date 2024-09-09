package ast

import (
	"fmt"
	"math"
	"strconv"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

type ASTValue interface {
	AsBool() (bool, error)
	AsString() (string, error)
	AsInt(bits int) (int64, error)
	AsUint(bits int) (uint64, error)
	AsFloat(bits int) (float64, error)

	Position() errpos.Position

	IsArray() bool
	IsScalar() bool
	AsArray() ([]ASTValue, bool)
}

type unknownValue struct {
	typeName string
}

func (uv unknownValue) AsString() (string, error) {
	return "", fmt.Errorf("value is %s is not a string", uv.typeName)
}

func (uv unknownValue) AsBool() (bool, error) {
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

func (uv unknownValue) IsArray() bool {
	return false
}

func (uv unknownValue) IsScalar() bool {
	return false
}

func (uv unknownValue) AsArray() ([]ASTValue, bool) {
	return nil, false
}

/*

type IntValue struct {
	unknownValue
	val int64
}

func NewIntValue(val int64) ASTValue {
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
}*/

type Value struct {
	token lexer.Token
	array []Value
	SourceNode
}

var _ ASTValue = Value{}

func (v Value) GoString() string {
	if v.IsArray() {
		return fmt.Sprintf("[%#v]", v.array)
	}
	return fmt.Sprintf("value(%s:%s)", v.token.Type, v.token.Lit)
}

func (v Value) IsArray() bool {
	return len(v.array) > 0
}

func (v Value) IsScalar() bool {
	return !v.IsArray()
}

func (v Value) AsArray() ([]ASTValue, bool) {
	if !v.IsArray() {
		return nil, false
	}
	out := make([]ASTValue, len(v.array))
	for idx, val := range v.array {
		out[idx] = val
	}
	return out, true
}

func (v Value) Position() errpos.Position {
	return v.SourceNode.Position()
}

func (v Value) AsString() (string, error) {
	if v.token.Type != lexer.STRING &&
		v.token.Type != lexer.DESCRIPTION &&
		v.token.Type != lexer.IDENT &&
		v.token.Type != lexer.REGEX {

		return "", &TypeError{
			Expected: "string",
			Got:      v.token.String(),
		}
	}
	return v.token.Lit, nil
}

func (v Value) AsBool() (bool, error) {
	if v.token.Type != lexer.BOOL {
		return false, &TypeError{
			Expected: "bool",
			Got:      v.token.String(),
		}
	}
	return v.token.Lit == "true", nil
}

func (v Value) AsUint(size int) (uint64, error) {
	if v.token.Type != lexer.INT {
		return 0, &TypeError{
			Expected: fmt.Sprintf("uint%d", size),
			Got:      v.token.String(),
		}
	}
	parsed, err := strconv.ParseUint(v.token.Lit, 10, size)
	return parsed, err

}

func (v Value) AsInt(size int) (int64, error) {
	if v.token.Type != lexer.INT {
		return 0, &TypeError{
			Expected: fmt.Sprintf("int%d", size),
			Got:      v.token.String(),
		}
	}
	parsed, err := strconv.ParseInt(v.token.Lit, 10, size)
	return parsed, err
}

func (v Value) AsFloat(size int) (float64, error) {
	switch v.token.Type {
	case lexer.INT:
		parsed, err := strconv.ParseFloat(v.token.Lit, size)
		return parsed, err
	case lexer.DECIMAL:
		parsed, err := strconv.ParseFloat(v.token.Lit, size)
		return parsed, err
	default:
		return 0, &TypeError{
			Expected: fmt.Sprintf("float%d", size),
			Got:      v.token.String(),
		}
	}
}

type IntValue struct {
	unknownValue
	val int64
	SourceNode
}

func NewIntValue(val int64, pos Position) ASTValue {
	return IntValue{
		unknownValue: unknownValue{typeName: "int"},
		val:          val,
	}
}

func (iv IntValue) IsScalar() bool {
	return true
}

func (iv IntValue) AsInt(size int) (int64, error) {
	if size == 32 {
		if iv.val > math.MaxInt32 || iv.val < math.MinInt32 {
			return 0, fmt.Errorf("int32 overflow")
		}
	}
	return iv.val, nil
}

type StringValue struct {
	unknownValue
	val string
	SourceNode
}

func NewStringValue(val string, src SourceNode) ASTValue {
	return StringValue{
		unknownValue: unknownValue{typeName: "string"},
		val:          val,
		SourceNode:   src,
	}
}

func (sv StringValue) AsString() (string, error) {
	return sv.val, nil
}
func (sv StringValue) IsScalar() bool {
	return true
}

type TagValue struct {
	Bang      bool
	Reference *Reference
	Value     *Value
	unknownValue
	SourceNode
}

var _ ASTValue = TagValue{}

func (tv TagValue) GoString() string {
	return fmt.Sprintf("tag(%s, %#v)", tv.Reference, tv.Value)
}

func (tv TagValue) IsScalar() bool {
	return true
}

func (tv TagValue) AsString() (string, error) {
	if tv.Value != nil {
		return tv.Value.AsString()
	}
	if tv.Reference != nil {
		return tv.Reference.String(), nil
	}
	return "", fmt.Errorf("tag value is nil")
}

type BoolValue struct {
	unknownValue
	val bool
	SourceNode
}

func NewBoolValue(val bool, pos Position) ASTValue {
	return BoolValue{
		unknownValue: unknownValue{typeName: "bool"},
		val:          val,
	}
}

func (iv BoolValue) IsScalar() bool {
	return true
}

func (iv BoolValue) AsBool() (bool, error) {
	return iv.val, nil
}
