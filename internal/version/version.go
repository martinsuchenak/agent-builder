// Package version holds build-time version metadata injected via -ldflags.
package version

// Version is the semantic version of the build (overridden at release time).
var Version = "dev"

// Date is the UTC build timestamp (overridden at release time).
var Date = "unknown"
