package buildlib

import (
	"encoding/json"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/internal/export"
	"github.com/pentops/j5build/internal/j5client"
	"github.com/pentops/j5build/internal/structure"
)

func DescriptorFromSource(img *source_j5pb.SourceImage) (*client_j5pb.API, error) {
	sourceAPI, err := structure.APIFromImage(img)
	if err != nil {
		return nil, err
	}

	clientAPI, err := j5client.APIFromSource(sourceAPI)
	if err != nil {
		return nil, err
	}

	err = structure.ResolveProse(img, clientAPI)
	if err != nil {
		return nil, err
	}

	return clientAPI, nil
}

func SwaggerFromDescriptor(descriptorAPI *client_j5pb.API) ([]byte, error) {
	swaggerDoc, err := export.BuildSwagger(descriptorAPI)
	if err != nil {
		return nil, err
	}

	asJson, err := json.Marshal(swaggerDoc)
	if err != nil {
		return nil, err
	}

	return asJson, nil
}

func JDefFromDescriptor(descriptorAPI *client_j5pb.API) ([]byte, error) {
	jDefJSON, err := export.FromProto(descriptorAPI)
	if err != nil {
		return nil, err
	}

	jDefJSONBytes, err := json.Marshal(jDefJSON)
	if err != nil {
		return nil, err
	}

	return jDefJSONBytes, nil
}
