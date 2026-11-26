package release

import (
	"strconv"
	"strings"
)

// NormalizeSemver turns a version string into a comparable canonical form
// without a leading "v". Dev / empty / non-numeric major.minor.patch return "".
func NormalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" || strings.HasSuffix(v, "-dev") {
		return ""
	}
	v = strings.TrimPrefix(v, "v")
	core, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return ""
	}
	nums := make([]int, 3)
	for i := 0; i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return ""
		}
		nums[i] = n
	}
	out := strconv.Itoa(nums[0]) + "." + strconv.Itoa(nums[1]) + "." + strconv.Itoa(nums[2])
	if hasPre && pre != "" {
		out += "-" + pre
	}
	return out
}

// IsUpdateAvailable reports whether latest is strictly newer than current.
// Non-semver either side returns false.
func IsUpdateAvailable(current, latest string) bool {
	c, l := NormalizeSemver(current), NormalizeSemver(latest)
	if c == "" || l == "" {
		return false
	}
	return compareSemver(c, l) < 0
}

// IsDevBuild reports versions that should not self-update.
func IsDevBuild(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" || v == "0.0.0-dev" {
		return true
	}
	return strings.HasSuffix(v, "-dev")
}

// compareSemver returns -1 if a<b, 0 if equal, 1 if a>b.
// Pre-release (suffix after -) is older than the same core without pre.
func compareSemver(a, b string) int {
	aCore, aPre, aHas := strings.Cut(a, "-")
	bCore, bPre, bHas := strings.Cut(b, "-")
	if c := compareCore(aCore, bCore); c != 0 {
		return c
	}
	switch {
	case !aHas && !bHas:
		return 0
	case !aHas && bHas:
		return 1 // release > pre-release
	case aHas && !bHas:
		return -1
	default:
		return strings.Compare(aPre, bPre)
	}
}

func compareCore(a, b string) int {
	ap := splitInts(a)
	bp := splitInts(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func splitInts(core string) [3]int {
	var out [3]int
	parts := strings.Split(core, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}
