package version

import (
	"fmt"
)

var (
	Version = "dev"
	Commit  = "unknown"
	BuildAt = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("%s+%s", Version, Commit)
}
