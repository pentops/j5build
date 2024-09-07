package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/proto"
)

type registryClient struct {
	remote string
	auth   string
	client *http.Client
}

func NewRegistryClient(remote string, authToken string) (*registryClient, error) {

	auth := ""
	if authToken != "" {
		auth = fmt.Sprintf("Bearer %s", authToken)
	}

	return &registryClient{
		remote: remote,
		auth:   auth,
		client: http.DefaultClient,
	}, nil
}

func envRegistryClient() (*registryClient, error) {
	addr := os.Getenv("J5_REGISTRY")
	token := os.Getenv("J5_REGISTRY_TOKEN")
	return NewRegistryClient(addr, token)
}

func (rc *registryClient) LatestImage(ctx context.Context, owner, repoName string, reference *string) (*source_j5pb.SourceImage, error) {
	if rc == nil {
		return nil, fmt.Errorf("registry client not set")
	}

	branch := "main"
	if reference != nil {
		branch = *reference
	}

	// registry returns the canonical version in the image
	return rc.GetImage(ctx, owner, repoName, branch)
}

func (rc *registryClient) GetImage(ctx context.Context, owner, repoName, version string) (*source_j5pb.SourceImage, error) {
	if rc == nil {
		return nil, fmt.Errorf("registry client not set")
	}

	fullName := fmt.Sprintf("registry/v1/%s/%s", owner, repoName)
	ctx = log.WithField(ctx, "bundle", fullName)
	log.Debug(ctx, "cache miss")

	imageURL := fmt.Sprintf("%s/%s/%s/image.bin", rc.remote, fullName, version)
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating registry input request: %w", err)
	}
	if rc.auth != "" {
		req.Header.Set("Authorization", rc.auth)
	}
	req = req.WithContext(ctx)

	res, err := rc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching registry input: %q %w", imageURL, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading registry input: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching registry input: %q %s %q", imageURL, res.Status, string(data))
	}

	apiDef := &source_j5pb.SourceImage{}
	if err := proto.Unmarshal(data, apiDef); err != nil {
		return nil, fmt.Errorf("unmarshalling registry input %s: %w", imageURL, err)
	}

	return apiDef, nil
}
