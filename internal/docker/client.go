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

type Client struct {
	platform string
}

func NewClient(platform string) (*Client, error) {
	if err := checkDocker(); err != nil {
		return nil, err
	}
	return &Client{platform: platform}, nil
}

func checkDocker() error {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docker not available: %w\nMake sure Docker is installed and running", err)
	}
	_ = out
	return nil
}

type ImageMetadata struct {
	ID        string            `json:"id"`
	Config    *ImageConfig      `json:"config"`
	RawConfig json.RawMessage   `json:"raw_config"`
	Platform  string            `json:"platform"`
	Size      int64             `json:"size"`
}

type ImageConfig struct {
	Env          []string            `json:"Env"`
	Cmd          []string            `json:"Cmd"`
	Entrypoint   []string            `json:"Entrypoint"`
	WorkingDir   string              `json:"WorkingDir"`
	User         string              `json:"User"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	Volumes      map[string]struct{} `json:"Volumes"`
	Labels       map[string]string   `json:"Labels"`
	StopSignal   string              `json:"StopSignal"`
}

type inspectResult struct {
	Id              string              `json:"Id"`
	Size            int64               `json:"Size"`
	Os              string              `json:"Os"`
	Architecture    string              `json:"Architecture"`
	Config          *ImageConfig        `json:"Config"`
}

func (c *Client) EnsureImage(ctx context.Context, imageName string) error {
	args := []string{"image", "inspect", imageName}
	cmd := exec.CommandContext(ctx, "docker", args...)
	if err := cmd.Run(); err != nil {
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
		ID:        info.Id,
		Config:    info.Config,
		RawConfig: rawCfg,
		Platform:  platStr,
		Size:      info.Size,
	}, nil
}

func (c *Client) ExtractFilesystem(ctx context.Context, imageName, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	containerName := fmt.Sprintf("dim-extract-%d", os.Getpid())

	createArgs := []string{"create", "--name", containerName, imageName, "true"}
	cmd := exec.CommandContext(ctx, "docker", createArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating temp container: %w\n%s", err, string(out))
	}

	defer func() {
		rmCmd := exec.Command("docker", "rm", "-f", containerName)
		rmCmd.Run()
	}()

	exportCmd := exec.CommandContext(ctx, "docker", "export", containerName)
	tarPath := filepath.Join(destDir, "export.tar")
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("creating tar file: %w", err)
	}
	exportCmd.Stdout = tarFile
	exportCmd.Stderr = os.Stderr
	if err := exportCmd.Run(); err != nil {
		tarFile.Close()
		return fmt.Errorf("exporting container: %w", err)
	}
	tarFile.Close()

	fmt.Fprintf(os.Stderr, "Extracting %s...\n", imageName)
	extractCmd := exec.CommandContext(ctx, "tar", "xf", tarPath, "-C", destDir)
	extractCmd.Stderr = os.Stderr
	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("extracting tar: %w", err)
	}

	os.Remove(tarPath)

	return nil
}

func (c *Client) ImportSquashed(ctx context.Context, tarDir, outputRef string, metadata *ImageMetadata, changes []string, message string) (string, error) {
	args := []string{"import"}

	if message != "" {
		args = append(args, "-m", message)
	}

	for _, change := range changes {
		args = append(args, "-c", change)
	}

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

	args = append(args, "-", outputRef)

	tarDirPath := filepath.Join(tarDir)
	tarCmd := exec.CommandContext(ctx, "tar", "cf", "-", "-C", tarDirPath, ".")
	importCmd := exec.CommandContext(ctx, "docker", args...)
	importCmd.Stdin, _ = tarCmd.StdoutPipe()
	importCmd.Stderr = os.Stderr

	if err := importCmd.Start(); err != nil {
		return "", fmt.Errorf("docker import: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		return "", fmt.Errorf("tar: %w", err)
	}

	tarCmd.Wait()
	importCmd.Wait()

	if importCmd.Process != nil && importCmd.ProcessState != nil && !importCmd.ProcessState.Success() {
		return "", fmt.Errorf("docker import failed")
	}

	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}", outputRef)
	out, err := inspectCmd.Output()
	if err != nil {
		return outputRef, nil
	}

	return strings.TrimSpace(string(out)), nil
}

func (c *Client) BuildLayered(ctx context.Context, baseImage, mergedDir, outputRef string, metadata *ImageMetadata, changes []string) (string, error) {
	dockerfilePath := filepath.Join(mergedDir, "Dockerfile.merge")

	var df strings.Builder
	df.WriteString(fmt.Sprintf("FROM %s\n", baseImage))
	df.WriteString("COPY . /tmp/merged/\n")
	df.WriteString("RUN cp -a /tmp/merged/. / && rm -rf /tmp/merged\n")

	for _, change := range changes {
		df.WriteString(change + "\n")
	}

	if err := os.WriteFile(dockerfilePath, []byte(df.String()), 0644); err != nil {
		return "", fmt.Errorf("writing Dockerfile: %w", err)
	}

	defer os.Remove(dockerfilePath)

	args := []string{"build", "-f", dockerfilePath, "-t", outputRef, "."}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = mergedDir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}

	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}", outputRef)
	out, err := inspectCmd.Output()
	if err != nil {
		return outputRef, nil
	}

	return strings.TrimSpace(string(out)), nil
}

func jsonArr(s []string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (c *Client) Close() error {
	return nil
}
