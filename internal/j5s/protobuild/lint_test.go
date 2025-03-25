package protobuild

import (
	"context"
	"strings"
	"testing"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/log.go/log"
)

func testLint(t *testing.T, tf *testFiles, td *testDeps) *errpos.ErrorsWithSource {
	before := log.DefaultLogger
	log.DefaultLogger = log.NewTestLogger(t)
	defer func() {
		log.DefaultLogger = before
	}()
	cc, err := NewPackageSet(td, tf)
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}

	out, err := LintAll(context.Background(), cc)
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}

	return out
}

func TestLintImport(t *testing.T) {
	tf := newTestFiles()

	tf.tAddJ5SFile("local/v1/foo.j5s",
		"object Foo {",
		"  field bar object:Bar", // No import required for local package
		"}",
	)

	tf.tAddProtoFile("local/v1/bar.proto",
		"message Bar {",
		"  string f1 = 1;",
		"}",
	)

	td := newTestDeps()

	es := testLint(t, tf, td)
	if es != nil {
		t.Fatalf("Expected no errors, got %v", es)
	}
}

func TestLintSingleProto(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    []string
		wantErr string
	}{{
		name: "unused import",
		body: []string{
			`import "j5/messaging/v1/annotations.proto";`,
			"message Foo {",
			"}",
		},
		wantErr: "not used",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			tf := newTestFiles()
			tf.tAddProtoFile("local/v1/foo.proto",
				tc.body...,
			)
			td := newTestDeps()
			es := testLint(t, tf, td)
			if es == nil {
				t.Fatalf("Expected errors, got nil")
			}

			if len(es.Errors) != 1 {
				t.Fatalf("Expected 1 error, got %d", len(es.Errors))
			}
			msg := es.Errors[0].Err.Error()
			t.Logf("Got error: %s", msg)
			if !strings.Contains(msg, tc.wantErr) {
				t.Fatalf("Expected error containing %q, got %q", tc.wantErr, msg)
			}
		})
	}
}

func TestLintSingleJ5(t *testing.T) {

	for _, tc := range []struct {
		name    string
		body    []string
		wantErr string
	}{{
		name: "unused import",
		body: []string{
			`import "j5/messaging/v1/annotations.proto"`,
			"object Foo {",
			"}",
		},
		wantErr: "not used",
	}} {

		t.Run(tc.name, func(t *testing.T) {
			tf := newTestFiles()
			tf.tAddJ5SFile("local/v1/foo.j5s",
				tc.body...,
			)
			td := newTestDeps()
			es := testLint(t, tf, td)
			if es == nil {
				t.Fatalf("Expected errors, got nil")
			}

			if len(es.Errors) != 1 {
				t.Fatalf("Expected 1 error, got %d", len(es.Errors))
			}
			msg := es.Errors[0].Err.Error()
			t.Logf("Got error: %s", msg)
			if !strings.Contains(msg, tc.wantErr) {
				t.Fatalf("Expected error containing %q, got %q", tc.wantErr, msg)
			}

		})
	}
}
