package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

// Client wraps the Docker client with additional functionality
type Client struct {
	client *client.Client
	logger *logrus.Logger
}

// ContainerInfo represents information about a running container
type ContainerInfo struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	ImageID    string            `json:"image_id"`
	Registry   string            `json:"registry"`
	Repository string            `json:"repository"`
	Tag        string            `json:"tag"`
	Created    time.Time         `json:"created"`
	State      string            `json:"state"`
	Status     string            `json:"status"`
	Labels     map[string]string `json:"labels"`
	Ports      []PortMapping     `json:"ports"`
	Mounts     []MountInfo       `json:"mounts"`
	Networks   []string          `json:"networks"`
	SizeRw     int64             `json:"size_rw,omitempty"`
	SizeRootFs int64             `json:"size_root_fs,omitempty"`
}

// PortMapping represents a port mapping for a container
type PortMapping struct {
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port,omitempty"`
	Type        string `json:"type"`
	IP          string `json:"ip,omitempty"`
}

// MountInfo represents mount information for a container
type MountInfo struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
	Propagation string `json:"propagation"`
}

// ImageReference represents a parsed image reference
type ImageReference struct {
	Registry   string `json:"registry"`
	Namespace  string `json:"namespace"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest,omitempty"`
	FullName   string `json:"full_name"`
}

// NewClient creates a new Docker client
func NewClient(socketPath, apiVersion string, logger *logrus.Logger) (*Client, error) {
	opts := []client.Opt{
		client.WithHost(socketPath),
		client.WithAPIVersionNegotiation(),
	}

	if apiVersion != "" {
		opts = append(opts, client.WithVersion(apiVersion))
	}

	dockerClient, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = dockerClient.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ping Docker daemon: %w", err)
	}

	return &Client{
		client: dockerClient,
		logger: logger,
	}, nil
}

// GetRunningContainers retrieves all running containers
func (c *Client) GetRunningContainers(ctx context.Context) ([]ContainerInfo, error) {
	containers, err := c.client.ContainerList(ctx, container.ListOptions{
		All: false, // Only running containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	result := make([]ContainerInfo, 0, len(containers))

	for _, cont := range containers {
		containerInfo, err := c.convertContainer(cont)
		if err != nil {
			c.logger.WithError(err).WithField("container_id", cont.ID).
				Warn("Failed to convert container info")
			continue
		}

		result = append(result, containerInfo)
	}

	c.logger.WithField("count", len(result)).Debug("Retrieved running containers")
	return result, nil
}

// GetContainersByImagePattern retrieves containers matching image patterns
func (c *Client) GetContainersByImagePattern(ctx context.Context, patterns []string) ([]ContainerInfo, error) {
	allContainers, err := c.GetRunningContainers(ctx)
	if err != nil {
		return nil, err
	}

	if len(patterns) == 0 {
		return allContainers, nil
	}

	var filteredContainers []ContainerInfo

	for _, container := range allContainers {
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, container.Image)
			if err != nil {
				c.logger.WithError(err).WithField("pattern", pattern).
					Warn("Invalid image pattern")
				continue
			}

			if matched {
				filteredContainers = append(filteredContainers, container)
				break
			}
		}
	}

	c.logger.WithFields(logrus.Fields{
		"patterns": patterns,
		"matched":  len(filteredContainers),
		"total":    len(allContainers),
	}).Debug("Filtered containers by image patterns")

	return filteredContainers, nil
}

// GetUniqueImages returns unique images from running containers
func (c *Client) GetUniqueImages(ctx context.Context) ([]ImageReference, error) {
	containers, err := c.GetRunningContainers(ctx)
	if err != nil {
		return nil, err
	}

	imageMap := make(map[string]ImageReference)

	for _, container := range containers {
		imageRef := ImageReference{
			Registry:   container.Registry,
			Repository: container.Repository,
			Tag:        container.Tag,
			FullName:   container.Image,
		}

		// Use full image name as key to ensure uniqueness
		imageMap[container.Image] = imageRef
	}

	result := make([]ImageReference, 0, len(imageMap))
	for _, imageRef := range imageMap {
		result = append(result, imageRef)
	}

	c.logger.WithField("count", len(result)).Debug("Retrieved unique images")
	return result, nil
}

// InspectContainer provides detailed information about a container
func (c *Client) InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error) {
	inspect, err := c.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	// Parse created timestamp
	created, err := time.Parse(time.RFC3339Nano, inspect.Created)
	if err != nil {
		created = time.Now() // fallback to current time if parsing fails
	}

	containerInfo := &ContainerInfo{
		ID:      inspect.ID,
		Name:    strings.TrimPrefix(inspect.Name, "/"),
		Image:   inspect.Config.Image,
		ImageID: inspect.Image,
		Created: created,
		State:   inspect.State.Status,
		Status:  inspect.State.Status,
		Labels:  inspect.Config.Labels,
	}

	// Parse image reference
	imageRef, err := ParseImageReference(inspect.Config.Image)
	if err != nil {
		c.logger.WithError(err).WithField("image", inspect.Config.Image).
			Warn("Failed to parse image reference")
	} else {
		containerInfo.Registry = imageRef.Registry
		containerInfo.Repository = imageRef.Repository
		containerInfo.Tag = imageRef.Tag
	}

	// Convert port mappings
	for port, bindings := range inspect.NetworkSettings.Ports {
		portInfo := PortMapping{
			Type: string(port.Proto()),
		}

		if privatePort := port.Int(); privatePort > 0 {
			portInfo.PrivatePort = privatePort
		}

		if len(bindings) > 0 && bindings[0].HostPort != "" {
			if publicPort := parsePort(bindings[0].HostPort); publicPort > 0 {
				portInfo.PublicPort = publicPort
				portInfo.IP = bindings[0].HostIP
			}
		}

		containerInfo.Ports = append(containerInfo.Ports, portInfo)
	}

	// Convert mounts
	for _, mount := range inspect.Mounts {
		mountInfo := MountInfo{
			Type:        string(mount.Type),
			Source:      mount.Source,
			Destination: mount.Destination,
			Mode:        mount.Mode,
			RW:          mount.RW,
			Propagation: string(mount.Propagation),
		}
		containerInfo.Mounts = append(containerInfo.Mounts, mountInfo)
	}

	// Get network names
	for networkName := range inspect.NetworkSettings.Networks {
		containerInfo.Networks = append(containerInfo.Networks, networkName)
	}

	return containerInfo, nil
}

