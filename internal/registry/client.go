package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// VersionFilterConfig defines version filtering options
type VersionFilterConfig struct {
	ExcludePreRelease bool
	ExcludeWindows    bool
	ExcludePatterns   []string
	OnlyStable        bool
}

// Client handles registry API operations
type Client struct {
	httpClient     *http.Client
	rateLimiter    *rate.Limiter
	logger         *logrus.Logger
	versionFilters VersionFilterConfig
}

// ImageManifest represents an image manifest
type ImageManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

// TagsResponse represents the response from tags API
type TagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// DockerHubTokenResponse represents the response from DockerHub token API
type DockerHubTokenResponse struct {
	Token       string    `json:"token"`
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

// ImageUpdateInfo contains information about available updates
type ImageUpdateInfo struct {
	CurrentTag    string    `json:"current_tag"`
	LatestTag     string    `json:"latest_tag"`
	AvailableTags []string  `json:"available_tags"`
	LastUpdated   time.Time `json:"last_updated"`
	HasUpdate     bool      `json:"has_update"`
	Registry      string    `json:"registry"`
	Repository    string    `json:"repository"`
}

// VersionComparison represents version comparison result
type VersionComparison int

const (
	VersionEqual VersionComparison = iota
	VersionOlder
	VersionNewer
	VersionIncomparable
)

// NewClient creates a new registry client
func NewClient(requestsPerMinute int, burst int, logger *logrus.Logger) *Client {
	// Create rate limiter
	limiter := rate.NewLimiter(rate.Limit(requestsPerMinute/60), burst)

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	return &Client{
		httpClient:  httpClient,
		rateLimiter: limiter,
		logger:      logger,
		versionFilters: VersionFilterConfig{
			ExcludePreRelease: true,
			ExcludeWindows:    true,
			OnlyStable:        true,
		},
	}
}

// NewClientWithFilters creates a new registry client with custom version filters
func NewClientWithFilters(requestsPerMinute int, burst int, logger *logrus.Logger, filters VersionFilterConfig) *Client {
	// Create rate limiter
	limiter := rate.NewLimiter(rate.Limit(requestsPerMinute/60), burst)

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	return &Client{
		httpClient:     httpClient,
		rateLimiter:    limiter,
		logger:         logger,
		versionFilters: filters,
	}
}

// CheckImageUpdate checks if there's an update available for an image
func (c *Client) CheckImageUpdate(ctx context.Context, registry, repository, currentTag string) (*ImageUpdateInfo, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	updateInfo := &ImageUpdateInfo{
		CurrentTag: currentTag,
		Registry:   registry,
		Repository: repository,
		HasUpdate:  false,
	}

	// Get available tags
	tags, err := c.getImageTags(ctx, registry, repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get image tags: %w", err)
	}

	updateInfo.AvailableTags = tags

	if len(tags) == 0 {
		c.logger.WithFields(logrus.Fields{
			"registry":   registry,
			"repository": repository,
		}).Warn("No tags found for image")
		return updateInfo, nil
	}

	// Find the latest version
	latestTag, err := c.findLatestTag(tags, currentTag)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"registry":    registry,
			"repository":  repository,
			"current_tag": currentTag,
		}).Warn("Failed to determine latest tag")
		return updateInfo, nil
	}

	updateInfo.LatestTag = latestTag

	// Compare versions
	comparison := c.compareVersions(currentTag, latestTag)
	updateInfo.HasUpdate = comparison == VersionOlder

	c.logger.WithFields(logrus.Fields{
		"registry":    registry,
		"repository":  repository,
		"current_tag": currentTag,
		"latest_tag":  latestTag,
		"has_update":  updateInfo.HasUpdate,
	}).Debug("Completed image update check")

	return updateInfo, nil
}

// getImageTags retrieves all available tags for an image
func (c *Client) getImageTags(ctx context.Context, registry, repository string) ([]string, error) {
	var url string
	var headers map[string]string

	if registry == "docker.io" || registry == "index.docker.io" {
		// DockerHub API
		token, err := c.getDockerHubToken(ctx, repository)
		if err != nil {
			return nil, fmt.Errorf("failed to get DockerHub token: %w", err)
		}

		url = fmt.Sprintf("https://registry-1.docker.io/v2/%s/tags/list", repository)
		headers = map[string]string{
			"Authorization": "Bearer " + token,
			"Accept":        "application/json",
		}
	} else {
		// Generic registry API
		url = fmt.Sprintf("https://%s/v2/%s/tags/list", registry, repository)
		headers = map[string]string{
			"Accept": "application/json",
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp TagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode tags response: %w", err)
	}

	return tagsResp.Tags, nil
}

// getDockerHubToken gets an authentication token for DockerHub
func (c *Client) getDockerHubToken(ctx context.Context, repository string) (string, error) {
	url := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repository)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp DockerHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.Token, nil
}

