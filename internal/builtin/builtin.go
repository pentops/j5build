package builtin

import (
	// Pre Loaded Protos
	"strings"

	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	_ "github.com/pentops/j5/gen/j5/auth/v1/auth_j5pb"
	_ "github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	_ "github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	_ "github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	_ "github.com/pentops/j5/gen/j5/state/v1/psm_j5pb"
	_ "github.com/pentops/j5/j5types/date_j5t"
	_ "github.com/pentops/j5/j5types/decimal_j5t"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	_ "google.golang.org/genproto/googleapis/api/httpbody"
)

var inbuiltPrefixes = []string{
	"google/protobuf/",
	"google/api/",
	"buf/validate/",
	"j5/ext/v1/",
	"j5/list/v1/",
	"j5/source/v1/",
	"j5/messaging/v1/",
	"j5/state/v1/",
	"j5/client/v1/",
	"j5/auth/v1/",
	"j5/types/decimal/v1/",
	"j5/types/date/v1/",
}

func IsBuiltInProto(filename string) bool {
	for _, prefix := range inbuiltPrefixes {
		if strings.HasPrefix(filename, prefix) {
			return true
		}
	}
	return false
}
