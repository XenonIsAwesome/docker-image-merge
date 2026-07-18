// Package docker provides a thin wrapper around the docker CLI for image
// operations needed by docker-image-merge.
//
// Rather than importing the Docker SDK (which has complex transitive
// dependencies), this package shells out to the docker CLI binary. This
// keeps the dependency tree small and avoids version-compatibility issues.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client communicates with the Docker daemon via the docker CLI.
type Client struct {
	// platform optionally constrains image operations to a specific platform.
	platform string
}

// NewClient creates a new Docker client wrapper. It verifies that the docker
// CLI is available and the daemon is reachable.
func NewClient(platform string) (*Client, error) {
	if err := checkDocker(); err != nil {
		return nil, err
	}
	return &Client{platform: platform}, nil
}

// checkDocker verifies the docker CLI is installed and the daemon responds.
func checkDocker() error {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docker not available: %w\nMake sure Docker is installed and running", err)
	}
	_ = out
	return nil
}

// ImageMetadata holds the configuration extracted from a Docker image via
// docker inspect. This is used to propagate ENV, CMD, ENTRYPOINT, etc.
// into the merged output image.
type ImageMetadata struct {
	// ID is the image's content-addressable identifier (sha256 digest).
	ID string `json:"id"`

	// Config holds the parsed image configuration (env, cmd, etc.).
	Config *ImageConfig `json:"config"`

	// RawConfig is the original JSON output from docker inspect for the Config field.
	RawConfig json.RawMessage `json:"raw_config"`

	// Platform is the OS/architecture string (e.g. "linux/amd64").
	Platform string `json:"platform"`

	// Size is the image size in bytes as reported by the daemon.
	Size int64 `json:"size"`
}

// ImageConfig mirrors the subset of Docker image configuration we care about.
// See the OCI image spec for the full schema.
type ImageConfig struct {
	// Env is the list of environment variables in "KEY=value" format.
	Env []string `json:"Env"`

	// Cmd is the default command to run when a container starts.
	Cmd []string `json:"Cmd"`

	// Entrypoint is the default entrypoint for the container.
	Entrypoint []string `json:"Entrypoint"`

	// WorkingDir is the default working directory inside the container.
	WorkingDir string `json:"WorkingDir"`

	// User is the default user for running processes (e.g. "1000:1000").
	User string `json:"User"`

	// ExposedPorts lists network ports the container exposes.
	ExposedPorts map[string]struct{} `json:"ExposedPorts"`

	// Volumes lists mount points declared by the image.
	Volumes map[string]struct{} `json:"Volumes"`

	// Labels is a set of key-value metadata labels.
	Labels map[string]string `json:"Labels"`

	// StopSignal is the signal used to gracefully stop the container.
	StopSignal string `json:"StopSignal"`
}

// inspectResult is the top-level structure returned by docker inspect.
type inspectResult struct {
	ID           string       `json:"Id"`
	Size         int64        `json:"Size"`
	Os           string       `json:"Os"`
	Architecture string       `json:"Architecture"`
	Config       *ImageConfig `json:"Config"`
}

// EnsureImage checks whether the named image exists locally. If it does not,
// it pulls it from the registry. If a platform is set, only that platform
// variant is pulled.
func (c *Client) EnsureImage(ctx context.Context, imageName string) error {
	// Try to inspect the image locally first.
	args := []string{"image", "inspect", imageName}
	cmd := exec.CommandContext(ctx, "docker", args...)
	if err := cmd.Run(); err != nil {
		// Not found locally — pull it.
		fmt.Fprintf(os.Stderr, "Pulling image %s...\n", imageName)
		pullArgs := []string{"pull"}
		if c.platform != "" {
			pullArgs = append(pullArgs, "--platform", c.platform)
		}
		pullArgs = append(pullArgs, imageName)
		pullCmd := exec.CommandContext(ctx, "docker", pullArgs...)
		pullCmd.Stdout = os.Stderr
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("pulling image %s: %w", imageName, err)
		}
	}
	return nil
}

// InspectImage runs docker inspect on the named image and returns the parsed
// configuration metadata.
func (c *Client) InspectImage(ctx context.Context, imageName string) (*ImageMetadata, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", imageName)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("inspecting image %s: %w", imageName, err)
	}

	var results []inspectResult
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Errorf("parsing inspect output: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no inspect data for %s", imageName)
	}

	info := results[0]

	var rawCfg json.RawMessage
	cfgBytes, _ := json.Marshal(info.Config)
	rawCfg = cfgBytes

	platStr := ""
	if info.Os != "" {
		platStr = info.Os
		if info.Architecture != "" {
			platStr += "/" + info.Architecture
		}
	}

	return &ImageMetadata{
		ID:        info.ID,
		Config:    info.Config,
		RawConfig: rawCfg,
		Platform:  platStr,
		Size:      info.Size,
	}, nil
}

