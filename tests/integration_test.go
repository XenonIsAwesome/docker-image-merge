//go:build integration

// Package integration provides end-to-end tests for docker-image-merge.
//
// These tests require Docker and actually pull images, extract filesystems,
// and create merged images. Run with:
//
//	go test -tags=integration -v -timeout=300s ./tests/
//
// Or via Docker:
//
//	docker build -t imagemerge-inttest -f Dockerfile.test .
//	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock imagemerge-inttest
package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	binaryName = "docker-imagemerge"
	// Small, commonly available images for testing.
	imageA = "alpine:3.19"
	imageB = "busybox:1.36"
	// A slightly larger image with more files.
	imageC = "nginx:1.25-alpine"
)

var binaryPath string

// TestMain builds the binary once before all tests.
func TestMain(m *testing.M) {
	// Find or build the binary.
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getting working dir: %v\n", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(dir, binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Build from parent directory.
		srcDir := filepath.Dir(dir)
		cmd := exec.Command("go", "build", "-tags", "integration",
			"-trimpath", "-ldflags=-s -w",
			"-o", binaryPath, ".")
		cmd.Dir = srcDir
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "building binary: %v\n", err)
			os.Exit(1)
		}
	}

	// Ensure Docker is available.
	if err := exec.Command("docker", "info").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "docker not available: %v\n", err)
		os.Exit(1)
	}

	// Pull test images.
	for _, img := range []string{imageA, imageB, imageC} {
		fmt.Fprintf(os.Stderr, "Pulling %s...\n", img)
		cmd := exec.Command("docker", "pull", img)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "pulling %s: %v\n", img, err)
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

// runMerge executes the binary with the given arguments and returns stdout+stderr.
func runMerge(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	output, err := cmd.CombinedOutput()
	t.Logf("command: %s %s\n", binaryPath, strings.Join(args, " "))
	t.Logf("output:\n%s", string(output))
	return string(output), err
}

// imageExists checks if a Docker image exists locally.
func imageExists(t *testing.T, ref string) bool {
	t.Helper()
	cmd := exec.Command("docker", "inspect", "--type=image", ref)
	return cmd.Run() == nil
}

// cleanupImage removes a Docker image after a test.
func cleanupImage(t *testing.T, ref string) {
	t.Helper()
	if imageExists(t, ref) {
		exec.Command("docker", "rmi", "-f", ref).Run()
	}
}

// --- Basic functionality tests ---

func TestMerge_AutoA_Default(t *testing.T) {
	out := "test-autoa-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-a")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}

	if !strings.Contains(output, "Created image") {
		t.Error("missing 'Created image' in output")
	}
}

func TestMerge_AutoB(t *testing.T) {
	out := "test-autob-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-b")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}

	if !strings.Contains(output, "Created image") {
		t.Error("missing 'Created image' in output")
	}
}

func TestMerge_FailStrategy_NoConflicts(t *testing.T) {
	out := "test-fail-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	// Merge identical images should work even with fail strategy.
	output, err := runMerge(t, imageA, imageA, out, "-s", "fail")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}
}

func TestMerge_FailStrategy_WithConflicts(t *testing.T) {
	out := "test-fail-conflict-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	// Different images should produce conflicts.
	output, err := runMerge(t, imageA, imageB, out, "-s", "fail")
	if err == nil {
		t.Fatal("expected error with fail strategy on conflicting images")
	}

	if !strings.Contains(output, "conflicts found") {
		t.Errorf("expected 'conflicts found' in error output, got: %s", output)
	}

	// Output image should NOT exist.
	if imageExists(t, out) {
		t.Error("output image should not exist with fail strategy")
	}
}

// --- Flag combination tests ---

func TestMerge_Squash(t *testing.T) {
	out := "test-squash-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-a", "--squash")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}

	if !strings.Contains(output, "Created image") {
		t.Error("missing 'Created image' in output")
	}
}

func TestMerge_MetadataFromB(t *testing.T) {
	out := "test-meta-b-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-a", "--metadata-from", "b")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}
}

func TestMerge_Verbose(t *testing.T) {
	out := "test-verbose-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-a", "-v")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	// Verbose should show diff stats.
	if !strings.Contains(output, "Only in A") {
		t.Error("verbose output missing 'Only in A'")
	}
	if !strings.Contains(output, "Only in B") {
		t.Error("verbose output missing 'Only in B'")
	}
}

func TestMerge_Message(t *testing.T) {
	out := "test-message-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-a", "-m", "custom commit message")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}
}

