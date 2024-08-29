package ast

import (
	"fmt"
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
	SourceNode
}

func (v Value) GoString() string {
	return fmt.Sprintf("value(%s:%s)", v.token.Type, v.token.Lit)
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
