package structure

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5/lib/j5schema"
	"github.com/pentops/j5/lib/patherr"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func APIFromImage(image *source_j5pb.SourceImage) (*source_j5pb.API, error) {

	if image.Options == nil {
		image.Options = &config_j5pb.PackageOptions{}
	}

	subPackageNames := make(map[string]struct{})
	for _, subPackage := range image.Options.SubPackages {
		subPackageNames[subPackage.Name] = struct{}{}
	}

	bb := packageSet{
		subPackageNames: subPackageNames,
		wantPackages:    map[string]bool{},
	}

	for _, pkg := range image.Packages {
		bb.wantPackages[pkg.Name] = true
		bb.packages = append(bb.packages, &source_j5pb.Package{
			Name:    pkg.Name,
			Label:   pkg.Label,
			Schemas: map[string]*schema_j5pb.RootSchema{},
		})
	}

	descFiles, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: image.File,
	})
	if err != nil {
		return nil, err
	}

	if err := bb.addStructure(descFiles); err != nil {
		return nil, err
	}

	selector := func(f protoreflect.FileDescriptor) bool {
		name := string(f.Package())
		for _, pkg := range image.Packages {
			if strings.HasPrefix(name, pkg.Name) {
				return true
			}
		}
		return false
	}

	if err := bb.addSchemas(descFiles, selector); err != nil {
		return nil, err
	}

	return bb.toAPI(), nil
}

func (b packageSet) toAPI() *source_j5pb.API {
	return &source_j5pb.API{
		Packages: b.packages,
	}
}

func (bb *packageSet) addSchemas(descFiles *protoregistry.Files, selector func(f protoreflect.FileDescriptor) bool) error {
	packageSet, err := j5schema.SchemaSetFromFiles(descFiles, selector)
	if err != nil {
		return fmt.Errorf("package set from files: %w", err)
	}

	for _, schemaPkg := range packageSet.Packages {
		ss, err := bb.getSchemaSet(schemaPkg.Name)
		if err != nil {
			return fmt.Errorf("get schema set: %w", err)
		}
		for name, schema := range schemaPkg.Schemas {
			ss[name] = schema.To.ToJ5Root()
		}

	}
	return nil
}
func (b packageSet) addStructure(descFiles *protoregistry.Files) error {

	services := make([]protoreflect.ServiceDescriptor, 0)

	descFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		fileServices := file.Services()
		for ii := 0; ii < fileServices.Len(); ii++ {
			service := fileServices.Get(ii)
			services = append(services, service)
		}

		return true
	})

	for _, service := range services {
		packageID, err := splitPackageParts(string(service.ParentFile().Package()))
		if err != nil {
			return err
		}
		if !b.wantPackages[packageID.packageName] {
			continue
		}

		pkg, err := b.getSubPackage(packageID)
		if err != nil {
			return patherr.Wrap(err, "service", string(service.Name()))
		}

		name := string(service.Name())
		if strings.HasSuffix(name, "Service") || strings.HasSuffix(name, "Sandbox") {
			built, err := buildService(service)
			if err != nil {
				return patherr.Wrap(err, "service", name)
			}
			pkg.Services = append(pkg.Services, built)
		} else if strings.HasSuffix(name, "Events") {
			// ignore for now.
		} else if strings.HasSuffix(name, "Topic") {
			built, err := buildTopic(service)
			if err != nil {
				return patherr.Wrap(err, string(service.FullName()))
			}
			pkg.Topics = append(pkg.Topics, built)
		} else {
			return fmt.Errorf("unsupported service name %q", name)
		}
	}
	return nil

}

type packageSet struct {
	packages        []*source_j5pb.Package
	wantPackages    map[string]bool
	subPackageNames map[string]struct{}
}

func (bb *packageSet) getSchemaSet(name string) (map[string]*schema_j5pb.RootSchema, error) {
	packageID, err := splitPackageParts(name)
	if err != nil {
		return nil, err
	}

	if packageID.subPackage == nil {
		pkg := bb.getPackage(packageID.packageName)
		return pkg.Schemas, nil
	}

	subPkg, err := bb.getSubPackage(packageID)
	if err != nil {
		return nil, err
	}

	return subPkg.Schemas, nil
}

func (bb *packageSet) getPackage(name string) *source_j5pb.Package {
	var pkg *source_j5pb.Package
	for _, search := range bb.packages {
		if search.Name == name {
			pkg = search
			break
		}
	}

	if pkg == nil {
		pkg = &source_j5pb.Package{
			Name:    name,
			Schemas: make(map[string]*schema_j5pb.RootSchema),
		}
		bb.packages = append(bb.packages, pkg)
	}

	return pkg
}

