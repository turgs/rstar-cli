package release

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ParseChecksums parses a goreleaser checksums.txt into name → lowercase hex.
// Lines look like: "<sha256>  <filename>" (two spaces) or "<sha256> *<filename>".
func ParseChecksums(data []byte) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		name := fields[len(fields)-1]
		name = strings.TrimPrefix(name, "*")
		// strip directory prefixes if present
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if len(sum) == 64 {
			out[name] = sum
		}
	}
	return out
}

func copySHA256(dst io.Writer, src io.Reader, limit int64) (string, error) {
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(dst, h), src)
	if err != nil {
		return "", err
	}
	if n > limit {
		return "", fmt.Errorf("download larger than %d bytes", limit)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFile returns the SHA-256 hex digest of path.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
