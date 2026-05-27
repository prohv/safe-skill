package proxy

import (
	"net/http"
	"strings"
)

var tarballExtensions = []string{".tgz", ".tar.gz"}

var tarballContentTypes = map[string]bool{
	"application/gzip":            true,
	"application/x-gzip":          true,
	"application/x-tar":           true,
	"application/octet-stream":    true,
	"application/x-compressed":    true,
	"application/gzip-compressed": true,
}

func isTarballURL(path string) bool {
	p := strings.ToLower(path)
	for _, ext := range tarballExtensions {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

func isTarballContent(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return tarballContentTypes[strings.ToLower(ct)]
}

func packageNameFromURL(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && strings.HasPrefix(parts[0], "@") {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) >= 1 {
		return parts[0]
	}
	return path
}
