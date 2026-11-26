package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/turgs/rstar-cli/internal/output"
	"github.com/turgs/rstar-cli/internal/release"
	"github.com/turgs/rstar-cli/internal/version"
)

const (
	codeUpgradeFailed     = "upgrade_failed"
	codeUpgradeRequired   = "upgrade_required"
	codeUpgradeIncomplete = "upgrade_incomplete"

	installerHint = "curl -fsSL https://raw.githubusercontent.com/turgs/rstar-cli/main/scripts/install.sh | bash"
)

// Seams for tests.
var (
	upgradeReleaseClient = func() *release.Client { return &release.Client{} }
	upgradeExecutable    = os.Executable
	upgradeProbeVersion  = probeBinaryVersion
	upgradeRunContext    = defaultUpgradeContext
)

func defaultUpgradeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade rstar to the latest GitHub release",
	Long: strings.TrimSpace(`
Check GitHub Releases for a newer rstar and upgrade when the install method allows it.

Install-script / tarball binaries under your home directory are replaced in place
after SHA-256 verification against checksums.txt on the same release.

go install, Homebrew, Scoop, Nix, and system-package installs are never mutated;
upgrade prints the command or page to use instead.

There is no background or silent auto-update on ordinary commands.
`),
	Args: cobra.NoArgs,
	RunE: runUpgrade,
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	current := version.Version
	progress := stdout
	if flagJSON {
		progress = stderr
	}

	if release.IsDevBuild(current) {
		data := map[string]any{
			"status":  "dev",
			"version": current,
			"method":  string(release.MethodDev),
		}
		summary := "Development build — upgrade not applicable (install a release or rebuild from source)"
		if flagJSON {
			return output.WriteSuccess(stdout, data, summary)
		}
		fmt.Fprintln(progress, summary)
		fmt.Fprintf(progress, "hint: %s\n", installerHint)
		return nil
	}

	fmt.Fprintf(progress, "Current version: %s\n", current)
	fmt.Fprint(progress, "Checking for updates… ")

	ctx, cancel := upgradeRunContext()
	defer cancel()

	client := upgradeReleaseClient()
	info, err := client.Latest(ctx)
	if err != nil {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("could not check for updates: %v", err),
			"Check network access to api.github.com and retry. In CI, set GITHUB_TOKEN to avoid anonymous rate limits.")
	}
	latest := info.Version()

	if release.NormalizeSemver(current) == "" {
		fmt.Fprintln(progress, "skipped")
		return upgradeErr(codeUpgradeRequired,
			fmt.Sprintf("current version %q is not a release semver; cannot compare to %s", current, latest),
			"Install a release build: "+installerHint)
	}
	if !release.IsUpdateAvailable(current, latest) {
		fmt.Fprintln(progress, "already up to date")
		data := map[string]any{"status": "up_to_date", "version": current, "latest": latest}
		summary := fmt.Sprintf("Already up to date (%s)", current)
		if flagJSON {
			return output.WriteSuccess(stdout, data, summary)
		}
		return nil
	}
	fmt.Fprintf(progress, "update available: %s\n", latest)

	exe, err := resolveUpgradeTarget()
	if err != nil {
		return upgradeErr(codeUpgradeRequired,
			fmt.Sprintf("update available (%s → %s) but the running executable could not be resolved: %v", current, latest, err),
			"Download manually: "+release.TagURL("", "", latest))
	}

	method := release.DetectInstallMethod(exe)
	if method != release.MethodUserNative {
		hint := release.MethodHint(method, latest)
		fmt.Fprintln(progress)
		fmt.Fprintf(progress, "Install method: %s — self-update is not applied.\n", method)
		fmt.Fprintf(progress, "  %s\n", hint)
		return upgradeErr(codeUpgradeRequired,
			fmt.Sprintf("update available (%s → %s) but this install method (%s) is not self-updated", current, latest, method),
			hint)
	}

	if err := ensureWritableDir(filepath.Dir(exe)); err != nil {
		return upgradeErr(codeUpgradeRequired,
			fmt.Sprintf("update available (%s → %s) but the install directory is not writable: %v", current, latest, err),
			"Fix permissions or reinstall: "+installerHint)
	}

	return runNativeUpgrade(ctx, client, info, exe, current, latest, progress)
}

