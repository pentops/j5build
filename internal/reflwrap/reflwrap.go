package reflwrap

import (
	"fmt"
	"math"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/lib/j5reflect"
)

type ContainerField interface {
	SchemaName() string
	//MaybeFindField(path []string) (Field, bool, error)
	RangeProperties(j5reflect.RangeCallback) error
	HasProperty(name string) bool
	Property(name string) (Field, error)
}

type Field interface {
	SchemaName() string
	SetScalar(value j5reflect.ASTValue) error
	//SetScalarAt(path []string, value j5reflect.ASTValue) error
	AsContainer() (ContainerField, error)
}

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

type NotScalarError struct {
	node *reflNode
}

func (nse NotScalarError) Error() string {
	return fmt.Sprintf("cannot set %s.%s as scalar, is %s", nse.node.context, nse.node.path, nse.node.Field.Type())
}

type NotContainerError struct {
	node *reflNode
}

func (npe NotContainerError) Error() string {
	return fmt.Sprintf("cannot set %s.%s as property set, is %s", npe.node.context, npe.node.path, npe.node.Field.Type())
}

type reflNode struct {
	j5reflect.Field
	context string
	path    string
}

func (node *reflNode) SchemaName() string {
	return node.context + "." + node.path
}

func (node *reflNode) SetScalar(value j5reflect.ASTValue) error {
	field, ok := node.Field.(j5reflect.ScalarField)
	if ok {
		err := field.SetASTValue(value)
		if err != nil {
			return err
		}
		return nil
	}

	enum, ok := node.Field.(j5reflect.EnumField)
	if ok {
		val, err := value.AsString()
		if err != nil {
			return err
		}
		err = enum.SetFromString(strcase.ToScreamingSnake(val))
		if err != nil {
			return err
		}
		return nil
	}

	oneof, ok := node.Field.(j5reflect.OneofField)
	if ok {
		oneofVal, err := oneof.Oneof()
		if err != nil {
			return err
		}
		str, err := value.AsString()
		if err != nil {
			return err
		}
		prop, err := oneofVal.GetProperty(str)
		if err != nil {
			return fmt.Errorf("implicitly setting oneof as scalar, but oneof does not have property %s", str)
		}
		if err := prop.Field().SetDefault(); err != nil {
			return err
		}
		return nil
	}

	return &NotScalarError{
		node: node,
	}
}

func (node *reflNode) AsContainer() (ContainerField, error) {
	return node.asContainer()
}

/*
func (node *reflNode) SetScalarAt(path []string, value j5reflect.ASTValue) error {
	cont, err := node.AsContainer()
	if err != nil {
		return err
	}

	field, ok, err := cont.MaybeFindField(path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no field %s in %s", path, node.SchemaName())
	}

	return field.SetScalar(value)
}
*/

func (node *reflNode) asContainer() (*containerNode, error) {
	props, err := node._toPropertySet()
	if err != nil {
		return nil, err
	}
	return &containerNode{
		//node:  node,
		props: props,
	}, nil
}

func (node *reflNode) _toPropertySet() (PropertySet, error) {
	switch ft := node.Field.(type) {
	case j5reflect.ObjectField:
		if err := ft.SetDefault(); err != nil {
			return nil, err
		}
		return ft.Object()

	case j5reflect.OneofField:
		return ft.Oneof()

	case j5reflect.ArrayOfObjectField:
		return ft.NewObjectElement()

	case j5reflect.ArrayOfOneofField:
		return ft.NewOneofElement()

	default:
		return nil, &NotContainerError{
			node: node,
		}
	}
}

type PropertySet interface {
	HasProperty(name string) bool
	GetProperty(name string) (j5reflect.Property, error)
	RangeProperties(j5reflect.RangeCallback) error
	Name() string
}

type containerNode struct {
	//	node  *reflNode
	props PropertySet
}

func NewContainerField(obj j5reflect.PropertySet) ContainerField {
	return &containerNode{
		props: obj,
	}
}

func (cn *containerNode) SchemaName() string {
	return cn.props.Name()
}

func (cn *containerNode) HasProperty(name string) bool {
	return cn.props.HasProperty(name)
}

func (cn *containerNode) Property(name string) (Field, error) {
	field, err := cn._stepPath([]string{name})
	if err != nil {
		return nil, err
	}
	if field == nil {
		return nil, fmt.Errorf("no field %s in %s", name, cn.SchemaName())
	}
	return field, nil
}

func (cn *containerNode) RangeProperties(cb j5reflect.RangeCallback) error {
	return cn.props.RangeProperties(cb)
}

/*
// FindNode returns the node or nil if it does not exist, and an error if the
// path is invalid (hits a non-container before the leaf)

	func (cn *containerNode) MaybeFindField(path []string) (Field, bool, error) {
		field, err := cn._stepPath(path)
		if err != nil {
			return nil, false, err
		}
		if field == nil {
			return nil, false, nil
		}
		return field, true, nil
	}
*/
func (cn *containerNode) _stepPath(path []string) (*reflNode, error) {

	prop, err := cn.props.GetProperty(path[0])
	if err != nil {
		return nil, err
	}

	field := prop.Field()

	node := &reflNode{
		Field:   field,
		context: cn.props.Name(),
		path:    path[0],
	}

	if len(path) == 1 {
		return node, nil
	}

	child, err := node.asContainer()
	if err != nil {
		return nil, err
	}
	return child._stepPath(path[1:])
}
