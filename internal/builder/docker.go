package builder

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pentops/j5/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/log.go/log"

	glob "github.com/ryanuber/go-glob"
)

var DefaultRegistryAuths = []*config_j5pb.DockerRegistryAuth{{
	Registry: "ghcr.io/*",
	Auth: &config_j5pb.DockerRegistryAuth_Github_{
		Github: &config_j5pb.DockerRegistryAuth_Github{},
	},
}, {
	Registry: "*.dkr.ecr.*.amazonaws.com/*",
	Auth: &config_j5pb.DockerRegistryAuth_AwsEcs{
		AwsEcs: &config_j5pb.DockerRegistryAuth_AWSECS{},
	},
}}

func (dw *Runner) runDocker(ctx context.Context, rc RunContext) error {

	if err := dw.pullIfNeeded(ctx, rc.Command.Docker.Image); err != nil {
		return err
	}

	resp, err := dw.client.ContainerCreate(ctx, &container.Config{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		StdinOnce:    true,
		OpenStdin:    true,

		Tty: false,

		Env:        rc.Command.Env,
		Image:      rc.Command.Docker.Image,
		Entrypoint: rc.Command.Docker.Entrypoint,
		Cmd:        rc.Command.Docker.Cmd,
	}, nil, nil, nil, "")
	if err != nil {
		return err
	}
	defer func() {
		if err := dw.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); err != nil {
			log.WithError(ctx, err).Error("failed to remove container")
		}
	}()

	hj, err := dw.client.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Stream: true,
		Logs:   true,
	})
	if err != nil {
		return err
	}

	defer hj.Close()

	t0 := time.Now()

	log.WithField(ctx, "t0", time.Since(t0).String()).Debug("ContainerStart")

	if err := dw.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	log.WithField(ctx, "t0", time.Since(t0).String()).Debug("ContainerStarted")

	chOut := make(chan error)
	go func() {
		_, err = stdcopy.StdCopy(rc.StdOut, rc.StdErr, hj.Reader)
		chOut <- err
	}()

	if _, err := io.Copy(hj.Conn, rc.StdIn); err != nil {
		return err
	}
	if err := hj.CloseWrite(); err != nil {
		return err
	}

	if err := <-chOut; err != nil {
		return fmt.Errorf("output copy error: %w", err)
	}

	log.WithField(ctx, "t0", time.Since(t0).String()).Debug("ContainerWait")

	statusCh, errCh := dw.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case st := <-statusCh:
		if st.StatusCode != 0 {
			return fmt.Errorf("non-zero exit code: %d", st.StatusCode)
		}

	}

	log.WithField(ctx, "t0", time.Since(t0).String()).Debug("ContainerDone")

	return nil
}

func (dw *Runner) markPull(img string) bool {
	dw.pullLock.Lock()
	defer dw.pullLock.Unlock()

	// skip if pulled...
	if dw.pulledImages[img] {
		return true
	}

	// only pull once for all plugins
	dw.pulledImages[img] = true
	return false
}
func (dw *Runner) pullIfNeeded(ctx context.Context, img string) error {

	alreadyPulled := dw.markPull(img)
	if alreadyPulled {
		return nil
	}

	images, err := dw.client.ImageList(ctx, image.ListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", img)),
	})
	if err != nil {
		return fmt.Errorf("image list: %w", err)
	}
	if len(images) > 0 {
		return nil
	}

	type basicAuth struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	pullOptions := image.PullOptions{}

	var registryAuth *config_j5pb.DockerRegistryAuth
	for _, auth := range dw.auth {
		// If auth's registry pattern with * wildcards matches the spec's image, use it.
		if glob.Glob(auth.Registry, img) {
			registryAuth = auth
			log.WithField(ctx, "registry", auth.Registry).Debug("using auth")
			break
		}
	}
	if registryAuth == nil {
		log.WithField(ctx, "image", img).Debug("no registry auth matched")
	}

	if registryAuth != nil {
		pullOptions.PrivilegeFunc = func(ctx context.Context) (string, error) {
			var authConfig *basicAuth

			switch authType := registryAuth.Auth.(type) {
			case *config_j5pb.DockerRegistryAuth_Basic_:
				val := os.Getenv(authType.Basic.PasswordEnvVar)
				if val == "" {
					return "", fmt.Errorf("basic auth password (%s) not set", authType.Basic.PasswordEnvVar)
				}

				authConfig = &basicAuth{
					Username: authType.Basic.Username,
					Password: val,
				}

			case *config_j5pb.DockerRegistryAuth_Github_:
				envVar := authType.Github.TokenEnvVar
				if envVar == "" {
					envVar = "GITHUB_TOKEN"
				}
				val := os.Getenv(envVar)
				if val == "" {
					return "", fmt.Errorf("github token (%s) not set", envVar)
				}

				authConfig = &basicAuth{
					Username: "GITHUB",
					Password: val,
				}

			case *config_j5pb.DockerRegistryAuth_AwsEcs:

				// TODO: This is a little too magic.
				awsConfig, err := config.LoadDefaultConfig(ctx)
				if err != nil {
					return "", fmt.Errorf("failed to load configuration: %w", err)
				}

				ecrClient := ecr.NewFromConfig(awsConfig)
				resp, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
				if err != nil {
					return "", fmt.Errorf("failed to get authorization token: %w", err)
				}

				if len(resp.AuthorizationData) == 0 {
					return "", fmt.Errorf("no authorization data returned")
				}

				authData, err := base64.StdEncoding.DecodeString(*resp.AuthorizationData[0].AuthorizationToken)
				if err != nil {
					return "", fmt.Errorf("failed to decode authorization token: %w", err)
				}

				parts := strings.SplitN(string(authData), ":", 2)
				if len(parts) != 2 {
					return "", fmt.Errorf("invalid authorization token")
				}

				authConfig = &basicAuth{
					Username: parts[0],
					Password: parts[1],
				}

			default:
				return "", fmt.Errorf("unknown auth type: %T", authType)
			}
			cred, _ := json.Marshal(authConfig)
			return base64.StdEncoding.EncodeToString(cred), nil
		}
	}

	reader, err := dw.client.ImagePull(ctx, img, pullOptions)
	if err != nil {
		// The ECS registry seems to return the 'wrong' status code for PrivilegeFunc errors.
		// This is a workaround.
		if strings.Contains(err.Error(), "no basic auth credentials") {
			token, err := pullOptions.PrivilegeFunc(ctx)
			if err != nil {
				return fmt.Errorf("image pull: %w", err)
			}
			pullOptions.PrivilegeFunc = nil
			pullOptions.RegistryAuth = token
			reader, err = dw.client.ImagePull(ctx, img, pullOptions)
			if err != nil {
				return fmt.Errorf("image pull: %w", err)
			}
		} else {
			return fmt.Errorf("image pull: %w", err)
		}
	}

	// cli.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.
	_, err = io.Copy(os.Stdout, reader)
	reader.Close()
	if err != nil {
		return err
	}
	return nil
}
