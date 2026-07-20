package analyzer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goropikari/goaaa/internal/analyzer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFixture(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()

	path := filepath.Join(dir, "example_test.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestAnalyzeFilesAcceptsOrderedAndRepeatedPhases(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "when markers are ordered with a repeated Arrange phase, no diagnostics are reported",
			source: `package example
import "testing"
func TestOK(t *testing.T) {
  // arrange: inputs
  // Arrange again
  // act
  // assert: result
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := writeFixture(t, tt.source)

			// Act
			diagnostics, err := analyzer.AnalyzeFiles([]string{path})

			// Assert
			require.NoError(t, err)
			assert.Empty(t, diagnostics)
		})
	}
}

func TestAnalyzeFilesReportsOrderAndSubtestIndependently(t *testing.T) {
	t.Run("when parent and child have different phase orders, only the parent diagnostic is reported", func(t *testing.T) {
		// Arrange
		path := writeFixture(t, `package example
import "testing"
func TestParent(t *testing.T) {
  // Arrange
	  t.Run("when child Assert precedes Act, the child diagnostic is evaluated independently", func(t *testing.T) {
    // Assert
    // Act
  })
  // Act
  // Assert
}`)

		// Act
		diagnostics, err := analyzer.AnalyzeFiles([]string{path})

		// Assert
		require.NoError(t, err)
		require.Len(t, diagnostics, 1)
		assert.Contains(t, diagnostics[0].Message, "Act phase appears after Assert")
	})
}

func TestWriteSARIF(t *testing.T) {
	t.Run("when a diagnostic is provided, SARIF output contains version 2.1.0", func(t *testing.T) {
		// Arrange
		var output strings.Builder

		// Act
		err := analyzer.WriteSARIF(&output, []analyzer.Diagnostic{{File: "x_test.go", Line: 4, Column: 3, Message: "bad order"}})

		// Assert
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(output.String()), &got))
		assert.Equal(t, "2.1.0", got["version"])
	})
}
