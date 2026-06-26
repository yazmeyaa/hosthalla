package version

import (
	"fmt"
	"log"
)

var (
	Version = "dev"
	Commit  = "unknown"
	BuildAt = "unknown"
)

func VersionString() string {
	log.Println("Version: ", Version)
	log.Println("Commit: ", Commit)
	log.Println("BuildAt: ", BuildAt)
	return fmt.Sprintf("%s-%s-%s", Version, Commit, BuildAt)
}