func runNativeUpgrade(ctx context.Context, client *release.Client, info *release.Info, target, current, latest string, progress io.Writer) error {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	archiveName := release.ArchiveName(latest, goos, goarch)
	format := "tar.gz"
	if goos == "windows" {
		format = "zip"
	}
	binName := release.BinaryName(goos)

	archiveAsset, ok := info.AssetByName(archiveName)
	if !ok {
		fmt.Fprintf(progress, "No prebuilt binary for %s/%s\n", goos, goarch)
		return upgradeErr(codeUpgradeRequired,
			fmt.Sprintf("update available (%s → %s) but release v%s has no prebuilt binary for %s/%s", current, latest, latest, goos, goarch),
			"Download manually: "+release.TagURL("", "", latest))
	}
	checksumsAsset, ok := info.AssetByName("checksums.txt")
	if !ok {
		return upgradeErr(codeUpgradeFailed,
			fmt.Sprintf("release v%s is missing checksums.txt — cannot verify the download", latest),
			"The existing install was left in place.")
	}

	tmpDir, err := os.MkdirTemp("", "rstar-upgrade-")
	if err != nil {
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("could not create a download directory: %v", err), "")
	}
	defer os.RemoveAll(tmpDir)

	fmt.Fprintf(progress, "Downloading %s… ", archiveName)
	archivePath := filepath.Join(tmpDir, archiveName)
	archiveSHA, err := client.Download(ctx, archiveAsset.BrowserDownloadURL, archivePath, release.MaxArchiveBytes)
	if err != nil {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("download failed: %v", err), "Retry or reinstall: "+installerHint)
	}
	checksums, err := client.FetchBytes(ctx, checksumsAsset.BrowserDownloadURL, release.MaxChecksumBytes)
	if err != nil {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("could not download checksums.txt: %v", err), "")
	}
	fmt.Fprintln(progress, "done")

	fmt.Fprint(progress, "Verifying SHA-256… ")
	expected, found := release.ParseChecksums(checksums)[archiveName]
	if !found {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("checksums.txt has no entry for %s", archiveName), "")
	}
	if !strings.EqualFold(expected, archiveSHA) {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed,
			fmt.Sprintf("checksum mismatch for %s: expected %s, got %s", archiveName, expected, archiveSHA),
			"The existing install was left in place.")
	}
	fmt.Fprintln(progress, "ok")

	fmt.Fprint(progress, "Installing… ")
	// Stage next to the target so rename stays on the same filesystem.
	staged, err := os.CreateTemp(filepath.Dir(target), ".rstar-upgrade-")
	if err != nil {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("could not create staging file: %v", err), "")
	}
	stagedPath := staged.Name()
	_ = staged.Close()
	_ = os.Remove(stagedPath) // ExtractBinary creates exclusively
	defer os.Remove(stagedPath)

	if err := release.ExtractBinary(archivePath, format, binName, stagedPath); err != nil {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("could not stage the new binary: %v", err), "")
	}
	if infoStat, statErr := os.Stat(target); statErr == nil {
		_ = os.Chmod(stagedPath, infoStat.Mode().Perm())
	}

	if reported, probeErr := upgradeProbeVersion(ctx, stagedPath); probeErr == nil && !versionMatches(reported, latest) {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed,
			fmt.Sprintf("downloaded binary reports version %s, expected %s", reported, latest),
			"The existing install was left in place.")
	}

	backup, err := release.ReplaceExecutable(target, stagedPath)
	if err != nil {
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed, fmt.Sprintf("could not replace executable: %v", err),
			"The existing install should still be in place. Retry or reinstall: "+installerHint)
	}

	reported, probeErr := upgradeProbeVersion(ctx, target)
	if probeErr != nil || !versionMatches(reported, latest) {
		detail := fmt.Sprintf("reports %s, expected %s", reported, latest)
		if probeErr != nil {
			detail = fmt.Sprintf("could not be probed: %v", probeErr)
		}
		if restoreErr := release.RestoreBackup(target, backup); restoreErr != nil {
			fmt.Fprintln(progress, "failed")
			return upgradeErr(codeUpgradeIncomplete,
				fmt.Sprintf("installed binary %s, and restoring the previous version failed: %v", detail, restoreErr),
				fmt.Sprintf("Previous binary may be at %s — move it back to %s, or reinstall: %s", backup, target, installerHint))
		}
		fmt.Fprintln(progress, "failed")
		return upgradeErr(codeUpgradeFailed,
			fmt.Sprintf("installed binary %s — the previous version (%s) was restored", detail, current),
			"Retry or reinstall: "+installerHint)
	}
	release.DiscardBackup(backup)

	fmt.Fprintln(progress, "done")
	summary := fmt.Sprintf("Upgraded %s → %s", current, latest)
	fmt.Fprintln(progress, summary)
	data := map[string]any{
		"status": "upgraded",
		"from":   current,
		"to":     latest,
		"method": string(release.MethodUserNative),
		"path":   target,
	}
	if flagJSON {
		return output.WriteSuccess(stdout, data, summary)
	}
	return nil
}

func versionMatches(reported, expected string) bool {
	if strings.EqualFold(strings.TrimPrefix(reported, "v"), strings.TrimPrefix(expected, "v")) {
		return true
	}
	r, e := release.NormalizeSemver(reported), release.NormalizeSemver(expected)
	return r != "" && r == e
}

func resolveUpgradeTarget() (string, error) {
	exe, err := upgradeExecutable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Abs(exe)
}

func ensureWritableDir(dir string) error {
	f, err := os.CreateTemp(dir, ".rstar-upgrade-probe-")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func probeBinaryVersion(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, path, "version", "--json")
	cmd.Env = append(os.Environ(), "RSTAR_CONFIG_DIR="+os.TempDir())
	out, err := cmd.Output()
	if err == nil {
		if v, ok := versionFromJSON(out); ok {
			return v, nil
		}
	}
	cmd2 := exec.CommandContext(ctx, path, "version")
	cmd2.Env = cmd.Env
	out2, err2 := cmd2.Output()
	if err2 != nil {
		if err != nil {
			return "", err
		}
		return "", err2
	}
	return parseVersionLine(string(out2)), nil
}

func versionFromJSON(raw []byte) (string, bool) {
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", false
	}
	if env.Data.Version != "" {
		return env.Data.Version, true
	}
	return "", false
}

func parseVersionLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	fields := strings.Fields(s)
	if len(fields) >= 2 && fields[0] == "rstar" {
		return fields[1]
	}
	if len(fields) >= 1 {
		return fields[0]
	}
	return s
}

func upgradeErr(code, msg, hint string) error {
	ce := output.NewError(code, msg, output.ExitAPI)
	if hint != "" {
		ce = ce.WithHint(hint)
	}
	return ce
}
