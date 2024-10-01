package j5convert

import (
	"path"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/internal/conversions/sourcewalk"
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

func convertService(ww *walkContext, sn *sourcewalk.ServiceFileNode) error {
	return sn.Accept(sourcewalk.ServiceFileCallbacks{
		Service: func(sn *sourcewalk.ServiceNode) error {
			convertServiceNode(ww, sn)
			return nil
		},
		Object: func(on *sourcewalk.ObjectNode) error {
			convertObject(ww, on)
			return nil
		},
	})
}

func convertServiceNode(ww *walkContext, node *sourcewalk.ServiceNode) {

	serviceWalker := ww.subPackageFile("service")

	service := blankService(serviceWalker.file, node.Name)
	service.basePath = node.BasePath

	for _, method := range node.Methods {
		convertMethod(ww, service, method)
	}

	if node.ServiceOptions != nil {
		service.desc.Options = &descriptorpb.ServiceOptions{}
		proto.SetExtension(service.desc.Options, ext_j5pb.E_Service, node.ServiceOptions)
	}

	serviceWalker.file.addService(service)

}

func convertMethod(ww *walkContext, service *ServiceBuilder, node *sourcewalk.ServiceMethodNode) {

	method := node.Schema
	methodBuilder := blankMethod(method.Name)
	methodBuilder.comment([]int32{}, method.Description)
	ww.file.ensureImport(googleApiAnnotationsImport)

	if method.Request == nil {
		ww.errorf(node.Source, "missing input")
		return
	}

	methodBuilder.desc.InputType = ptr(node.InputType)
	methodBuilder.desc.OutputType = ptr(node.OutputType)

	if node.OutputType == "google.api.HttpBody" {
		ww.file.ensureImport(googleApiHttpBodyImport)
	}

	annotation := &annotations.HttpRule{}
	reqPathParts := strings.Split(path.Join(service.basePath, method.HttpPath), "/")
	for idx, part := range reqPathParts {
		if strings.HasPrefix(part, ":") {
			var field *schema_j5pb.ObjectProperty
			for _, search := range method.Request.Properties {
				if search.Name == part[1:] {
					field = search
					break
				}
			}
			if field == nil {
				ww.errorf(node.Source, "missing field %s in request", part[1:])
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
		ww.errorf(node.Source, "unsupported http method %s", method.HttpMethod)
		return
	}

	proto.SetExtension(methodBuilder.desc.Options, annotations.E_Http, annotation)

	if method.Options != nil {
		proto.SetExtension(methodBuilder.desc.Options, ext_j5pb.E_Method, method.Options)
	}
	service.desc.Method = append(service.desc.Method, methodBuilder.desc)
}