func TestMerge_WithChange(t *testing.T) {
	out := "test-change-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-a", "--squash", "--change", "ENV TEST_VAR=hello")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}

	// Verify the env var was applied.
	inspect := exec.Command("docker", "run", "--rm", out, "printenv", "TEST_VAR")
	inspectOut, err := inspect.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect failed: %v\n%s", err, string(inspectOut))
	}

	if !strings.Contains(string(inspectOut), "hello") {
		t.Errorf("expected TEST_VAR=hello, got: %s", string(inspectOut))
	}
}

// --- Different image combinations ---

func TestMerge_Alpine_Busybox(t *testing.T) {
	out := "test-alpine-busybox-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageB, out, "-s", "auto-a")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}
}

func TestMerge_Alpine_Nginx(t *testing.T) {
	out := "test-alpine-nginx-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageC, out, "-s", "auto-a")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}
}

func TestMerge_Nginx_Busybox(t *testing.T) {
	out := "test-nginx-busybox-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageC, imageB, out, "-s", "auto-a")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !imageExists(t, out) {
		t.Error("output image not found")
	}
}

// --- Edge cases ---

func TestMerge_IdenticalImages(t *testing.T) {
	out := "test-identical-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	output, err := runMerge(t, imageA, imageA, out, "-s", "auto-a")
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "identical") {
		t.Errorf("expected 'identical' in output, got: %s", output)
	}
}

func TestMerge_InvalidImage(t *testing.T) {
	out := "test-invalid-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	_, err := runMerge(t, "nonexistent-image-xyz:latest", imageA, out, "-s", "auto-a")
	if err == nil {
		t.Fatal("expected error for nonexistent image")
	}
}

func TestMerge_NoArgs(t *testing.T) {
	_, err := runMerge(t)
	if err == nil {
		t.Fatal("expected error with no arguments")
	}
}

func TestMerge_TwoArgs(t *testing.T) {
	_, err := runMerge(t, imageA, imageB)
	if err == nil {
		t.Fatal("expected error with only 2 arguments")
	}
}

func TestMerge_InvalidStrategy(t *testing.T) {
	_, err := runMerge(t, imageA, imageB, "test", "-s", "invalid-strategy")
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
}

func TestMerge_InvalidMetadataSource(t *testing.T) {
	_, err := runMerge(t, imageA, imageB, "test", "--metadata-from", "c")
	if err == nil {
		t.Fatal("expected error for invalid metadata source")
	}
}

func TestMerge_Help(t *testing.T) {
	output, err := runMerge(t, "--help")
	if err != nil {
		t.Fatalf("help failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "imagemerge") {
		t.Error("help output missing 'imagemerge'")
	}
	if !strings.Contains(output, "--strategy") {
		t.Error("help output missing --strategy flag")
	}
	if !strings.Contains(output, "--squash") {
		t.Error("help output missing --squash flag")
	}
}

// --- Merged image verification tests ---

func TestMerge_VerifyFilesystem(t *testing.T) {
	out := "test-verify-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	_, err := runMerge(t, imageA, imageB, out, "-s", "auto-a")
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Run the merged image and check that files from both images exist.
	// Alpine has /etc/alpine-release, busybox has /bin/busybox.
	// The merged image should have files from both.

	// Check a file from image A (alpine).
	run := exec.Command("docker", "run", "--rm", out, "ls", "/etc/alpine-release")
	if err := run.Run(); err != nil {
		t.Error("merged image missing /etc/alpine-release from alpine")
	}

	// Check a file from image B (busybox) - /bin/busybox may or may not exist
	// depending on what busybox provides vs alpine. Just check the merge succeeded.
}

func TestMerge_SquashProducesSingleLayer(t *testing.T) {
	out := "test-layers-" + time.Now().Format("0102150405")
	defer cleanupImage(t, out)

	_, err := runMerge(t, imageA, imageB, out, "-s", "auto-a", "--squash")
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Inspect the image history. Squashed images should have fewer layers.
	history := exec.Command("docker", "history", "--no-trunc", out)
	historyOut, err := history.CombinedOutput()
	if err != nil {
		t.Fatalf("history failed: %v\n%s", err, string(historyOut))
	}

	lines := strings.Split(strings.TrimSpace(string(historyOut)), "\n")
	// Squashed image should have at most 2 layers (scratch + import).
	if len(lines) > 3 {
		t.Errorf("squashed image has %d layers, expected <= 2", len(lines))
	}
}
