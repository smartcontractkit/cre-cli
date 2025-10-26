package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rs/zerolog" // <-- ADDED IMPORT
)

const (
	githubAPIURL  = "https://api.github.com/repos/smartcontractkit/cre-cli/releases/latest"
	repoURL       = "https://github.com/smartcontractkit/cre-cli/releases"
	timeout       = 2 * time.Second
	cacheDuration = 24 * time.Hour
	cacheFileName = "update.json"
	cacheDirName  = ".cre"
)

// Logger interface is removed. We now use zerolog.Logger directly.

// githubRelease is a minimal struct to parse the JSON response
// from the GitHub releases API.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// cacheState stores the data for our update check cache.
type cacheState struct {
	LatestVersion string    `json:"latest_version"`
	LastCheck     time.Time `json:"last_check"`
}

// getCachePath returns the platform-specific path to the cache file.
// This now uses ~/.cre/update.json as requested.
func getCachePath(logger *zerolog.Logger) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Debug().Msgf("Failed to get user home directory: %v", err)
		return "", err
	}
	return filepath.Join(homeDir, cacheDirName, cacheFileName), nil
}

// loadCache reads the cache file from disk.
func loadCache(path string, logger *zerolog.Logger) (*cacheState, error) {
	logger.Debug().Msgf("Loading cache from %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug().Msg("Cache file not found.")
			return &cacheState{}, nil // Return empty state, not an error
		}
		return nil, err
	}

	var state cacheState
	if err := json.Unmarshal(data, &state); err != nil {
		logger.Debug().Msgf("Cache file corrupted, ignoring: %v", err)
		// Return empty state, not an error, so we can overwrite it
		return &cacheState{}, nil
	}

	logger.Debug().Msgf("Cache loaded. Last check: %v, Latest version: %s", state.LastCheck, state.LatestVersion)
	return &state, nil
}

// saveCache writes the cache state to disk.
func saveCache(path string, state cacheState, logger *zerolog.Logger) error {
	logger.Debug().Msgf("Saving cache to %s", path)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Ensure the directory ~/.cre exists
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// fetchLatestVersionFromGitHub performs the actual network request.
func fetchLatestVersionFromGitHub(logger *zerolog.Logger) (string, error) {
	client := &http.Client{
		Timeout: timeout,
	}

	logger.Debug().Msgf("Fetching latest release from %s", githubAPIURL)
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "cre-cli-update-check")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API returned non-200 status: %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode GitHub API response: %w", err)
	}

	if release.TagName == "" {
		return "", errors.New("github API response contained no tag_name")
	}

	logger.Debug().Msgf("Latest release tag found: %s", release.TagName)
	return release.TagName, nil
}

// CheckForUpdates fetches the latest release from GitHub and compares it
// to the current version. If a newer version is found, it prints a
// message to os.Stderr.
// This function is designed to be run in a goroutine so it doesn't
// block the main CLI execution.
// It now accepts your application's logger.
func CheckForUpdates(currentVersion string, logger *zerolog.Logger) {
	// --- TESTING HOOK ---
	// Allow forcing the check even for "development" version
	forceCheck := os.Getenv("CRE_FORCE_UPDATE_CHECK") == "1"
	if currentVersion == "development" && !forceCheck {
		logger.Debug().Msg("Current version is 'development', skipping update check. (Set CRE_FORCE_UPDATE_CHECK=1 to override)")
		return
	}
	// --- END TESTING HOOK ---

	currentSemVer, err := semver.NewVersion(currentVersion)
	if err != nil {
		logger.Debug().Msgf("Failed to parse current version '%s': %v", currentVersion, err)
		return
	}
	logger.Debug().Msgf("Current version parsed as: %s", currentSemVer.String())

	cachePath, err := getCachePath(logger)
	if err != nil {
		logger.Debug().Msgf("Failed to get cache path: %v", err)
		return // Non-critical, just skip the check
	}

	cache, err := loadCache(cachePath, logger)
	if err != nil {
		logger.Debug().Msgf("Failed to load cache: %v", err)
		// Non-critical, just skip
	}
	if cache == nil {
		cache = &cacheState{}
	}

	now := time.Now()
	needsCheck := now.Sub(cache.LastCheck) > cacheDuration
	latestVersionString := cache.LatestVersion

	if needsCheck {
		logger.Debug().Msg("Cache expired or empty. Fetching from GitHub.")
		newLatestVersion, fetchErr := fetchLatestVersionFromGitHub(logger)
		if fetchErr != nil {
			logger.Debug().Msgf("Failed to fetch latest version: %v", fetchErr)
			// Don't update cache, just use stale data (if any)
		} else {
			logger.Debug().Msgf("Fetched new latest version: %s", newLatestVersion)
			latestVersionString = newLatestVersion
			cache.LatestVersion = newLatestVersion
			cache.LastCheck = now
			if err := saveCache(cachePath, *cache, logger); err != nil {
				logger.Debug().Msgf("Failed to save cache: %v", err)
			}
		}
	} else {
		logger.Debug().Msgf("Using cached latest version: %s", latestVersionString)
	}

	// Always compare, even with cached data
	if latestVersionString == "" {
		logger.Debug().Msg("No latest version available to compare.")
		return
	}

	latestSemVer, err := semver.NewVersion(latestVersionString)
	if err != nil {
		logger.Debug().Msgf("Failed to parse latest tag '%s' (from cache or fetch): %v", latestVersionString, err)
		return
	}

	// Check if the latest version is greater than the current one
	if latestSemVer.GreaterThan(currentSemVer) {
		// Print to Stderr so it doesn't interfere with command stdout (e.g., piping)
		fmt.Fprintf(os.Stderr,
			"\n⚠️  Update available! You’re running %s, but %s is the latest.\n"+
				"Run `cre update` or visit %s to upgrade.\n\n",
			currentSemVer.String(),
			latestSemVer.String(),
			repoURL,
		)
	} else {
		logger.Debug().Msgf("Current version %s is up-to-date.", currentSemVer.String())
	}
}