// findLatestTag finds the latest semantic version tag from available tags
func (c *Client) findLatestTag(tags []string, currentTag string) (string, error) {
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags available")
	}

	// If current tag is "latest", find the highest semantic version
	if currentTag == "latest" {
		return c.findHighestSemanticVersion(tags), nil
	}

	// Filter semantic version tags and exclude unwanted variants
	semverTags := c.filterSemanticVersionTags(tags)
	filteredTags := c.filterUnwantedVersions(semverTags)

	if len(filteredTags) == 0 {
		// No semantic versions found, check if there's a "latest" tag
		for _, tag := range tags {
			if tag == "latest" {
				return "latest", nil
			}
		}
		// Return the first available tag as fallback
		return tags[0], nil
	}

	// Find the highest semantic version
	return c.findHighestSemanticVersion(filteredTags), nil
}

// filterSemanticVersionTags filters tags that look like semantic versions
func (c *Client) filterSemanticVersionTags(tags []string) []string {
	semverRegex := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?(?:\+([a-zA-Z0-9\-\.]+))?$`)
	var semverTags []string

	for _, tag := range tags {
		if semverRegex.MatchString(tag) {
			semverTags = append(semverTags, tag)
		}
	}

	return semverTags
}

// filterUnwantedVersions filters out RC, beta, alpha, Windows, and other unwanted version variants
func (c *Client) filterUnwantedVersions(tags []string) []string {
	var filtered []string

	// Build exclude patterns based on configuration
	var excludePatterns []string

	if c.versionFilters.ExcludePreRelease {
		excludePatterns = append(excludePatterns, "rc", "alpha", "beta", "dev", "snapshot", "nightly", "pre")
	}

	if c.versionFilters.ExcludeWindows {
		excludePatterns = append(excludePatterns, "windows", "windowsservercore", "nanoserver", "ltsc", "insider")
	}

	// Add custom patterns from configuration
	excludePatterns = append(excludePatterns, c.versionFilters.ExcludePatterns...)

	for _, tag := range tags {
		shouldExclude := false
		tagLower := strings.ToLower(tag)

		// Check if tag contains any exclude patterns
		for _, pattern := range excludePatterns {
			if strings.Contains(tagLower, strings.ToLower(pattern)) {
				shouldExclude = true
				c.logger.WithFields(logrus.Fields{
					"tag":     tag,
					"pattern": pattern,
				}).Debug("Excluding version tag due to filter")
				break
			}
		}

		// If only stable versions are wanted, check for proper semantic versioning
		if !shouldExclude && c.versionFilters.OnlyStable {
			if !c.isStableSemanticVersion(tag) {
				shouldExclude = true
				c.logger.WithField("tag", tag).Debug("Excluding non-stable version tag")
			}
		}

		if !shouldExclude {
			filtered = append(filtered, tag)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"original_count": len(tags),
		"filtered_count": len(filtered),
	}).Debug("Applied version filtering")

	return filtered
}

// isStableSemanticVersion checks if a tag represents a stable semantic version
func (c *Client) isStableSemanticVersion(tag string) bool {
	// Remove 'v' prefix if present
	cleanTag := strings.TrimPrefix(tag, "v")

	// Check for stable semantic version pattern (x.y.z with optional build metadata)
	// This excludes pre-release versions like 1.2.3-alpha
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:\+([a-zA-Z0-9\-\.]+))?$`)
	return re.MatchString(cleanTag)
}

// findHighestSemanticVersion finds the highest semantic version from a list of tags
func (c *Client) findHighestSemanticVersion(tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	highest := tags[0]
	for _, tag := range tags[1:] {
		if c.compareVersions(highest, tag) == VersionOlder {
			highest = tag
		}
	}

	return highest
}

// compareVersions compares two version strings
func (c *Client) compareVersions(version1, version2 string) VersionComparison {
	// Handle special cases
	if version1 == version2 {
		return VersionEqual
	}

	if version1 == "latest" || version2 == "latest" {
		return VersionIncomparable
	}

	// Try semantic version comparison
	v1 := c.parseSemanticVersion(version1)
	v2 := c.parseSemanticVersion(version2)

	if v1 == nil || v2 == nil {
		// Fall back to string comparison
		if version1 < version2 {
			return VersionOlder
		} else if version1 > version2 {
			return VersionNewer
		}
		return VersionEqual
	}

	// Compare major version
	if v1.Major < v2.Major {
		return VersionOlder
	} else if v1.Major > v2.Major {
		return VersionNewer
	}

	// Compare minor version
	if v1.Minor < v2.Minor {
		return VersionOlder
	} else if v1.Minor > v2.Minor {
		return VersionNewer
	}

	// Compare patch version
	if v1.Patch < v2.Patch {
		return VersionOlder
	} else if v1.Patch > v2.Patch {
		return VersionNewer
	}

	// Compare pre-release versions
	if v1.PreRelease == "" && v2.PreRelease != "" {
		return VersionNewer // Release is newer than pre-release
	} else if v1.PreRelease != "" && v2.PreRelease == "" {
		return VersionOlder // Pre-release is older than release
	} else if v1.PreRelease != "" && v2.PreRelease != "" {
		if v1.PreRelease < v2.PreRelease {
			return VersionOlder
		} else if v1.PreRelease > v2.PreRelease {
			return VersionNewer
		}
	}

	return VersionEqual
}

