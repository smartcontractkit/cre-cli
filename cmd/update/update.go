package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	osruntime "runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/version"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

const (
	repo           = "smartcontractkit/cre-cli"
	cliName        = "cre"
	maxExtractSize = 500 * 1024 * 1024
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type releaseInfo struct {
	TagName string `json:"tag_name"`
}

func getLatestTag() (string, error) {
	resp, err := httpClient.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}(resp.Body)
	var info releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.TagName == "" {
		return "", errors.New("could not fetch latest release tag")
	}
	return info.TagName, nil
}

func getAssetName() (asset string, platform string, err error) {
	osName := osruntime.GOOS
	arch := osruntime.GOARCH
	var ext string
	switch osName {
	case "darwin":
		platform = "darwin"
		ext = ".zip"
	case "linux":
		platform = "linux"
		ext = ".tar.gz"
	case "windows":
		platform = "windows"
		ext = ".zip"
	default:
		return "", "", fmt.Errorf("unsupported OS: %s", osName)
	}
	var archName string
	switch arch {
	case "amd64", "x86_64":
		archName = "amd64"
	case "arm64", "aarch64":
		if osName == "windows" {
			archName = "amd64"
		} else {
			archName = "arm64"
		}
	default:
		return "", "", fmt.Errorf("unsupported architecture: %s", arch)
	}
	asset = fmt.Sprintf("%s_%s_%s%s", cliName, platform, archName, ext)
	return asset, platform, nil
}

func downloadFile(url, dest string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}(resp.Body)
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			fmt.Println("Error closing out file:", err)
		}
	}(out)
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(assetPath string) (string, error) {
	if strings.HasSuffix(assetPath, ".tar.gz") {
		return untar(assetPath)
	} else if filepath.Ext(assetPath) == ".zip" {
		return unzip(assetPath)
	}
	return "", fmt.Errorf("unsupported archive type: %s", filepath.Ext(assetPath))
}

func untar(assetPath string) (string, error) {
	// .tar.gz
	outDir := filepath.Dir(assetPath)
	f, err := os.Open(assetPath)
	if err != nil {
		return "", err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Println("Error closing file:", err)
		}
	}(f)
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer func(gz *gzip.Reader) {
		err := gz.Close()
		if err != nil {
			fmt.Println("Error closing gzip reader:", err)
		}
	}(gz)
	// Untar
	tr := tar.NewReader(gz)
	var binName string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if strings.Contains(hdr.Name, cliName) && hdr.Typeflag == tar.TypeReg {
			binName = hdr.Name
			cleanName := filepath.Clean(binName)
			if strings.Contains(cleanName, "..") || filepath.IsAbs(cleanName) {
				return "", fmt.Errorf("tar entry contains forbidden path elements: %s", cleanName)
			}
			outPath := filepath.Join(outDir, cleanName)
			absOutDir, err := filepath.Abs(outDir)
			if err != nil {
				return "", err
			}
			absOutPath, err := filepath.Abs(outPath)
			if err != nil {
				return "", err
			}
			if !strings.HasPrefix(absOutPath, absOutDir+string(os.PathSeparator)) && absOutPath != absOutDir {
				return "", fmt.Errorf("tar extraction outside of output directory: %s", absOutPath)
			}
			out, err := os.Create(outPath)
			if err != nil {
				return "", err
			}

			written, err := io.CopyN(out, tr, maxExtractSize+1)
			if err != nil && !errors.Is(err, io.EOF) {
				closeErr := out.Close()
				if closeErr != nil {
					return "", fmt.Errorf("copy error: %w; additionally, close error: %w", err, closeErr)
				}
				return "", err
			}
			if written > maxExtractSize {
				closeErr := out.Close()
				if closeErr != nil {
					return "", closeErr
				}
				return "", fmt.Errorf("extracted file exceeds maximum allowed size")
			}
			closeErr := out.Close()
			if closeErr != nil {
				return "", closeErr
			}
			return outPath, nil
		}
	}
	return "", errors.New("binary not found in tar.gz")

}

