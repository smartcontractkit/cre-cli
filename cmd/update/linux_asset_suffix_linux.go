//go:build linux

package update

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const (
	linuxLdd235Suffix   = "_ldd2-35"
	linuxGlibcThreshold = "2.36"
)

var glibcVersionPattern = regexp.MustCompile(`(\d+\.\d+)(?:\.\d+)?`)

func linuxAssetSuffix() string {
	out, err := exec.Command("ldd", "--version").Output() // #nosec G204 -- fixed args, standard glibc probe
	if err != nil {
		return ""
	}

	version, err := parseGlibcVersionFromLddOutput(string(out))
	if err != nil {
		return ""
	}

	threshold, err := semver.NewVersion(linuxGlibcThreshold)
	if err != nil {
		return ""
	}
	if version.LessThan(threshold) {
		return linuxLdd235Suffix
	}
	return ""
}

func parseGlibcVersionFromLddOutput(output string) (*semver.Version, error) {
	firstLine := strings.TrimSpace(output)
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	if firstLine == "" {
		return nil, fmt.Errorf("empty ldd --version output")
	}

	matches := glibcVersionPattern.FindAllString(firstLine, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("could not parse glibc version from %q", firstLine)
	}

	version, err := semver.NewVersion(matches[len(matches)-1])
	if err != nil {
		return nil, fmt.Errorf("parse glibc version %q: %w", matches[len(matches)-1], err)
	}
	return version, nil
}
