package update

import (
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

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

const (
	repo    = "smartcontractkit/cre-cli"
	cliName = "cre"
)

type releaseInfo struct {
	TagName string `json:"tag_name"`
}

func getLatestTag() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var info releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.TagName == "" {
		return "", errors.New("could not fetch latest release tag")
	}
	return info.TagName, nil
}

func getAssetName() (string, string, error) {
	osName := osruntime.GOOS
	arch := osruntime.GOARCH
	var platform, ext string
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
	asset := fmt.Sprintf("%s_%s_%s%s", cliName, platform, archName, ext)
	return asset, platform, nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(assetPath, platform, tag string) (string, error) {
	// Returns path to extracted binary
	outDir := filepath.Dir(assetPath)
	var binName string
	if platform == "linux" {
		// .tar.gz
		f, err := os.Open(assetPath)
		if err != nil {
			return "", err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		defer gz.Close()
		// Untar
		return untar(gz, outDir)
	} else {
		// .zip
		zr, err := zip.OpenReader(assetPath)
		if err != nil {
			return "", err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if strings.Contains(f.Name, cliName) {
				binName = f.Name
				outPath := filepath.Join(outDir, binName)
				rc, err := f.Open()
				if err != nil {
					return "", err
				}
				defer rc.Close()
				out, err := os.Create(outPath)
				if err != nil {
					return "", err
				}
				defer out.Close()
				if _, err := io.Copy(out, rc); err != nil {
					return "", err
				}
				return outPath, nil
			}
		}
		return "", errors.New("binary not found in zip")
	}
}

func untar(r io.Reader, outDir string) (string, error) {
	// Minimal tar.gz extraction for cre binary
	return "", errors.New("untar not implemented")
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
		return nil
	}
	// On Unix, can replace in-place
	return os.Rename(newBin, self)
}

func Run() {
	fmt.Println("Updating cre CLI...")
	tag, err := getLatestTag()
	if err != nil {
		fmt.Println("Error fetching latest version:", err)
		return
	}
	asset, platform, err := getAssetName()
	if err != nil {
		fmt.Println("Error determining asset name:", err)
		return
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
	tmpDir, err := os.MkdirTemp("", "cre_update_")
	if err != nil {
		fmt.Println("Error creating temp dir:", err)
		return
	}
	defer os.RemoveAll(tmpDir)
	assetPath := filepath.Join(tmpDir, asset)
	fmt.Println("Downloading:", url)
	if err := downloadFile(url, assetPath); err != nil {
		fmt.Println("Download failed:", err)
		return
	}
	binPath, err := extractBinary(assetPath, platform, tag)
	if err != nil {
		fmt.Println("Extraction failed:", err)
		return
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		fmt.Println("Failed to set permissions:", err)
		return
	}
	if err := replaceSelf(binPath); err != nil {
		fmt.Println("Failed to replace binary:", err)
		return
	}
	fmt.Println("cre CLI updated to", tag)
	cmd := exec.Command(cliName, "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "update",
		Short: "Self-update the cre CLI to the latest version",
		Run: func(cmd *cobra.Command, args []string) {
			Run()
		},
	}

	return versionCmd
}
