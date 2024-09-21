package builder

import (
	"fmt"
	"strings"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/structure"
	"google.golang.org/protobuf/types/descriptorpb"
)

// MutateImageWithMods applies a set of image mods to a source image.
// This is a minor foot gun, hence the name.
// The caller is responsible for ensuring that the image is fresh and not reused
// if that isn't desired. Adding a clone protection is too expensive for the
// likely image sizes, and all existing use cases only use the image once.
func MutateImageWithMods(img *source_j5pb.SourceImage, mods []*config_j5pb.ImageMod) error {
	for _, mod := range mods {
		switch mod := mod.Type.(type) {
		case *config_j5pb.ImageMod_GoPackageNames_:
			if err := runGoPackageNamesMod(img, mod.GoPackageNames); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown image mod type: %T", mod)
		}
	}

	return nil
}

func runGoPackageNamesMod(img *source_j5pb.SourceImage, mod *config_j5pb.ImageMod_GoPackageNames) error {

	isSource := make(map[string]bool)
	for _, file := range img.SourceFilenames {
		isSource[file] = true
	}

	for _, file := range img.File {
		if !isSource[*file.Name] {
			continue
		}
		if file.Options == nil {
			file.Options = &descriptorpb.FileOptions{}
		}

		pkg := *file.Package
		for _, prefix := range mod.TrimPrefixes {
			if strings.HasPrefix(pkg, prefix) {
				pkg = pkg[len(prefix):]
				pkg = strings.TrimPrefix(pkg, ".")
				break
			}
		}
		basePkg, subPkg, err := structure.SplitPackageParts(pkg)
		if err != nil {
			return fmt.Errorf("splitting package %q: %w", pkg, err)
		}

		suffix := "_pb"
		if subPkg != nil {
			sub := *subPkg
			suffix = fmt.Sprintf("_%spb", sub[0:1])
		}
		if modSuffix, ok := mod.Suffixes[basePkg]; ok {
			suffix = modSuffix
		}
		pkgParts := strings.Split(basePkg, ".")
		baseName := strings.Join(pkgParts, "/")
		namePart := pkgParts[len(pkgParts)-2]
		lastPart := fmt.Sprintf("%s%s", namePart, suffix)
		fullName := strings.Join([]string{mod.Prefix, baseName, lastPart}, "/")
		file.Options.GoPackage = &fullName
	}
	return nil
}