func (bb *packageSet) getSubPackage(packageID *packageID) (*source_j5pb.SubPackage, error) {

	if packageID.subPackage == nil {
		return nil, fmt.Errorf("missing sub-package name")
	}

	if _, ok := bb.subPackageNames[*packageID.subPackage]; !ok {
		return nil, fmt.Errorf("unknown sub-package name in %q %q", packageID.packageName, *packageID.subPackage)
	}

	parentPkg := bb.getPackage(packageID.packageName)

	for _, subPkg := range parentPkg.SubPackages {
		if subPkg.Name == *packageID.subPackage {
			return subPkg, nil
		}
	}

	pkg := &source_j5pb.SubPackage{
		Name:    *packageID.subPackage,
		Schemas: make(map[string]*schema_j5pb.RootSchema),
	}
	parentPkg.SubPackages = append(parentPkg.SubPackages, pkg)

	return pkg, nil
}

type packageID struct {
	packageName string
	subPackage  *string
}

var reVersion = regexp.MustCompile(`^v[0-9]+$`)

func SplitPackageParts(packageName string) (string, *string, error) {
	id, err := splitPackageParts(packageName)
	if err != nil {
		return "", nil, err
	}
	return id.packageName, id.subPackage, nil
}
func splitPackageParts(packageName string) (*packageID, error) {
	packageParts := strings.Split(packageName, ".")
	var idxOfVersion int = -1

	for idx, part := range packageParts {
		if reVersion.MatchString(part) {
			if idxOfVersion != -1 {
				return nil, fmt.Errorf("package %q: multiple path parts matched version regex", packageName)
			}
			idxOfVersion = idx
		}
	}
	if idxOfVersion == -1 {
		return nil, fmt.Errorf("package %q: no version part found", packageName)
	}

	prefixParts := packageParts[:idxOfVersion+1]
	suffixParts := packageParts[idxOfVersion+1:]
	if len(suffixParts) == 0 {
		return &packageID{
			packageName: packageName,
		}, nil
	} else if len(suffixParts) == 1 {
		return &packageID{
			packageName: strings.Join(prefixParts, "."),
			subPackage:  &suffixParts[0],
		}, nil
	} else {
		return nil, fmt.Errorf("package %q: multiple sub version path parts", packageName)
	}
}

/*
func buildEvents(src protoreflect.ServiceDescriptor) ([]*j5schema.Event, error) {
	events := make([]*j5package.Event, 0)
	methods := src.Methods()
	for ii := 0; ii < methods.Len(); ii++ {
		method := methods.Get(ii)

		schema, err := refs.SchemaReflect(method.Input())
		if err != nil {
			return nil, fmt.Errorf("method %s: %w", method.FullName(), err)
		}
		objSchema, ok := schema.(*j5schema.ObjectSchema)
		if !ok {
			return nil, fmt.Errorf("method %s: expected object schema", method.FullName())
		}

		eventSpec := &j5schema.Event{
			Name:   string(method.Name()),
			Schema: objSchema,
		}

		events = append(events, eventSpec)
	}
	return events, nil
}*/

func buildService(src protoreflect.ServiceDescriptor) (*source_j5pb.Service, error) {

	methods := src.Methods()
	service := &source_j5pb.Service{
		Name:    string(src.Name()),
		Methods: make([]*source_j5pb.Method, 0, methods.Len()),
	}

	if serviceExt := proto.GetExtension(src.Options(), ext_j5pb.E_Service).(*ext_j5pb.ServiceOptions); serviceExt != nil {
		if serviceExt.Type != nil {
			switch set := serviceExt.Type.(type) {
			case *ext_j5pb.ServiceOptions_StateQuery_:
				service.Type = &source_j5pb.ServiceType{
					Type: &source_j5pb.ServiceType_StateEntityQuery_{
						StateEntityQuery: &source_j5pb.ServiceType_StateEntityQuery{
							Entity: set.StateQuery.Entity,
						},
					},
				}

			case *ext_j5pb.ServiceOptions_StateCommand_:
				service.Type = &source_j5pb.ServiceType{
					Type: &source_j5pb.ServiceType_StateEntityCommand_{
						StateEntityCommand: &source_j5pb.ServiceType_StateEntityCommand{
							Entity: set.StateCommand.Entity,
						},
					},
				}

			default:
				return nil, fmt.Errorf("unsupported state service type %T", set)

			}

		}

		service.DefaultAuth = serviceExt.DefaultAuth
	}

	for ii := 0; ii < methods.Len(); ii++ {
		method := methods.Get(ii)
		builtMethod, err := buildMethod(method)
		if err != nil {
			return nil, fmt.Errorf("build method %s: %w", method.FullName(), err)
		}
		service.Methods = append(service.Methods, builtMethod)
	}
	return service, nil
}

