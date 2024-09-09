package j5convert

import (
	"fmt"
	"path"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ServiceBuilder struct {
	root     fileContext // service always belongs to a file
	desc     *descriptorpb.ServiceDescriptorProto
	basePath string
	commentSet
}

func blankService(root fileContext, name string) *ServiceBuilder {
	return &ServiceBuilder{
		root: root,
		desc: &descriptorpb.ServiceDescriptorProto{
			Name: ptr(name),
		},
	}
}

type MethodBuilder struct {
	desc *descriptorpb.MethodDescriptorProto
	commentSet
}

func blankMethod(name string) *MethodBuilder {
	return &MethodBuilder{
		desc: &descriptorpb.MethodDescriptorProto{
			Name:    ptr(name),
			Options: &descriptorpb.MethodOptions{},
		},
	}
}

func (ww *walkNode) doService(spec *sourcedef_j5pb.Service) {
	serviceWalker := ww.subPackageFile("service")

	service := blankService(serviceWalker.file, spec.Name+"Service")
	service.basePath = spec.BasePath

	for idx, method := range spec.Methods {
		serviceWalker.at("methods", fmt.Sprint(idx)).doMethod(service, method)
	}

	if spec.Options != nil {
		service.desc.Options = &descriptorpb.ServiceOptions{}
		proto.SetExtension(service.desc.Options, ext_j5pb.E_Service, spec.Options)
	}

	serviceWalker.file.addService(service)
}

func (ww *walkNode) doMethod(service *ServiceBuilder, method *sourcedef_j5pb.Method) {
	methodBuilder := blankMethod(method.Name)
	methodBuilder.comment([]int32{}, method.Description)
	ww.file.ensureImport(googleApiAnnotationsImport)

	if method.Request == nil {
		ww.errorf("missing input")
		return
	}
	methodBuilder.desc.InputType = ptr(fmt.Sprintf("%sRequest", method.Name))
	request := &schema_j5pb.Object{
		Name:       fmt.Sprintf("%sRequest", method.Name),
		Properties: method.Request.Properties,
	}
	ww.at("request").doObject(request)

	if method.Response == nil {
		ww.file.ensureImport(googleApiHttpBodyImport)
		methodBuilder.desc.OutputType = ptr("google.api.HttpBody")
	} else {
		methodBuilder.desc.OutputType = ptr(fmt.Sprintf("%sResponse", method.Name))
		response := &schema_j5pb.Object{
			Name:       fmt.Sprintf("%sResponse", method.Name),
			Properties: method.Response.Properties,
		}
		ww.at("response").doObject(response)
	}

	annotation := &annotations.HttpRule{}
	reqPathParts := strings.Split(path.Join(service.basePath, method.HttpPath), "/")
	for idx, part := range reqPathParts {
		if strings.HasPrefix(part, ":") {
			var field *schema_j5pb.ObjectProperty
			for _, search := range request.Properties {
				if search.Name == part[1:] {
					field = search
					break
				}
			}
			if field == nil {
				ww.errorf("missing field %s in request", part[1:])
			}

			fieldName := strcase.ToSnake(part[1:])
			reqPathParts[idx] = "{" + fieldName + "}"

		}
	}

	reqPath := strings.Join(reqPathParts, "/")

	switch method.HttpMethod {
	case client_j5pb.HTTPMethod_GET:
		annotation.Pattern = &annotations.HttpRule_Get{
			Get: reqPath,
		}
	case client_j5pb.HTTPMethod_POST:
		annotation.Pattern = &annotations.HttpRule_Post{
			Post: reqPath,
		}
		annotation.Body = "*"

	case client_j5pb.HTTPMethod_DELETE:
		annotation.Pattern = &annotations.HttpRule_Delete{
			Delete: reqPath,
		}
		annotation.Body = "*"

	case client_j5pb.HTTPMethod_PATCH:
		annotation.Pattern = &annotations.HttpRule_Patch{
			Patch: reqPath,
		}
		annotation.Body = "*"

	case client_j5pb.HTTPMethod_PUT:
		annotation.Pattern = &annotations.HttpRule_Put{
			Put: reqPath,
		}
		annotation.Body = "*"

	default:
		ww.errorf("unsupported http method %s", method.HttpMethod)
		return
	}

	proto.SetExtension(methodBuilder.desc.Options, annotations.E_Http, annotation)

	if method.Options != nil {
		proto.SetExtension(methodBuilder.desc.Options, ext_j5pb.E_Method, method.Options)
	}
	service.desc.Method = append(service.desc.Method, methodBuilder.desc)
}
