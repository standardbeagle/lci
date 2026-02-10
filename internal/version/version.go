package version

import (
	"crypto/sha256"
	"fmt"
	"runtime/debug"
	"sync"
)

// Version information for Lightning Code Index
const (
	// Version is the current semantic version of LCI
	Version = "0.4.1"

	// BuildDate is set during build time (use -ldflags)
	BuildDate = "development"

	// GitCommit is set during build time (use -ldflags)
	GitCommit = "unknown"
)

// Info returns version information as a string
func Info() string {
	return Version
}

// FullInfo returns detailed version information
func FullInfo() string {
	return "Lightning Code Index " + Version + " (commit: " + GitCommit + ", built: " + BuildDate + ")"
}

var (
	buildID     string
	buildIDOnce sync.Once
)

// BuildID returns a fingerprint of the current binary build.
// It hashes Go version, module path/version, and VCS build settings
// so that ensureServerRunning can detect stale servers from old builds.
func BuildID() string {
	buildIDOnce.Do(func() {
		buildID = computeBuildID()
	})
	return buildID
}

func computeBuildID() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version + "-" + GitCommit
	}

	h := sha256.New()
	h.Write([]byte(info.GoVersion))
	h.Write([]byte(info.Main.Path))
	h.Write([]byte(info.Main.Version))

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision", "vcs.modified", "vcs.time":
			h.Write([]byte(s.Key))
			h.Write([]byte(s.Value))
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