// ExtractFilesystem creates a temporary container from the given image and
// exports its flattened filesystem to destDir. The export is a single-layer
// tar that is extracted in place.
//
// The temporary container is automatically removed after extraction.
func (c *Client) ExtractFilesystem(ctx context.Context, imageName, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	// Use a deterministic container name based on PID to avoid collisions.
	containerName := fmt.Sprintf("dim-extract-%d", os.Getpid())

	// Create a stopped container from the image (true is a no-op command).
	createArgs := []string{"create", "--name", containerName, imageName, "true"}
	cmd := exec.CommandContext(ctx, "docker", createArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating temp container: %w\n%s", err, string(out))
	}

	// Ensure the container is cleaned up regardless of outcome.
	defer func() {
		rmCmd := exec.Command("docker", "rm", "-f", containerName)
		_ = rmCmd.Run()
	}()

	// Export the container filesystem to a tar archive.
	exportCmd := exec.CommandContext(ctx, "docker", "export", containerName)
	tarPath := filepath.Join(destDir, "export.tar")
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("creating tar file: %w", err)
	}
	exportCmd.Stdout = tarFile
	exportCmd.Stderr = os.Stderr
	if err := exportCmd.Run(); err != nil {
		_ = tarFile.Close()
		return fmt.Errorf("exporting container: %w", err)
	}
	_ = tarFile.Close()

	// Extract the tar into the destination directory.
	fmt.Fprintf(os.Stderr, "Extracting %s...\n", imageName)
	extractCmd := exec.CommandContext(ctx, "tar", "xf", tarPath, "-C", destDir)
	extractCmd.Stderr = os.Stderr
	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("extracting tar: %w", err)
	}

	// Clean up the intermediate tar file.
	_ = os.Remove(tarPath)

	return nil
}

// ImportSquashed creates a single-layer image by piping the merged directory
// through tar into docker import. The image is tagged as outputRef.
//
// If metadata is non-nil, its configuration (ENV, CMD, etc.) is applied to
// the imported image via --change flags.
func (c *Client) ImportSquashed(ctx context.Context, tarDir, outputRef string, metadata *ImageMetadata, changes []string, message string) (string, error) {
	args := []string{"import"}

	if message != "" {
		args = append(args, "-m", message)
	}

	for _, change := range changes {
		args = append(args, "-c", change)
	}

	// Carry over image configuration metadata via dockerfile-style changes.
	if metadata != nil && metadata.Config != nil {
		for _, env := range metadata.Config.Env {
			args = append(args, "-c", "ENV "+env)
		}
		if metadata.Config.WorkingDir != "" {
			args = append(args, "-c", "WORKDIR "+metadata.Config.WorkingDir)
		}
		if len(metadata.Config.Cmd) > 0 {
			cmdStr := "CMD " + jsonArr(metadata.Config.Cmd)
			args = append(args, "-c", cmdStr)
		}
		if len(metadata.Config.Entrypoint) > 0 {
			epStr := "ENTRYPOINT " + jsonArr(metadata.Config.Entrypoint)
			args = append(args, "-c", epStr)
		}
		if metadata.Config.User != "" {
			args = append(args, "-c", "USER "+metadata.Config.User)
		}
	}

	// "-" tells docker import to read the tar from stdin.
	args = append(args, "-", outputRef)

	// Pipe tar output into docker import via stdout/stdin.
	tarCmd := exec.CommandContext(ctx, "tar", "cf", "-", "-C", tarDir, ".")
	importCmd := exec.CommandContext(ctx, "docker", args...)
	importCmd.Stdin, _ = tarCmd.StdoutPipe()
	importCmd.Stderr = os.Stderr

	if err := importCmd.Start(); err != nil {
		return "", fmt.Errorf("docker import: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		return "", fmt.Errorf("tar: %w", err)
	}

	_ = tarCmd.Wait()
	_ = importCmd.Wait()

	if importCmd.Process != nil && importCmd.ProcessState != nil && !importCmd.ProcessState.Success() {
		return "", fmt.Errorf("docker import failed")
	}

	// Retrieve the new image's ID for display.
	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}", outputRef)
	out, err := inspectCmd.Output()
	if err != nil {
		return outputRef, nil
	}

	return strings.TrimSpace(string(out)), nil
}

// BuildLayered creates a new image by building a Dockerfile that copies the
// merged filesystem on top of baseImage. This preserves image A's layer
// structure.
//
// The generated Dockerfile does:
//
//	FROM <baseImage>
//	COPY . /tmp/merged/
//	RUN cp -a /tmp/merged/. / && rm -rf /tmp/merged
//
// If the build fails, the caller should fall back to ImportSquashed.
func (c *Client) BuildLayered(ctx context.Context, baseImage, mergedDir, outputRef string, metadata *ImageMetadata, changes []string) (string, error) {
	dockerfilePath := filepath.Join(mergedDir, "Dockerfile.merge")

	var df strings.Builder
	fmt.Fprintf(&df, "FROM %s\n", baseImage)
	fmt.Fprintf(&df, "COPY . /tmp/merged/\n")
	fmt.Fprintf(&df, "RUN cp -a /tmp/merged/. / 2>/dev/null; rm -rf /tmp/merged\n")

	for _, change := range changes {
		df.WriteString(change + "\n")
	}

	if err := os.WriteFile(dockerfilePath, []byte(df.String()), 0644); err != nil {
		return "", fmt.Errorf("writing Dockerfile: %w", err)
	}

	defer os.Remove(dockerfilePath) //nolint:errcheck

	args := []string{"build", "-f", dockerfilePath, "-t", outputRef, "."}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = mergedDir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}

	// Retrieve the new image's ID.
	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}", outputRef)
	out, err := inspectCmd.Output()
	if err != nil {
		return outputRef, nil
	}

	return strings.TrimSpace(string(out)), nil
}

// jsonArr marshals a string slice to JSON for use in docker --change flags.
func jsonArr(s []string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Close is a no-op since we use the docker CLI instead of a persistent connection.
func (c *Client) Close() error {
	return nil
}
