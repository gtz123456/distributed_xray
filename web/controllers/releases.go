package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type CachedAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type ReleaseMetadata struct {
	LastFetched int64         `json:"lastFetched"`
	Assets      []CachedAsset `json:"assets"`
}

var (
	cacheMutex      sync.RWMutex
	cachedMetadata  *ReleaseMetadata
	isDownloading   bool
	cacheDir        = "./public/releases"
	metaFile        = filepath.Join(cacheDir, "release_meta.json")
	pollingInterval = 10 * time.Minute
)

func init() {
	os.MkdirAll(cacheDir, os.ModePerm)
	// Try loading existing metadata
	data, err := os.ReadFile(metaFile)
	if err == nil {
		var meta ReleaseMetadata
		if json.Unmarshal(data, &meta) == nil {
			cachedMetadata = &meta
		}
	}
}

func StartReleaseCacheMonitor() {
	// Run once immediately
	refreshCache()

	ticker := time.NewTicker(pollingInterval)
	for range ticker.C {
		refreshCache()
	}
}

func refreshCache() {
	cacheMutex.Lock()
	if isDownloading {
		cacheMutex.Unlock()
		return
	}
	isDownloading = true
	cacheMutex.Unlock()

	defer func() {
		cacheMutex.Lock()
		isDownloading = false
		cacheMutex.Unlock()
	}()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/gtz123456/freewayvpn_client/releases/latest", nil)
	if err != nil {
		log.Println("Error creating GitHub API request:", err)
		return
	}
	req.Header.Set("User-Agent", "FreewayVPN-Backend")

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error fetching GitHub release:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("GitHub API returned non-200 status:", resp.StatusCode)
		return
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Println("Error decoding GitHub release JSON:", err)
		return
	}

	var newAssets []CachedAsset
	for _, asset := range release.Assets {
		filePath := filepath.Join(cacheDir, asset.Name)

		// Check if file already exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("Downloading new asset: %s\n", asset.Name)
			err := downloadFile(filePath, asset.BrowserDownloadURL)
			if err != nil {
				log.Printf("Failed to download %s: %v\n", asset.Name, err)
				continue
			}
		}

		newAssets = append(newAssets, CachedAsset{
			Name: asset.Name,
			URL:  "/downloads/" + asset.Name,
		})
	}

	meta := ReleaseMetadata{
		LastFetched: time.Now().UnixMilli(),
		Assets:      newAssets,
	}

	// Save to memory
	cacheMutex.Lock()
	cachedMetadata = &meta
	cacheMutex.Unlock()

	// Save to disk
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(metaFile, metaBytes, 0644)
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func GetReleases(c *gin.Context) {
	cacheMutex.RLock()
	meta := cachedMetadata
	downloading := isDownloading
	cacheMutex.RUnlock()

	if meta != nil {
		if downloading {
			c.JSON(http.StatusOK, gin.H{
				"assets": meta.Assets,
				"notice": "A newer version may be publishing. Please refresh later for updates.",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"assets": meta.Assets,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "Releases not yet available or failed to fetch",
	})
}
