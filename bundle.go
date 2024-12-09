package postcards

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/jphastings/dotpostcard/formats"
)

// MakeBundles takes paths of files and directories and will figure out the bundles of files that need to be processed together.
// Subdirectories are not descended into.
func MakeBundles(filesAndDirPaths []string) ([]formats.Bundle, error) {
	groups, err := groupFiles(filesAndDirPaths)
	if err != nil {
		return nil, err
	}

	var bundles []formats.Bundle
	for _, group := range groups {
		for _, codecID := range Codecs {
			codec := codecs[codecID]
			newBundles, remaining, err := codec.Bundle(group)
			if err != nil {
				return bundles, err
			}

			bundles = append(bundles, newBundles...)
			group.Files = remaining
			if len(group.Files) == 0 {
				break
			}
		}
	}

	return bundles, nil
}

func groupFiles(inputPaths []string) ([]formats.FileGroup, error) {
	dirset := make(map[string]map[string]struct{})

	addToFiles := func(filepath string) {
		dir := path.Dir(filepath)
		base := path.Base(filepath)

		fileset, ok := dirset[dir]
		if !ok {
			fileset = make(map[string]struct{})
		}

		fileset[base] = struct{}{}
		dirset[dir] = fileset
	}

	for _, inputPath := range inputPaths {
		info, err := os.Stat(inputPath)
		if err != nil {
			return nil, fmt.Errorf("input path '%s' not usable: %w", inputPath, err)
		}
		if !info.IsDir() {
			addToFiles(inputPath)
		}
	}

	var groups []formats.FileGroup

	for dir, fileset := range dirset {
		dirFS := os.DirFS(dir)
		var fileFSs []fs.File
		for base, _ := range fileset {
			f, err := dirFS.Open(base)
			if err != nil {
				return nil, fmt.Errorf("file path '%s' not usable: %w", path.Join(dir, base), err)
			}
			fileFSs = append(fileFSs, f)
		}

		groups = append(groups, formats.FileGroup{
			Dir:     dirFS,
			Files:   fileFSs,
			DirPath: dir,
		})
	}

	return groups, nil
}

func allFilesInDir(dir string) ([]string, error) {
	var files []string

	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, de := range des {
		if !de.IsDir() {
			files = append(files, path.Join(dir, de.Name()))
		}
	}

	return files, err
}
