package version

// Version information for Lightning Code Index
const (
	// Version is the current semantic version of LCI
	Version = "0.4.0"

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