// SemanticVersion represents a parsed semantic version
type SemanticVersion struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
}

// parseSemanticVersion parses a semantic version string
func (c *Client) parseSemanticVersion(version string) *SemanticVersion {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Regular expression for semantic versioning
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?(?:\+([a-zA-Z0-9\-\.]+))?$`)
	matches := re.FindStringSubmatch(version)

	if len(matches) < 4 {
		return nil
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil
	}

	preRelease := ""
	if len(matches) > 4 {
		preRelease = matches[4]
	}

	build := ""
	if len(matches) > 5 {
		build = matches[5]
	}

	return &SemanticVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		Build:      build,
	}
}

// GetImageManifest retrieves the manifest for a specific image tag
func (c *Client) GetImageManifest(ctx context.Context, registry, repository, tag string) (*ImageManifest, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	var url string
	var headers map[string]string

	if registry == "docker.io" || registry == "index.docker.io" {
		// DockerHub API
		token, err := c.getDockerHubToken(ctx, repository)
		if err != nil {
			return nil, fmt.Errorf("failed to get DockerHub token: %w", err)
		}

		url = fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", repository, tag)
		headers = map[string]string{
			"Authorization": "Bearer " + token,
			"Accept":        "application/vnd.docker.distribution.manifest.v2+json",
		}
	} else {
		// Generic registry API
		url = fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, tag)
		headers = map[string]string{
			"Accept": "application/vnd.docker.distribution.manifest.v2+json",
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("manifest API returned status %d: %s", resp.StatusCode, string(body))
	}

	var manifest ImageManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest response: %w", err)
	}

	return &manifest, nil
}

// CheckMultipleImages checks multiple images for updates concurrently
func (c *Client) CheckMultipleImages(ctx context.Context, images []ImageCheck, maxConcurrency int) ([]ImageUpdateInfo, error) {
	if len(images) == 0 {
		return nil, nil
	}

	// Create semaphore for concurrency control
	sem := make(chan struct{}, maxConcurrency)
	results := make(chan ImageUpdateResult, len(images))

	// Launch goroutines for each image check
	for _, img := range images {
		go func(imageCheck ImageCheck) {
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			updateInfo, err := c.CheckImageUpdate(ctx, imageCheck.Registry, imageCheck.Repository, imageCheck.Tag)
			results <- ImageUpdateResult{
				UpdateInfo: updateInfo,
				Error:      err,
				Image:      imageCheck,
			}
		}(img)
	}

	// Collect results
	var updateInfos []ImageUpdateInfo
	var errors []error

	for i := 0; i < len(images); i++ {
		result := <-results
		if result.Error != nil {
			c.logger.WithError(result.Error).WithFields(logrus.Fields{
				"registry":   result.Image.Registry,
				"repository": result.Image.Repository,
				"tag":        result.Image.Tag,
			}).Error("Failed to check image update")
			errors = append(errors, result.Error)
		} else if result.UpdateInfo != nil {
			updateInfos = append(updateInfos, *result.UpdateInfo)
		}
	}

	if len(errors) > 0 && len(updateInfos) == 0 {
		return nil, fmt.Errorf("all image checks failed: %d errors", len(errors))
	}

	return updateInfos, nil
}

// ImageCheck represents an image to check for updates
type ImageCheck struct {
	Registry   string
	Repository string
	Tag        string
}

// ImageUpdateResult represents the result of an image update check
type ImageUpdateResult struct {
	UpdateInfo *ImageUpdateInfo
	Error      error
	Image      ImageCheck
}

// Health checks the health of registry connections
func (c *Client) Health(ctx context.Context) error {
	// Test connection to DockerHub
	req, err := http.NewRequestWithContext(ctx, "GET", "https://registry-1.docker.io/v2/", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DockerHub is not accessible: %w", err)
	}
	defer resp.Body.Close()

	// DockerHub returns 401 for unauthenticated requests to /v2/, which is expected
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DockerHub returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}
