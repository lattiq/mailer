package mailer

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// Version information for the mailer library.
// These values are injected during build time via ldflags.
// The values below are fallbacks for development builds.
var (
	// Version is the semantic version of the library.
	Version = "dev"

	// GitCommit is the git commit hash when the binary was built.
	GitCommit = "unknown"

	// GitBranch is the git branch when the binary was built.
	GitBranch = "unknown"

	// BuildDate is the date when the binary was built.
	BuildDate = "unknown"

	// GoVersion is the version of Go used to build the binary.
	GoVersion = "unknown"
)

// VersionInfo contains detailed version information.
type VersionInfo struct {
	// Version is the semantic version of the library.
	Version string `json:"version"`

	// GitCommit is the git commit hash.
	GitCommit string `json:"git_commit"`

	// GitBranch is the git branch.
	GitBranch string `json:"git_branch"`

	// BuildDate is the build timestamp.
	BuildDate string `json:"build_date"`

	// GoVersion is the Go version used for building.
	GoVersion string `json:"go_version"`

	// Platform is the target platform (GOOS/GOARCH).
	Platform string `json:"platform"`

	// Module information from debug.BuildInfo.
	Module *ModuleInfo `json:"module,omitempty"`
}

// ModuleInfo contains Go module information.
type ModuleInfo struct {
	// Path is the module path.
	Path string `json:"path"`

	// Version is the module version.
	Version string `json:"version"`

	// Sum is the module checksum.
	Sum string `json:"sum"`

	// Replace information if the module is replaced.
	Replace *ModuleInfo `json:"replace,omitempty"`
}

// GetVersion returns the current version string.
func GetVersion() string {
	return Version
}

// GetVersionInfo returns detailed version information.
func GetVersionInfo() *VersionInfo {
	info := &VersionInfo{
		Version:   Version,
		GitCommit: GitCommit,
		GitBranch: GitBranch,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	// Try to get build info from runtime
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		// Update module information
		if buildInfo.Main.Path != "" {
			info.Module = &ModuleInfo{
				Path:    buildInfo.Main.Path,
				Version: buildInfo.Main.Version,
				Sum:     buildInfo.Main.Sum,
			}

			if buildInfo.Main.Replace != nil {
				info.Module.Replace = &ModuleInfo{
					Path:    buildInfo.Main.Replace.Path,
					Version: buildInfo.Main.Replace.Version,
					Sum:     buildInfo.Main.Replace.Sum,
				}
			}
		}

		// Extract additional build information from VCS info
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if GitCommit == "unknown" {
					info.GitCommit = setting.Value
					if len(info.GitCommit) > 12 {
						// Short commit hash for display
						GitCommit = info.GitCommit[:12]
					}
				}
			case "vcs.time":
				if BuildDate == "unknown" {
					if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
						info.BuildDate = t.Format("2006-01-02T15:04:05Z")
						BuildDate = info.BuildDate
					}
				}
			case "vcs.modified":
				if setting.Value == "true" && !strings.HasSuffix(info.GitCommit, "-dirty") {
					info.GitCommit += "-dirty"
				}
			}
		}
	}

	return info
}

// String returns a human-readable version string.
func (v *VersionInfo) String() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Version: %s", v.Version))

	if v.GitCommit != "unknown" && v.GitCommit != "" {
		parts = append(parts, fmt.Sprintf("Commit: %s", v.GitCommit))
	}

	if v.GitBranch != "unknown" && v.GitBranch != "" {
		parts = append(parts, fmt.Sprintf("Branch: %s", v.GitBranch))
	}

	if v.BuildDate != "unknown" && v.BuildDate != "" {
		parts = append(parts, fmt.Sprintf("Built: %s", v.BuildDate))
	}

	if v.GoVersion != "unknown" && v.GoVersion != "" {
		parts = append(parts, fmt.Sprintf("Go: %s", v.GoVersion))
	}

	if v.Platform != "/" {
		parts = append(parts, fmt.Sprintf("Platform: %s", v.Platform))
	}

	return strings.Join(parts, ", ")
}

