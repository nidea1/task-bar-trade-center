package game

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const LayoutRequestTimeout = 5 * time.Second

func LoadGameLayoutFromFile(filePath string) (GameLayout, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return GameLayout{}, err
	}
	return ParseGameLayout(raw)
}

func ResolveGameLayout(remoteURL, cacheFilePath string, client *http.Client, embeddedDefaults []byte, userAgent string) (GameLayout, string, error) {
	remoteBytes, err := DownloadGameLayout(remoteURL, client, userAgent)
	if err == nil {
		layout, parseErr := ParseGameLayout(remoteBytes)
		if parseErr == nil {
			layout, parseErr = ApplyEmbeddedAOBFallback(layout, embeddedDefaults)
			if parseErr != nil {
				return GameLayout{}, "", parseErr
			}
			if cacheFilePath != "" {
				if writeErr := WriteGameLayoutCache(cacheFilePath, remoteBytes); writeErr != nil {
					fmt.Printf("Game layout cache could not be written: %v\n", writeErr)
				}
			}
			return layout, LayoutSourceRemote, nil
		}
		fmt.Printf("Downloaded game layout is invalid: %v\n", parseErr)
	} else {
		fmt.Printf("Game layout download failed: %v\n", err)
	}

	if cacheFilePath != "" {
		if cachedBytes, readErr := os.ReadFile(cacheFilePath); readErr == nil {
			if layout, parseErr := ParseGameLayout(cachedBytes); parseErr == nil {
				layout, parseErr = ApplyEmbeddedAOBFallback(layout, embeddedDefaults)
				if parseErr != nil {
					return GameLayout{}, "", parseErr
				}
				return layout, LayoutSourceCache, nil
			}
			fmt.Println("Game layout cache is invalid; using embedded layout.")
		} else if !os.IsNotExist(readErr) {
			fmt.Printf("Game layout cache could not be read: %v\n", readErr)
		}
	}

	layout, parseErr := ParseGameLayout(embeddedDefaults)
	if parseErr != nil {
		return GameLayout{}, "", fmt.Errorf("embedded game layout is invalid: %w", parseErr)
	}
	return layout, LayoutSourceEmbeddedDefault, nil
}

func DownloadGameLayout(remoteURL string, client *http.Client, userAgent string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("HTTP client is not configured")
	}

	req, err := http.NewRequest(http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not contact GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}
	return body, nil
}

func WriteGameLayoutCache(cacheFilePath string, raw []byte) error {
	directory := filepath.Dir(cacheFilePath)
	temporaryFile, err := os.CreateTemp(directory, filepath.Base(cacheFilePath)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if err := temporaryFile.Chmod(0600); err != nil {
		temporaryFile.Close()
		return err
	}
	if _, err := temporaryFile.Write(raw); err != nil {
		temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Sync(); err != nil {
		temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, cacheFilePath)
}
