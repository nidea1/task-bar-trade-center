package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

func IsNewerVersion(current, latest string) bool {
	current = strings.TrimPrefix(strings.ToLower(current), "v")
	latest = strings.TrimPrefix(strings.ToLower(latest), "v")

	currParts := strings.Split(current, ".")
	lateParts := strings.Split(latest, ".")

	for i := 0; i < len(currParts) && i < len(lateParts); i++ {
		cVal, _ := strconv.Atoi(currParts[i])
		lVal, _ := strconv.Atoi(lateParts[i])
		if lVal > cVal {
			return true
		}
		if cVal > lVal {
			return false
		}
	}
	return len(lateParts) > len(currParts)
}

func FetchLatestRelease(releaseURL string, client *http.Client, userAgent string) (Release, int, error) {
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return Release{}, 0, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Release{}, resp.StatusCode, fmt.Errorf("update server returned status: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Release{}, resp.StatusCode, err
	}
	return release, resp.StatusCode, nil
}

func ExecutableAsset(release Release) (Asset, bool) {
	for _, asset := range release.Assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), ".exe") {
			return asset, true
		}
	}
	return Asset{}, false
}

func DownloadAndSwapExecutable(downloadURL string, client *http.Client, userAgent string, exePath string) error {
	tmpPath := exePath + ".tmp"
	oldPath := exePath + ".old"

	_ = os.Remove(tmpPath)

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned status: %s", resp.Status)
	}

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temporary update file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write downloaded data: %w", err)
	}

	_ = os.Remove(oldPath)

	if err := os.Rename(exePath, oldPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename active executable: %w", err)
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Remove(tmpPath)
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("failed to swap executable: %w", err)
	}

	return nil
}