// UserAgent returns a user agent string for HTTP requests.
func (v *VersionInfo) UserAgent() string {
	return fmt.Sprintf("lattiq-mailer/%s (%s)", v.Version, v.Platform)
}

// IsDevBuild returns true if this is a development build.
func (v *VersionInfo) IsDevBuild() bool {
	return strings.Contains(v.Version, "dev") ||
		strings.Contains(v.Version, "snapshot") ||
		strings.HasSuffix(v.GitCommit, "-dirty") ||
		v.GitCommit == "unknown"
}

// SemVer represents a semantic version.
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
}

// ParseSemVer parses a semantic version string.
func ParseSemVer(version string) (*SemVer, error) {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	var sv SemVer
	var err error

	// Split on '+' to separate build metadata
	parts := strings.SplitN(version, "+", 2)
	if len(parts) == 2 {
		sv.Build = parts[1]
	}
	version = parts[0]

	// Split on '-' to separate pre-release
	parts = strings.SplitN(version, "-", 2)
	if len(parts) == 2 {
		sv.PreRelease = parts[1]
	}
	version = parts[0]

	// Parse major.minor.patch
	versionParts := strings.Split(version, ".")
	if len(versionParts) != 3 {
		return nil, fmt.Errorf("invalid semantic version format: %s", version)
	}

	if sv.Major, err = parseInt(versionParts[0]); err != nil {
		return nil, fmt.Errorf("invalid major version: %w", err)
	}

	if sv.Minor, err = parseInt(versionParts[1]); err != nil {
		return nil, fmt.Errorf("invalid minor version: %w", err)
	}

	if sv.Patch, err = parseInt(versionParts[2]); err != nil {
		return nil, fmt.Errorf("invalid patch version: %w", err)
	}

	return &sv, nil
}

// String returns the string representation of the semantic version.
func (sv *SemVer) String() string {
	version := fmt.Sprintf("%d.%d.%d", sv.Major, sv.Minor, sv.Patch)

	if sv.PreRelease != "" {
		version += "-" + sv.PreRelease
	}

	if sv.Build != "" {
		version += "+" + sv.Build
	}

	return version
}

// Compare compares two semantic versions.
// Returns -1 if sv < other, 0 if sv == other, 1 if sv > other.
func (sv *SemVer) Compare(other *SemVer) int {
	// Compare major version
	if sv.Major < other.Major {
		return -1
	}
	if sv.Major > other.Major {
		return 1
	}

	// Compare minor version
	if sv.Minor < other.Minor {
		return -1
	}
	if sv.Minor > other.Minor {
		return 1
	}

	// Compare patch version
	if sv.Patch < other.Patch {
		return -1
	}
	if sv.Patch > other.Patch {
		return 1
	}

	// Compare pre-release versions
	// No pre-release has higher precedence than pre-release
	if sv.PreRelease == "" && other.PreRelease != "" {
		return 1
	}
	if sv.PreRelease != "" && other.PreRelease == "" {
		return -1
	}

	// Both have pre-release, compare lexically
	if sv.PreRelease < other.PreRelease {
		return -1
	}
	if sv.PreRelease > other.PreRelease {
		return 1
	}

	return 0
}

// IsCompatible checks if the version is compatible with the given version.
// Compatible means same major version and greater or equal minor.patch.
func (sv *SemVer) IsCompatible(other *SemVer) bool {
	return sv.Major == other.Major && sv.Compare(other) >= 0
}

// Helper function to parse integer safely
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	var result int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid character: %c", r)
		}
		result = result*10 + int(r-'0')
	}

	return result, nil
}

// PrintVersion prints version information to stdout.
func PrintVersion() {
	info := GetVersionInfo()
	fmt.Println("Lattiq Mailer Library")
	fmt.Println(info.String())
	if info.Module != nil {
		fmt.Printf("Module: %s@%s\n", info.Module.Path, info.Module.Version)
	}
}
