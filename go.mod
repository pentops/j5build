module github.com/pentops/bcl.go

go 1.22.4

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.34.2-20240717164558-a6c49f84cc0f.2
	github.com/bufbuild/protocompile v0.14.0
	github.com/bufbuild/protovalidate-go v0.6.4
	github.com/google/go-cmp v0.6.0
	github.com/iancoleman/strcase v0.3.0
	github.com/jhump/protoreflect v1.16.0
	github.com/pentops/j5 v0.0.0-20240813155015-c79c0768e385
	github.com/pentops/prototools v0.0.0-20240806163000-2d02c62be4f1
	github.com/pentops/runner v0.0.0-20240806162317-0eb1ced9ab3d
	github.com/sourcegraph/jsonrpc2 v0.2.0
	github.com/stretchr/testify v1.9.0
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7
	google.golang.org/protobuf v1.34.2
)

require (
	buf.build/gen/go/bufbuild/buf/grpc/go v1.5.1-20240801225352-56ed5eaafdd5.1 // indirect
	buf.build/gen/go/bufbuild/buf/protocolbuffers/go v1.34.2-20240801225352-56ed5eaafdd5.2 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/bufbuild/protoyaml-go v0.1.10 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.17.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.21.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pentops/log.go v0.0.0-20240806161938-2742d05b4c24 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.23.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/pentops/j5 => /Users/daemonl/pentops/j5

replace github.com/pentops/prototools => /Users/daemonl/pentops/prototools

replace github.com/pentops/runner => /Users/daemonl/pentops/runner

replace github.com/bufbuild/protovalidate-go v0.6.4 => github.com/bufbuild/protovalidate-go v0.6.5-0.20240828223213-2e83672b747d

replace github.com/bufbuild/protocompile => /Users/daemonl/projects/protocompile
