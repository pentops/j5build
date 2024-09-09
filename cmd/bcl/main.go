package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/runner/commander"
	"google.golang.org/protobuf/encoding/prototext"
)

var Version = "dev"

func main() {
	cmdGroup := commander.NewCommandSet()
	cmdGroup.Add("lint", commander.NewCommand(runLint))
	//cmdGroup.Add("fmt", commander.NewCommand(runFmt))
	//cmdGroup.Add("lsp", commander.NewCommand(runLSP))
	cmdGroup.RunMain("bcl", Version)
}

type RootConfig struct {
	ProjectRoot string `flag:"project-root" default:"" desc:"Project root directory"`
	Verbose     bool   `flag:"verbose" env:"BCL_VERBOSE" default:"false" desc:"Verbose output"`
}

//func (rc *RootConfig) SchemaConfig() *bcl_j5pb.Schema {

//}

func runLint(ctx context.Context, cfg struct {
	RootConfig
	Filename string `flag:"filename" desc:"Filename to lint"`
}) error {

	schemaSpec := &bcl_j5pb.Schema{
		Blocks: []*bcl_j5pb.Block{{
			SchemaName: "j5.bcl.v1.Block",
			Name: &bcl_j5pb.Tag{
				Path: &bcl_j5pb.Path{
					Path: []string{"schemaName"},
				},
			},
			Children: []*bcl_j5pb.Child{{
				Name: "schemaName",
				Path: &bcl_j5pb.Path{
					Path: []string{"schemaName"},
				},
				IsScalar: true,
			}, {
				Name: "name",
				Path: &bcl_j5pb.Path{
					Path: []string{"name"},
				},
			}},
		}, {
			SchemaName: "j5.bcl.v1.ScalarSplit",
			Children: []*bcl_j5pb.Child{{
				Name: "required",
				Path: &bcl_j5pb.Path{Path: []string{"requiredFields", "path"}},
			}, {
				Name: "optional",
				Path: &bcl_j5pb.Path{Path: []string{"optionalFields", "path"}},
			}, {
				Name: "remainder",
				Path: &bcl_j5pb.Path{Path: []string{"remainderField", "path"}},
			}},
		}, {
			SchemaName: "j5.bcl.v1.Tag",
			Children: []*bcl_j5pb.Child{{
				Name: "path",
				Path: &bcl_j5pb.Path{
					Path: []string{"path", "path"},
				},
				IsScalar:     true,
				IsCollection: true,
			}},
		}},
	}

	msg := &bcl_j5pb.SchemaFile{}
	/*
		//sc := j5schema.NewSchemaCache()
			rootSchema, err := sc.Schema(msg.ProtoReflect().Descriptor())
			if err != nil {
				return err
			}*/

	parser, err := bcl.NewParser(schemaSpec) //rootSchema.(*j5schema.ObjectSchema))
	if err != nil {
		return err
	}
	parser.Verbose = cfg.Verbose

	content, err := os.ReadFile(cfg.Filename)
	if err != nil {
		return err
	}

	_, mainError := parser.ParseFile(cfg.Filename, string(content), msg.ProtoReflect())
	if mainError == nil {
		fmt.Println(prototext.Format(msg))
		return nil
	}

	locErr, ok := errpos.AsErrorsWithSource(mainError)
	if !ok {
		return mainError
	}

	log.Println(locErr.HumanString(2))

	os.Exit(100)
	return nil
}

/*
func runFmt(ctx context.Context, cfg struct {
	Dir   string `flag:"dir" default:"." desc:"Root schema directory"`
	Write bool   `flag:"write" default:"false" desc:"Write fixes to files"`
}) error {

	doFile := func(pathname string, data []byte) (string, error) {
		tree, err := ast.ParseFile(string(data), true)
		if err != nil {
			if err == ast.HadErrors {
				return "", errpos.AddSource(tree.Errors, string(data))
			}
			return "", fmt.Errorf("parse file not HadErrors: %w", err)
		}

		fixed := ast.Print(tree)
		return fixed, nil
	}

	stat, err := os.Lstat(cfg.Dir)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		data, err := os.ReadFile(cfg.Dir)
		if err != nil {
			return err
		}
		out, err := doFile(cfg.Dir, data)
		if err != nil {
			return err
		}
		if !cfg.Write {
			fmt.Printf("Fixed: %s\n", cfg.Dir)
			fmt.Println(out)
		} else {
			return os.WriteFile(cfg.Dir, []byte(out), 0644)
		}
		return nil
	}

	outWriter := &fileWriter{dir: cfg.Dir}
	root := os.DirFS(cfg.Dir)
	err = fs.WalkDir(root, ".", func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if path.Ext(pathname) != ".j5s" {
			return nil
		}

		data, err := fs.ReadFile(root, pathname)
		if err != nil {
			return err
		}

		out, err := doFile(pathname, data)
		if err != nil {
			return err
		}
		if !cfg.Write {
			fmt.Printf("Fixed: %s\n", pathname)
			fmt.Println(out)
			return nil
		} else {
			return outWriter.PutFile(ctx, pathname+".fied", []byte(out))
		}
	})
	if err != nil {
		return err
	}
	return nil
}

type fileWriter struct {
	dir string
}

func (f *fileWriter) PutFile(ctx context.Context, filename string, data []byte) error {
	dir := path.Join(f.dir, path.Dir(filename))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path.Join(f.dir, filename), data, 0644)
}

type fileReader struct {
	fs       fs.FS
	packages []string
}

func (rr *fileReader) GetLocalFile(ctx context.Context, filename string) ([]byte, error) {
	return fs.ReadFile(rr.fs, filename)
}

func (rr *fileReader) ListPackages() []string {
	return rr.packages
}

func (rr *fileReader) ListSourceFiles(ctx context.Context, pkgName string) ([]string, error) {
	pkgRoot := strings.ReplaceAll(pkgName, ".", "/")

	files := make([]string, 0)
	err := fs.WalkDir(rr.fs, pkgRoot, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".j5gen.proto") {
			return nil
		}
		if strings.HasSuffix(path, ".proto") {
			files = append(files, path)
		}
		if strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (rr *fileReader) ListJ5Files(ctx context.Context) ([]string, error) {
	files := make([]string, 0)
	err := fs.WalkDir(rr.fs, ".", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil

}*/
