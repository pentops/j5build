package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/pentops/j5build/internal/protosrc"
	"github.com/pentops/runner/commander"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func protocSet() *commander.CommandSet {
	protoGroup := commander.NewCommandSet()
	protoGroup.Add("request", commander.NewCommand(runProtoRequest))
	return protoGroup
}

func runProtoRequest(ctx context.Context, cfg struct {
	SourceConfig
	Command string `flag:"command" default:"" description:"Pipe the output to a builder command and print files"`
}) error {

	img, _, err := cfg.GetBundleImage(ctx)
	if err != nil {
		return err
	}

	protoBuildRequest, err := protosrc.CodeGeneratorRequestFromImage(img)
	if err != nil {
		return err
	}

	protoBuildRequestBytes, err := proto.Marshal(protoBuildRequest)
	if err != nil {
		return err
	}

	if cfg.Command == "" {
		_, err = os.Stdout.Write(protoBuildRequestBytes)
		return err
	}

	cmd := exec.CommandContext(ctx, cfg.Command)

	inPipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer inPipe.Close()

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer outPipe.Close()

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	outErr := make(chan error)
	outBuf := &bytes.Buffer{}
	go func() {
		_, err := io.Copy(outBuf, outPipe)
		outErr <- err
	}()

	if _, err := inPipe.Write(protoBuildRequestBytes); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	outPipe.Close()

	if err := <-outErr; err != nil {
		return err
	}

	res := pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(outBuf.Bytes(), &res); err != nil {
		return err
	}

	for _, file := range res.File {
		fmt.Println(file.GetName())
	}

	return nil
}
