package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/goropikari/goaaa/internal/analyzer"

	"github.com/spf13/cobra"
)

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

type exitCode int

func (e exitCode) Error() string { return "" }

func execute(args []string, stdout, stderr io.Writer) int {
	var format string

	root := &cobra.Command{
		Use:   "goaaa [flags] <file|directory> [... ]",
		Short: "Check Go tests for Arrange–Act–Assert marker order",
		Args: func(cmd *cobra.Command, positional []string) error {
			if len(positional) == 0 {
				return fmt.Errorf("requires a file or directory, unless the diff subcommand is used")
			}

			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, positional []string) error {
			files, err := analyzer.CollectGoFiles(positional)
			if err != nil {
				return err
			}

			return analyze(files, format, stdout, stderr)
		},
	}
	root.PersistentFlags().StringVar(&format, "format", "text", "output format: text or sarif")
	root.AddCommand(newDiffCommand(&format, stdout, stderr))
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		code := 2
		if e, ok := err.(exitCode); ok {
			code = int(e)
		} else {
			fmt.Fprintf(stderr, "goaaa: %v\n", err)
		}

		return code
	}

	return 0
}

func newDiffCommand(format *string, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "diff [<range>] [--] [<path>...]",
		Short: "Analyze Go files changed by git diff",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, positional []string) error {
			files, err := collectDiffFiles(positional)
			if err != nil {
				return err
			}

			return analyze(files, *format, stdout, stderr)
		},
	}
}

func analyze(files []string, format string, stdout, stderr io.Writer) error {
	if format != "text" && format != "sarif" {
		return fmt.Errorf("unsupported format %q (use text or sarif)", format)
	}

	diagnostics, err := analyzer.AnalyzeFiles(files)
	if err != nil {
		return err
	}

	if format == "sarif" {
		if err := analyzer.WriteSARIF(stdout, diagnostics); err != nil {
			return err
		}
	} else {
		for _, diagnostic := range diagnostics {
			fmt.Fprintln(stderr, diagnostic.Text())
		}
	}

	if len(diagnostics) > 0 {
		return exitCode(1)
	}

	return nil
}

func collectDiffFiles(positional []string) ([]string, error) {
	var gitArgs []string
	if len(positional) > 0 && strings.Contains(positional[0], "..") {
		gitArgs = append(gitArgs, positional[0])
		positional = positional[1:]
	}

	gitArgs = append(gitArgs, "--")
	gitArgs = append(gitArgs, positional...)
	args := []string{"diff", "--name-only", "--diff-filter=ACMR", "-z"}
	args = append(args, gitArgs...)

	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	paths := bytes.Split(output, []byte{0})

	goFiles := make([]string, 0, len(paths))
	for _, path := range paths {
		name := string(path)
		if name == "" {
			continue
		}

		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".gen.go") {
			goFiles = append(goFiles, name)
		}
	}

	sort.Strings(goFiles)

	return goFiles, nil
}