// convertContainer converts Docker API container to our ContainerInfo
func (c *Client) convertContainer(cont types.Container) (ContainerInfo, error) {
	containerInfo := ContainerInfo{
		ID:         cont.ID,
		Image:      cont.Image,
		ImageID:    cont.ImageID,
		Created:    time.Unix(cont.Created, 0),
		State:      cont.State,
		Status:     cont.Status,
		Labels:     cont.Labels,
		SizeRw:     cont.SizeRw,
		SizeRootFs: cont.SizeRootFs,
	}

	// Get container name (remove leading slash)
	if len(cont.Names) > 0 {
		containerInfo.Name = strings.TrimPrefix(cont.Names[0], "/")
	}

	// Parse image reference
	imageRef, err := ParseImageReference(cont.Image)
	if err != nil {
		return containerInfo, fmt.Errorf("failed to parse image reference: %w", err)
	}

	containerInfo.Registry = imageRef.Registry
	containerInfo.Repository = imageRef.Repository
	containerInfo.Tag = imageRef.Tag

	// Convert port mappings
	for _, port := range cont.Ports {
		portInfo := PortMapping{
			PrivatePort: int(port.PrivatePort),
			Type:        port.Type,
		}

		if port.PublicPort > 0 {
			portInfo.PublicPort = int(port.PublicPort)
			portInfo.IP = port.IP
		}

		containerInfo.Ports = append(containerInfo.Ports, portInfo)
	}

	// Get network names
	for networkName := range cont.NetworkSettings.Networks {
		containerInfo.Networks = append(containerInfo.Networks, networkName)
	}

	return containerInfo, nil
}

// ParseImageReference parses a Docker image reference
func ParseImageReference(image string) (*ImageReference, error) {
	if image == "" {
		return nil, fmt.Errorf("empty image reference")
	}

	// Regular expression to parse image references
	// Supports: [registry[:port]/][namespace/]repository[:tag][@digest]
	re := regexp.MustCompile(`^(?:([^/]+(?:\.[^/]+)*(?::[0-9]+)?)/)?(?:([^/]+)/)?([^:@/]+)(?::([^@]+))?(?:@(.+))?$`)
	matches := re.FindStringSubmatch(image)

	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid image reference format: %s", image)
	}

	registry := matches[1]
	namespace := matches[2]
	repository := matches[3]
	tag := matches[4]
	digest := matches[5]

	// Set default registry if not specified
	if registry == "" {
		registry = "docker.io"
	}

	// Set default tag if not specified and no digest
	if tag == "" && digest == "" {
		tag = "latest"
	}

	// Build full repository name
	fullRepo := repository
	if namespace != "" {
		fullRepo = namespace + "/" + repository
	}

	// For Docker Hub, add library namespace for official images
	if registry == "docker.io" && namespace == "" && !strings.Contains(repository, "/") {
		fullRepo = "library/" + repository
	}

	return &ImageReference{
		Registry:   registry,
		Namespace:  namespace,
		Repository: fullRepo,
		Tag:        tag,
		Digest:     digest,
		FullName:   image,
	}, nil
}

// IsPrivateRegistry checks if the image is from a private registry
func (ir *ImageReference) IsPrivateRegistry() bool {
	return ir.Registry != "docker.io" && ir.Registry != "index.docker.io"
}

// GetRegistryURL returns the full registry URL
func (ir *ImageReference) GetRegistryURL() string {
	if ir.Registry == "docker.io" || ir.Registry == "index.docker.io" {
		return "https://registry-1.docker.io"
	}
	return "https://" + ir.Registry
}

// GetRepositoryPath returns the repository path for API calls
func (ir *ImageReference) GetRepositoryPath() string {
	return ir.Repository
}

// String returns a string representation of the image reference
func (ir *ImageReference) String() string {
	if ir.Tag != "" {
		return fmt.Sprintf("%s/%s:%s", ir.Registry, ir.Repository, ir.Tag)
	}
	if ir.Digest != "" {
		return fmt.Sprintf("%s/%s@%s", ir.Registry, ir.Repository, ir.Digest)
	}
	return fmt.Sprintf("%s/%s", ir.Registry, ir.Repository)
}

// Close closes the Docker client
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// parsePort parses a port string to integer
func parsePort(portStr string) int {
	if portStr == "" {
		return 0
	}

	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 0
	}
	return port
}

// Health checks the health of the Docker daemon connection
func (c *Client) Health(ctx context.Context) error {
	_, err := c.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("Docker daemon is not accessible: %w", err)
	}
	return nil
}

// GetDockerInfo returns information about the Docker daemon
func (c *Client) GetDockerInfo(ctx context.Context) (*system.Info, error) {
	info, err := c.client.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}
	return &info, nil
}
