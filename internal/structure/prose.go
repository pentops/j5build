package structure

import (
	"fmt"
	"strings"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
)

func ResolveProse(source *source_j5pb.SourceImage, api *client_j5pb.API) error {
	//resolver ProseResolver, rootConfig *config_j5pb.Config, api *schema_j5pb.API) error {

	resolver := imageResolver(source.Prose)

	configsByName := map[string]*source_j5pb.PackageInfo{}
	for _, cfg := range source.Packages {
		configsByName[cfg.Name] = cfg
	}

	for _, pkg := range api.Packages {
		config, ok := configsByName[pkg.Name]
		if !ok {
			continue
		}
		var prose string
		if config.Prose != "" {
			resolved, err := resolver.ResolveProse(config.Prose)
			if err != nil {
				return fmt.Errorf("prose resolver: package %s: %w", pkg.Name, err)
			}
			prose = removeMarkdownHeader(resolved)
		}
		pkg.Prose = prose
	}
	return nil
}

type mapResolver map[string]string

func (mr mapResolver) ResolveProse(filename string) (string, error) {
	data, ok := mr[filename]
	if !ok {
		return "", fmt.Errorf("prose file %q not found", filename)
	}
	return data, nil
}

func removeMarkdownHeader(data string) string {
	// only look at the first 5 lines, that should be well enough to deal with
	// both title formats (# or \n===), and a few trailing empty lines

	lines := strings.SplitN(data, "\n", 5)
	if len(lines) == 0 {
		return ""
	}

	if len(lines) > 1 {
		if strings.HasPrefix(lines[0], "# ") {
			lines = lines[1:]
		} else if strings.HasPrefix(lines[1], "==") {
			lines = lines[2:]
		}
	}

	// Remove any leading empty lines
	for len(lines) > 1 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	return strings.Join(lines, "\n")
}

func imageResolver(proseFiles []*source_j5pb.ProseFile) mapResolver {
	mr := make(mapResolver)
	for _, proseFile := range proseFiles {
		mr[proseFile.Path] = string(proseFile.Content)
	}
	return mr
}
