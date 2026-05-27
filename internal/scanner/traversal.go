package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

var sourceExts = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true,
	".json": true,
	".sh": true, ".bash": true,
	".py": true,
}

var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".safeskill":   true,
}

const defaultMaxSize = 1 << 20

func Walk(root string) ([]string, error) {
	return WalkWithLimit(root, defaultMaxSize)
}

func WalkWithLimit(root string, maxSize int64) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !sourceExts[ext] {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxSize {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}
