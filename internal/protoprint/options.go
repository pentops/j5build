package protoprint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pentops/prototools/optionreflect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type parsedOption struct {
	fullName string
	txt      string
}

type parsedOptions []parsedOption

func (a parsedOptions) Len() int           { return len(a) }
func (a parsedOptions) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a parsedOptions) Less(i, j int) bool { return a[i].fullName < a[j].fullName }

func optionsFor(desc protoreflect.Descriptor) ([]parsedOption, error) {
	opts := desc.Options()
	refl := opts.ProtoReflect()
	out := make([]parsedOption, 0)
	var outerError error
	refl.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		msg := v.Message()
		txt, err := protojson.Marshal(msg.Interface())
		if err != nil {
			outerError = err
			return false
		}
		option := parsedOption{
			fullName: string(fd.FullName()),
			txt:      string(txt),
		}
		out = append(out, option)
		return true
	})
	sort.Sort(parsedOptions(out))

	if outerError != nil {
		return nil, outerError
	}
	return out, nil
}

func (extInd *fileBuilder) printOption(parsed parsedOption) {
	extInd.p("option NOT DONE ;")
	/*

		typeName := optionTypeName(opt)
		if parsed.inlineString != nil {
			extInd.p("option ", typeName, " = ", *parsed.inlineString, ";")
			return
		}

		switch parsed.root.FieldType {
		case optionreflect.FieldTypeMessage:

			if len(parsed.root.Children) == 0 {
				extInd.p("option ", typeName, " = {};")
			}
			extInd.p("option ", typeName, " = {")
			extInd.printOptionMessageFields(parsed.root.Children)
			extInd.endElem("};")
		case optionreflect.FieldTypeArray:
			opener := fmt.Sprintf("option %s", typeName)
			extInd.printOptionArray(opener, parsed.root.Children, ";")
		case optionreflect.FieldTypeScalar:
			extInd.p("option ", typeName, " = ", parsed.root.ScalarValue, ";")

		}*/

}

func optionTypeName(opt *optionreflect.OptionDefinition) string {

	name, err := contextRefName(opt.Context, opt.RootType)
	if err != nil {
		panic(err.Error())
	}

	if len(opt.SubPath) == 0 {
		return fmt.Sprintf("(%s)", name)
	}

	return fmt.Sprintf("(%s).%s", name, strings.Join(opt.SubPath, "."))
}

func (ind *fileBuilder) printOptionArray(opener string, children []optionreflect.OptionField, trailer string) {
	if len(children) == 0 {
		ind.p(opener, ": []", trailer)
		return
	}
	if len(children) == 1 && children[0].FieldType == optionreflect.FieldTypeScalar {
		ind.p(opener, ": [", children[0].ScalarValue, "]", trailer)
		return
	}
	if children[0].FieldType == optionreflect.FieldTypeMessage {
		ind.p(opener, ": [{")
		for idx, child := range children {
			if idx != 0 {
				ind.p("}, {")
			}
			ind.printOptionMessageFields(child.Children)
		}
		ind.endElem("}]", trailer)
		return
	}
}

func (ind *fileBuilder) printOptionMessageFields(children []optionreflect.OptionField) {
	ind2 := ind.indent()
	for _, child := range children {
		switch child.FieldType {
		case optionreflect.FieldTypeMessage:
			ind2.p(child.Key, ": {")
			ind2.printOptionMessageFields(child.Children)
			ind2.endElem("}")
		case optionreflect.FieldTypeArray:
			ind2.printOptionArray(child.Key, child.Children, "")
		case optionreflect.FieldTypeScalar:
			ind2.p(child.Key, ": ", child.ScalarValue)
		}
	}

}

func (fb *fileBuilder) printFieldStyle(name string, number int32, elem protoreflect.Descriptor) error {

	srcLoc := elem.ParentFile().SourceLocations().ByDescriptor(elem)

	options, err := optionsFor(elem)
	if err != nil {
		return err
	}

	fb.leadingComments(srcLoc)

	if len(options) == 0 {
		fb.p(name, " = ", number, ";", inlineComment(srcLoc))
	} else {
		fb.p(name, " = ", number, " [", inlineComment(srcLoc))
		extInd := fb.indent()
		for idx, parsed := range options {
			trailer := ","
			if idx == len(options)-1 {
				trailer = ""
			}
			extInd.p("(", parsed.fullName, ")", " = ", parsed.txt, trailer)
			/*
				opt := parsed.def
				val := parsed.root

				if parsed.inlineString != nil {
					extInd.p(optionTypeName(opt), " = ", *parsed.inlineString, trailer)
					continue
				}

				switch val.FieldType {
				case optionreflect.FieldTypeMessage:
					extInd.p(optionTypeName(opt), " = {")
					extInd.printOptionMessageFields(parsed.root.Children)
					extInd.endElem("}", trailer)
				case optionreflect.FieldTypeArray:
					extInd.printOptionArray(optionTypeName(opt), parsed.root.Children, trailer)
				case optionreflect.FieldTypeScalar:
					extInd.p(optionTypeName(opt), " = ", parsed.root.ScalarValue, trailer)
				}*/
		}
		fb.endElem("];", inlineComment(srcLoc))
	}
	fb.trailingComments(srcLoc)
	return nil
}
