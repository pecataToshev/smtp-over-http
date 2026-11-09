// Package buildinfo holds build metadata.
// These dev defaults are overwritten by the deploy workflow
// before build / deploy.
package buildinfo

const (
	Commit    = "dev"
	BuildTime = "now"
	Source    = "local"
)