func unzip(assetPath string) (string, error) {
	// .zip
	outDir := filepath.Dir(assetPath)
	var binName string
	zr, err := zip.OpenReader(assetPath)
	if err != nil {
		return "", err
	}
	defer func(zr *zip.ReadCloser) {
		err := zr.Close()
		if err != nil {
			fmt.Println("Error closing zip reader:", err)
		}
	}(zr)
	for _, f := range zr.File {
		if strings.Contains(f.Name, cliName) {
			binName = f.Name
			cleanName := filepath.Clean(binName)
			// Check that zip entry is not absolute and does not contain ".."
			if strings.Contains(cleanName, "..") || filepath.IsAbs(cleanName) {
				return "", fmt.Errorf("zip entry contains forbidden path elements: %s", cleanName)
			}
			outPath := filepath.Join(outDir, cleanName)
			absOutDir, err := filepath.Abs(outDir)
			if err != nil {
				return "", err
			}
			absOutPath, err := filepath.Abs(outPath)
			if err != nil {
				return "", err
			}
			// Ensure extracted file is within the intended directory
			if !strings.HasPrefix(absOutPath, absOutDir+string(os.PathSeparator)) && absOutPath != absOutDir {
				return "", fmt.Errorf("zip extraction outside of output directory: %s", absOutPath)
			}
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			out, err := os.Create(outPath)
			if err != nil {
				return "", err
			}

			written, err := io.CopyN(out, rc, maxExtractSize+1)
			if err != nil && !errors.Is(err, io.EOF) {
				closeErr := out.Close()
				if closeErr != nil {
					// Optionally, combine both errors
					return "", fmt.Errorf("copy error: %w; additionally, close error: %w", err, closeErr)
				}
				return "", err
			}
			if written > maxExtractSize {
				closeErr := out.Close()
				if closeErr != nil {
					return "", closeErr
				}
				return "", fmt.Errorf("extracted file exceeds maximum allowed size")
			}
			closeErr := out.Close()
			if closeErr != nil {
				return "", closeErr
			}
			closeErr = rc.Close()
			if closeErr != nil {
				return "", closeErr
			}
			return outPath, nil
		}
	}
	return "", errors.New("binary not found in zip")
}

func replaceSelf(newBin string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	// On Windows, need to move after process exit
	if osruntime.GOOS == "windows" {
		fmt.Println("Please close all running cre processes and manually replace the binary at:", self)
		fmt.Println("New binary downloaded at:", newBin)
		return fmt.Errorf("automatic replacement not supported on Windows")
	}
	// On Unix, can replace in-place
	return os.Rename(newBin, self)
}

// Run accepts the currentVersion string
func Run(currentVersion string) error {
	fmt.Println("Checking for updates...")
	tag, err := getLatestTag()
	if err != nil {
		return fmt.Errorf("error fetching latest version: %w", err)
	}

	// --- New Update Check Logic ---
	// Clean the current version string (e.g., "version v1.2.3" -> "v1.2.3")
	cleanedCurrent := strings.Replace(currentVersion, "version", "", 1)
	cleanedCurrent = strings.TrimSpace(cleanedCurrent)

	// Clean the latest tag (e.g., "v1.2.4")
	cleanedLatest := strings.TrimSpace(tag)

	currentSemVer, errCurrent := semver.NewVersion(cleanedCurrent)
	latestSemVer, errLatest := semver.NewVersion(cleanedLatest)

	if errCurrent != nil || errLatest != nil {
		// If we can't parse either version, fall back to just updating.
		// Print a warning to stderr.
		fmt.Fprintf(os.Stderr, "Warning: could not compare versions (current: '%s', latest: '%s'). Proceeding with update.\n", cleanedCurrent, cleanedLatest)
		if errCurrent != nil {
			fmt.Fprintf(os.Stderr, "Current version parse error: %v\n", errCurrent)
		}
		if errLatest != nil {
			fmt.Fprintf(os.Stderr, "Latest version parse error: %v\n", errLatest)
		}
	} else {
		// Compare versions
		if latestSemVer.LessThan(currentSemVer) || latestSemVer.Equal(currentSemVer) {
			fmt.Printf("You are already using the latest version %s\n", currentSemVer.String())
			return nil // Skip the update
		}
	}
	// --- End of New Logic ---

	// If we're here, an update is needed.
	fmt.Println("Updating cre CLI...")

	asset, _, err := getAssetName()
	if err != nil {
		return fmt.Errorf("error determining asset name: %w", err)
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
	tmpDir, err := os.MkdirTemp("", "cre_update_")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}
	assetPath := filepath.Join(tmpDir, asset)
	fmt.Println("Downloading:", url)
	if err := downloadFile(url, assetPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	binPath, err := extractBinary(assetPath)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := replaceSelf(binPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)
	fmt.Println("cre CLI updated to", tag)
	cmd := exec.Command(cliName, "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Failed to run version command:", err)
	}
	return nil
}

// New is modified to use the version package
func New(_ *runtime.Context) *cobra.Command { // <-- No longer uses rt
	var versionCmd = &cobra.Command{
		Use:   "update",
		Short: "Update the cre CLI to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(version.Version)
		},
	}

	return versionCmd
}
