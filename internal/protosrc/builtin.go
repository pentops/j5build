package protosrc

import (
	// Pre Loaded Protos
	"fmt"
	"os"
	"strings"
	"sync"

	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/bufbuild/protocompile"
	_ "github.com/pentops/j5/gen/j5/auth/v1/auth_j5pb"
	_ "github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	_ "github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	_ "github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	_ "github.com/pentops/j5/gen/j5/state/v1/psm_j5pb"
	_ "github.com/pentops/j5/j5types/date_j5t"
	_ "github.com/pentops/j5/j5types/decimal_j5t"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	_ "google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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
	"j5/types/any/v1/",
	"j5/schema/v1/",
	"j5/sourcedef/v1",
	"j5/bcl/v1/",
}

func IsBuiltInType(typeName protoreflect.FullName) bool {
	fp := strings.ReplaceAll(string(typeName), ".", "/")
	return IsBuiltInProto(fp)
}

func IsBuiltInProto(filename string) bool {
	for _, prefix := range inbuiltPrefixes {
		if strings.HasPrefix(filename, prefix) {
			return true
		}
	}
	return false
}

type builtinResolver struct {
	lock    sync.Mutex
	inbuilt map[string]protocompile.SearchResult
}

var BuiltinResolver = &builtinResolver{
	inbuilt: map[string]protocompile.SearchResult{},
}

func (rr *builtinResolver) FindFileByPath(filename string) (protocompile.SearchResult, error) {
	if !IsBuiltInProto(filename) {
		return protocompile.SearchResult{}, os.ErrNotExist
	}
	rr.lock.Lock()
	defer rr.lock.Unlock()
	if result, ok := rr.inbuilt[filename]; ok {
		return result, nil
	}
	desc, err := protoregistry.GlobalFiles.FindFileByPath(filename)
	if err != nil {
		return protocompile.SearchResult{}, fmt.Errorf("global file: %w", err)
	}
	res := protocompile.SearchResult{
		Desc: desc,
	}
	rr.inbuilt[filename] = res
	return res, nil
}