func buildMethod(method protoreflect.MethodDescriptor) (*source_j5pb.Method, error) {

	input := method.Input()
	expectedInputName := method.Name() + "Request"
	if input.ParentFile().Package() != method.ParentFile().Package() || input.Name() != expectedInputName {
		return nil, fmt.Errorf("j5 service input message must be %q, got %q", expectedInputName, input.Name())
	}
	output := method.Output()
	expectedOutputName := method.Name() + "Response"
	if output.Name() != expectedOutputName {
		if output.FullName() != "google.api.HttpBody" {
			return nil, fmt.Errorf("j5 service output message must be %q, got %q", expectedOutputName, output.FullName())
		}
	}

	httpOpt := proto.GetExtension(method.Options(), annotations.E_Http).(*annotations.HttpRule)

	if httpOpt == nil {
		return nil, fmt.Errorf("missing http rule")
	}

	builtMethod := &source_j5pb.Method{
		Name:           string(method.Name()),
		FullGrpcName:   fmt.Sprintf("/%s/%s", method.Parent().FullName(), method.Name()),
		RequestSchema:  string(method.Input().Name()),
		ResponseSchema: string(method.Output().Name()),
	}

	switch pt := httpOpt.Pattern.(type) {
	case *annotations.HttpRule_Get:
		builtMethod.HttpMethod = client_j5pb.HTTPMethod_HTTP_METHOD_GET
		builtMethod.HttpPath = pt.Get

	case *annotations.HttpRule_Post:
		builtMethod.HttpMethod = client_j5pb.HTTPMethod_HTTP_METHOD_POST
		builtMethod.HttpPath = pt.Post

	case *annotations.HttpRule_Put:
		builtMethod.HttpMethod = client_j5pb.HTTPMethod_HTTP_METHOD_PUT
		builtMethod.HttpPath = pt.Put

	case *annotations.HttpRule_Delete:
		builtMethod.HttpMethod = client_j5pb.HTTPMethod_HTTP_METHOD_DELETE
		builtMethod.HttpPath = pt.Delete

	case *annotations.HttpRule_Patch:
		builtMethod.HttpMethod = client_j5pb.HTTPMethod_HTTP_METHOD_PATCH
		builtMethod.HttpPath = pt.Patch

	default:
		return nil, fmt.Errorf("unsupported http method %T", pt)
	}

	pathParts := strings.Split(builtMethod.HttpPath, "/")
	for idx, part := range pathParts {
		if part == "" {
			continue
		}
		if part[0] == '{' && part[len(part)-1] == '}' {
			fieldName := part[1 : len(part)-1]
			inputField := method.Input().Fields().ByName(protoreflect.Name(fieldName))
			if inputField == nil {
				return nil, fmt.Errorf("path field %q not found in input", fieldName)
			}
			jsonName := inputField.JSONName()
			pathParts[idx] = ":" + jsonName

		} else if strings.ContainsAny(part, "{}*:") {
			return nil, fmt.Errorf("invalid path part %q", part)
		}

	}
	builtMethod.HttpPath = strings.Join(pathParts, "/")

	if ext := proto.GetExtension(method.Options(), ext_j5pb.E_Method).(*ext_j5pb.MethodOptions); ext != nil {
		builtMethod.Auth = ext.Auth
	}

	return builtMethod, nil
}

func buildTopic(src protoreflect.ServiceDescriptor) (*source_j5pb.Topic, error) {
	methods := src.Methods()
	topic := &source_j5pb.Topic{
		Name:     string(src.Name()),
		Messages: make([]*source_j5pb.TopicMessage, 0, methods.Len()),
	}
	for ii := 0; ii < methods.Len(); ii++ {
		method := methods.Get(ii)
		builtMethod, err := buildTopicMethod(method)
		if err != nil {
			return nil, patherr.Wrap(err, "method", string(method.Name()))
		}
		topic.Messages = append(topic.Messages, builtMethod)
	}
	return topic, nil
}

func buildTopicMethod(method protoreflect.MethodDescriptor) (*source_j5pb.TopicMessage, error) {
	input := method.Input()
	expectedName := method.Name() + "Message"
	if input.ParentFile().Package() != method.ParentFile().Package() || input.Name() != expectedName {
		return nil, fmt.Errorf("j5 topic input message must be %s in the same package", expectedName)
	}
	output := method.Output()
	if output.FullName() != "google.protobuf.Empty" {
		return nil, fmt.Errorf("j5 topic output message must be google.protobuf.Empty, got %q", output.FullName())
	}
	return &source_j5pb.TopicMessage{
		Name:         string(method.Name()),
		Schema:       string(method.Input().Name()),
		FullGrpcName: string(method.FullName()),
	}, nil
}
