package proxy

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxExtractDepth = 10
	maxExtractSize  = 1 << 20
	maxTotalExtract = 50 << 20
)

func ExtractTarball(r io.Reader, dest string) ([]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var files []string
	var totalWritten int64

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}

		target := filepath.Join(dest, filepath.Clean(hdr.Name))
		if !isSubPath(dest, target) {
			continue
		}

		if relDepth(dest, target) > maxExtractDepth {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return nil, fmt.Errorf("mkdir %s: %w", target, err)
			}

		case tar.TypeReg:
			if hdr.Size > maxExtractSize {
				continue
			}
			if totalWritten+hdr.Size > maxTotalExtract {
				continue
			}

			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, fmt.Errorf("mkdir parent %s: %w", target, err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return nil, fmt.Errorf("create %s: %w", target, err)
			}

			written, err := io.CopyN(f, tr, hdr.Size)
			f.Close()
			if err != nil {
				return nil, fmt.Errorf("write %s: %w", target, err)
			}
			totalWritten += written
			files = append(files, target)

		case tar.TypeSymlink:
			linkTarget := hdr.Linkname
			resolved := filepath.Join(filepath.Dir(target), linkTarget)
			if !isSubPath(dest, resolved) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, fmt.Errorf("mkdir parent for symlink %s: %w", target, err)
			}
			if err := os.Symlink(linkTarget, target); err != nil {
				return nil, fmt.Errorf("symlink %s -> %s: %w", target, linkTarget, err)
			}
		}
	}

	return files, nil
}

func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func relDepth(parent, child string) int {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return -1
	}
	clean := filepath.Clean(rel)
	if clean == "." {
		return 0
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return -1
	}
	return strings.Count(clean, string(filepath.Separator)) + 1
}
