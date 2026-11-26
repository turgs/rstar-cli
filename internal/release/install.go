package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// MaxArchiveBytes caps a downloaded release archive.
	MaxArchiveBytes = 200 << 20
	// MaxChecksumBytes caps checksums.txt.
	MaxChecksumBytes = 1 << 20
	// MaxBinaryBytes caps the uncompressed binary extracted from an archive.
	MaxBinaryBytes = 300 << 20
)

// ExtractBinary pulls binName out of a tar.gz or zip archive into destPath (exclusive).
func ExtractBinary(archivePath, format, binName, destPath string) error {
	switch format {
	case "tar.gz", "tgz":
		return extractTarGz(archivePath, binName, destPath)
	case "zip":
		return extractZip(archivePath, binName, destPath)
	default:
		return fmt.Errorf("unsupported archive format %q", format)
	}
}

func extractTarGz(archivePath, binName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != binName {
			continue
		}
		if hdr.Size > MaxBinaryBytes {
			return fmt.Errorf("binary larger than %d bytes", MaxBinaryBytes)
		}
		return writeExclusive(destPath, io.LimitReader(tr, MaxBinaryBytes+1), MaxBinaryBytes, 0o755)
	}
	return fmt.Errorf("archive does not contain %s", binName)
}

func extractZip(archivePath, binName, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(zf.Name) != binName {
			continue
		}
		if int64(zf.UncompressedSize64) > MaxBinaryBytes {
			return fmt.Errorf("binary larger than %d bytes", MaxBinaryBytes)
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeExclusive(destPath, io.LimitReader(rc, MaxBinaryBytes+1), MaxBinaryBytes, 0o755)
		_ = rc.Close()
		return err
	}
	return fmt.Errorf("archive does not contain %s", binName)
}

func writeExclusive(dest string, r io.Reader, limit int64, mode os.FileMode) error {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return copyErr
	}
	if n > limit {
		_ = os.Remove(dest)
		return fmt.Errorf("extracted binary larger than %d bytes", limit)
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return closeErr
	}
	return os.Chmod(dest, mode)
}

// ReplaceExecutable swaps staged into target. On Unix, rename is atomic on the same filesystem.
// On Windows, rename over a running executable may fail; the previous file is moved aside first.
// Returns the backup path (may be empty if no prior file).
func ReplaceExecutable(target, staged string) (backup string, err error) {
	if runtime.GOOS == "windows" {
		return replaceWindows(target, staged)
	}
	return replaceUnix(target, staged)
}

func replaceUnix(target, staged string) (string, error) {
	backup := target + ".rstar-upgrade-bak"
	_ = os.Remove(backup)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		hadTarget = true
		if err := moveFile(target, backup); err != nil {
			return "", fmt.Errorf("could not move current binary aside: %w", err)
		}
	}
	if err := moveFile(staged, target); err != nil {
		if hadTarget {
			_ = moveFile(backup, target)
		}
		return "", fmt.Errorf("could not install new binary: %w", err)
	}
	if hadTarget {
		return backup, nil
	}
	return "", nil
}

// moveFile renames src to dst, falling back to copy+remove across filesystems.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	return os.Remove(src)
}

func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cross-device") || strings.Contains(msg, "exdev")
}

func replaceWindows(target, staged string) (string, error) {
	// Same strategy: move target aside, then move staged into place.
	return replaceUnix(target, staged)
}

// RestoreBackup puts backup back onto target and removes backup when successful.
func RestoreBackup(target, backup string) error {
	if backup == "" {
		return fmt.Errorf("no backup to restore")
	}
	_ = os.Remove(target)
	return moveFile(backup, target)
}

// DiscardBackup removes a successful-upgrade backup.
func DiscardBackup(backup string) {
	if backup != "" {
		_ = os.Remove(backup)
	}
}

// PathWithin reports whether target is inside root after resolving symlinks.
func PathWithin(rootDir, target string) bool {
	canonRoot, err := canonicalPath(rootDir)
	if err != nil {
		return false
	}
	canonTarget, err := canonicalPath(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(canonRoot, canonTarget)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func canonicalPath(p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		// If the leaf does not exist yet, resolve the parent.
		dir, base := filepath.Dir(p), filepath.Base(p)
		resolvedDir, err2 := filepath.EvalSymlinks(dir)
		if err2 != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(resolvedDir, base))
	}
	return filepath.Abs(resolved)
}

// DetectMethod classifies how the running binary was likely installed.
type Method string

const (
	MethodDev        Method = "dev"
	MethodHomebrew   Method = "homebrew"
	MethodScoop      Method = "scoop"
	MethodGoInstall  Method = "go_install"
	MethodNix        Method = "nix"
	MethodSystem     Method = "system"
	MethodUserNative Method = "native" // under $HOME, self-update eligible
)

// DetectInstallMethod inspects the resolved executable path.
func DetectInstallMethod(exe string) Method {
	n := filepath.ToSlash(strings.ToLower(exe))
	switch {
	case strings.Contains(n, "/homebrew/") || strings.Contains(n, "/linuxbrew/") ||
		strings.Contains(n, "/cellar/") || strings.Contains(n, "/caskroom/"):
		return MethodHomebrew
	case strings.Contains(n, "/scoop/apps/") || strings.Contains(n, "/scoop/shims/"):
		return MethodScoop
	case strings.Contains(n, "/nix/store/"):
		return MethodNix
	case strings.Contains(n, "/go/bin/") || strings.HasSuffix(n, "/gopath/bin/rstar") ||
		strings.HasSuffix(n, "/gopath/bin/rstar.exe"):
		return MethodGoInstall
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" && PathWithin(home, exe) {
		// go install often lands in ~/go/bin
		if strings.Contains(n, "/go/bin/") {
			return MethodGoInstall
		}
		return MethodUserNative
	}
	return MethodSystem
}

// MethodHint returns guidance when self-update is not appropriate.
func MethodHint(m Method, latest string) string {
	latest = strings.TrimPrefix(latest, "v")
	switch m {
	case MethodHomebrew:
		return "Homebrew installs are not self-updated yet. Reinstall from releases or wait for a tap; see https://github.com/turgs/rstar-cli/releases"
	case MethodScoop:
		return "Scoop installs are not self-updated yet. Download the latest release manually or reinstall from https://github.com/turgs/rstar-cli/releases"
	case MethodGoInstall:
		return "Run: go install github.com/turgs/rstar-cli/cmd/rstar@latest"
	case MethodNix:
		return "This binary is in the Nix store. Upgrade via your Nix flake/profile pin."
	case MethodSystem:
		return "This binary is outside your home directory. Reinstall with the install script under your home, or replace it with the package that owns it: curl -fsSL https://raw.githubusercontent.com/turgs/rstar-cli/main/scripts/install.sh | bash"
	case MethodDev:
		return "Development builds are not upgraded in place. Build from source or install a release binary."
	default:
		return "Download: " + TagURL("", "", latest)
	}
}
