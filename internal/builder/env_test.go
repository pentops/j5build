package builder

import (
	"strings"
	"testing"
)

func TestMapEnvVars(t *testing.T) {
	sampleIn := []string{"PROTOC_GEN_GO_MESSAGING_EXTRA_HEADERS=api-version:$GIT_HASH"}

	info := map[string]string{
		"GIT_HASH": "abcdef",
	}

	r, err := mapEnvVars(sampleIn, info)
	if err != nil {
		t.Errorf("Received error in mapenvvars: %v", err.Error())
	}
	if len(r) != 1 {
		t.Error("Got len other than expected")
	}
	if strings.Count(r[0], "=") > 1 {
		t.Errorf("Too many equal signs in env var %s", r[0])
	}
	if r[0] != "PROTOC_GEN_GO_MESSAGING_EXTRA_HEADERS=api-version:abcdef" {
		t.Error("Output not correct for git hash substitution")
	}
}
