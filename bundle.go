package postcards

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/jphastings/postcards/formats"
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
		files := group.Files
		for _, format := range Formats {
			newBundles, remaining, err := format.Bundle(group)
			if err != nil {
				return bundles, err
			}
			bundles = append(bundles, newBundles...)
			group.Files = remaining
			if len(files) == 0 {
				break
			}
		}
	}

	return bundles, nil
}

func groupFiles(inputPaths []string) (map[string]formats.FileGroup, error) {
	groups := make(map[string]formats.FileGroup)

	for _, inputPath := range inputPaths {
		info, err := os.Stat(inputPath)
		if err != nil {
			return nil, fmt.Errorf("input path '%s' not usable: %w", inputPath, err)
		}
		if info.IsDir() {
			dir := os.DirFS(inputPath)
			files, err := allfileGroup(dir)
			if err != nil {
				return nil, err
			}

			groups[inputPath] = formats.FileGroup{
				Dir:     dir,
				Files:   files,
				DirPath: inputPath,
			}
		} else {
			dirPath := path.Dir(inputPath)
			dir := os.DirFS(dirPath)
			file, err := dir.Open(path.Base(inputPath))
			if err != nil {
				return nil, err
			}

			if group, ok := groups[dirPath]; !ok {
				groups[inputPath] = formats.FileGroup{
					Dir:     dir,
					Files:   []fs.File{file},
					DirPath: dirPath,
				}
			} else {
				group.Files = append(groups[dirPath].Files, file)
				groups[dirPath] = group
			}
		}
	}

	return groups, nil
}

func allfileGroup(dir fs.FS) ([]fs.File, error) {
	var files []fs.File
	err := fs.WalkDir(dir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return fs.SkipDir
		}

		f, err := dir.Open(path)
		if err != nil {
			return fmt.Errorf("unable to open %s: %w", err)
		}
		files = append(files, f)
		return nil
	})
	return files, err
}
