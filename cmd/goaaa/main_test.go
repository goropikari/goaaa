package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCommandFixture(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture_test.go")
	require.NoError(t, os.WriteFile(path, []byte(source), 0o600))

	return path
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name       string
		args       func(path string) []string
		source     string
		wantCode   int
		wantStderr string
		wantSARIF  bool
	}{
		{
			name: "when markers are ordered, exit code 0 is returned",
			args: func(path string) []string { return []string{path} },
			source: `package fixture
import "testing"
func TestValue(t *testing.T) {
  // Arrange
  value := 1
  // Act
  got := value + 1
  // Assert
  if got != 2 { t.Fatal(got) }
}`,
			wantCode: 0,
		},
		{
			name: "when phase order is invalid, exit code 1 and a text diagnostic are returned",
			args: func(path string) []string { return []string{path} },
			source: `package fixture
import "testing"
func TestValue(t *testing.T) {
  // Assert
  // Act
  got := 2
  if got != 2 { t.Fatal(got) }
}`,
			wantCode:   1,
			wantStderr: "AAA001",
		},
		{
			name: "when SARIF format is requested for invalid phases, exit code 1 and structured stdout are returned",
			args: func(path string) []string { return []string{"--format", "sarif", path} },
			source: `package fixture
import "testing"
func TestValue(t *testing.T) {
  // Assert
  // Act
}`,
			wantCode:  1,
			wantSARIF: true,
		},
		{
			name:       "when input is missing, exit code 2 and a usage error are returned",
			args:       func(string) []string { return nil },
			wantCode:   2,
			wantStderr: "requires a file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := ""
			if tt.source != "" {
				path = writeCommandFixture(t, tt.source)
			}

			var stdout, stderr strings.Builder

			// Act
			code := execute(tt.args(path), &stdout, &stderr)

			// Assert
			assert.Equal(t, tt.wantCode, code)

			if tt.wantSARIF {
				var document map[string]any
				require.NoError(t, json.Unmarshal([]byte(stdout.String()), &document))
				assert.Equal(t, "2.1.0", document["version"])
				assert.Empty(t, stderr.String())
			} else {
				assert.Contains(t, stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestExecuteDiffAnalyzesOnlyChangedGoFiles(t *testing.T) {
	t.Run("when only one Go file changes, only that file's diagnostic is reported", func(t *testing.T) {
		// Arrange
		repo := t.TempDir()
		runGit(t, repo, "init", "-q")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "clean_test.go"), []byte(`package fixture
import "testing"
func TestClean(t *testing.T) {
  // Arrange
  // Act
  // Assert
}`), 0o600))
		badPath := filepath.Join(repo, "bad_test.go")
		require.NoError(t, os.WriteFile(badPath, []byte(`package fixture
import "testing"
func TestBad(t *testing.T) {
  // Arrange
  // Act
  // Assert
}`), 0o600))
		runGit(t, repo, "add", "clean_test.go", "bad_test.go")
		runGit(t, repo, "-c", "user.name=goaaa-test", "-c", "user.email=goaaa@example.com", "commit", "-qm", "baseline")
		require.NoError(t, os.WriteFile(badPath, []byte(`package fixture
import "testing"
func TestBad(t *testing.T) {
  // Assert
  // Act
}`), 0o600))

		t.Chdir(repo)

		var stdout, stderr strings.Builder

		// Act
		code := execute([]string{"diff"}, &stdout, &stderr)

		// Assert
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr.String(), "bad_test.go")
		assert.Contains(t, stderr.String(), "AAA001")
		assert.NotContains(t, stderr.String(), "clean_test.go")
		assert.Empty(t, stdout.String())
	})
}

func TestCollectFilesDiff(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{
			name:     "when a range is specified, files changed in the range are returned",
			args:     []string{"main..HEAD"},
			wantPath: "range_test.go",
		},
		{
			name:     "when a path is specified, only files changed at that path are returned",
			args:     []string{"selected_test.go"},
			wantPath: "selected_test.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := t.TempDir()
			runGit(t, repo, "init", "-q")
			require.NoError(t, os.WriteFile(filepath.Join(repo, "range_test.go"), []byte("package fixture\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(repo, "selected_test.go"), []byte("package fixture\n"), 0o600))
			runGit(t, repo, "add", ".")
			runGit(t, repo, "-c", "user.name=goaaa-test", "-c", "user.email=goaaa@example.com", "commit", "-qm", "baseline")
			require.NoError(t, os.WriteFile(filepath.Join(repo, "range_test.go"), []byte("package fixture\n\n// changed\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(repo, "selected_test.go"), []byte("package fixture\n\n// changed\n"), 0o600))
			t.Chdir(repo)

			args := tt.args
			if tt.args[0] == "main..HEAD" {
				runGit(t, repo, "add", "range_test.go")
				runGit(t, repo, "-c", "user.name=goaaa-test", "-c", "user.email=goaaa@example.com", "commit", "-qm", "range change")

				args = []string{"HEAD~1..HEAD"}
			}

			// Act
			got, err := collectDiffFiles(args)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, []string{tt.wantPath}, got)
		})
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir
	require.NoError(t, command.Run())
}
