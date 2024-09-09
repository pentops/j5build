package builder

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/docker/docker/client"
	"github.com/pentops/j5/gen/j5/config/v1/config_j5pb"
	"go.opentelemetry.io/otel/trace/noop"
)

type Runner struct {
	pulledImages map[string]bool
	pullLock     sync.Mutex

	client *client.Client
	auth   []*config_j5pb.DockerRegistryAuth

	DockerOverride map[string]string // map[cmd]localCommand
}

func NewRunner(registryAuth []*config_j5pb.DockerRegistryAuth) (*Runner, error) {

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithTraceProvider(noop.NewTracerProvider()),
	)
	if err != nil {
		return nil, err
	}

	return &Runner{
		pulledImages: make(map[string]bool),
		client:       cli,
		auth:         registryAuth,
	}, nil
}

func (dw *Runner) Close() error {
	return dw.client.Close()
}

type RunContext struct {
	Vars    map[string]string
	StdIn   io.Reader
	StdOut  io.Writer
	StdErr  io.Writer
	Command *config_j5pb.BuildPlugin
}

func (rr *Runner) Run(ctx context.Context, rc RunContext) error {
	envVars, err := mapEnvVars(rc.Command.Env, rc.Vars)
	if err != nil {
		return err
	}
	baseEnv := os.Environ()
	envVars = append(baseEnv, envVars...)

	if rc.Command.Docker == nil {
		cmd := exec.CommandContext(ctx, rc.Command.Cmd, rc.Command.Args...)
		cmd.Stdin = rc.StdIn
		cmd.Stdout = rc.StdOut
		cmd.Stderr = rc.StdErr
		cmd.Env = envVars
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("running command %q: %w", rc.Command.Cmd, err)
		}
		return nil
	}

	err = rr.runDocker(ctx, rc)
	if err != nil {
		return fmt.Errorf("running docker: %w", err)
	}
	return nil

}

func mapEnvVars(spec []string, vars map[string]string) ([]string, error) {
	env := make([]string, len(spec))
	for idx, src := range spec {

		parts := strings.Split(src, "=")
		if len(parts) == 1 {
			env[idx] = fmt.Sprintf("%s=%s", src, os.Getenv(src))
			continue
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env var: %s", src)
		}
		val := os.Expand(parts[1], func(key string) string {
			if v, ok := vars[key]; ok {
				return v
			}
			return ""
		})

		env[idx] = fmt.Sprintf("%s=%s", parts[0], val)
	}
	return env, nil
}
