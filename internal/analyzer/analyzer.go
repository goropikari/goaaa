package analyzer

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const ruleID = "AAA001"

type Diagnostic struct {
	File, Message string
	Line, Column  int
}

func (d Diagnostic) Text() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", d.File, d.Line, d.Column, ruleID, d.Message)
}

type (
	marker struct{ phase, line, column int }
	span   struct{ start, end token.Pos }
)

var markerPattern = regexp.MustCompile(`(?i)^(arrange|act|assert)(?:\s*:|\s|$)`)

func AnalyzeFiles(files []string) ([]Diagnostic, error) {
	fset := token.NewFileSet()

	var all []Diagnostic

	for _, filename := range files {
		f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}

		all = append(all, analyzeFile(filename, fset, f)...)
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}

		return all[i].Line < all[j].Line
	})

	return all, nil
}

func analyzeFile(filename string, fset *token.FileSet, file *ast.File) []Diagnostic {
	var out []Diagnostic

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}

		out = append(out, inspectScope(filename, fset, file, fn.Body, nestedRunScopes(fn.Body))...)
	}

	return out
}

func nestedRunScopes(body *ast.BlockStmt) []*ast.BlockStmt {
	var scopes []*ast.BlockStmt

	ast.Inspect(body, func(n ast.Node) bool {
		if n == body {
			return true
		}

		if _, ok := n.(*ast.FuncLit); ok {
			// A callback is its own analysis scope. Descendants are collected
			// when that callback is inspected, so do not collect them twice.
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}

		fn, ok := call.Args[1].(*ast.FuncLit)
		if ok && fn.Body != nil {
			scopes = append(scopes, fn.Body)
		}

		return true
	})

	return scopes
}

func inspectScope(filename string, fset *token.FileSet, file *ast.File, body *ast.BlockStmt, children []*ast.BlockStmt) []Diagnostic {
	var excluded []span
	for _, child := range children {
		excluded = append(excluded, span{child.Pos(), child.End()})
	}

	var markers []marker

	for _, group := range file.Comments {
		for _, c := range group.List {
			if c.Pos() <= body.Lbrace || c.Pos() >= body.Rbrace || inside(c.Pos(), excluded) {
				continue
			}

			match := markerPattern.FindStringSubmatch(commentText(c.Text))
			if len(match) == 0 {
				continue
			}

			phase := map[string]int{"arrange": 0, "act": 1, "assert": 2}[strings.ToLower(match[1])]
			p := fset.Position(c.Pos())
			markers = append(markers, marker{phase, p.Line, p.Column})
		}
	}

	sort.SliceStable(markers, func(i, j int) bool { return markers[i].line < markers[j].line })

	var out []Diagnostic

	maxPhase, lastReported := -1, -1
	for _, m := range markers {
		if m.phase < maxPhase && m.phase != lastReported {
			previous := phaseName(maxPhase)
			current := phaseName(m.phase)
			out = append(out, Diagnostic{filename, fmt.Sprintf("%s phase appears after %s; expected Arrange → Act → Assert order", current, previous), m.line, m.column})
			lastReported = m.phase
		}

		if m.phase > maxPhase {
			maxPhase = m.phase
			lastReported = -1
		}
	}

	for _, child := range children {
		out = append(out, inspectScope(filename, fset, file, child, nestedRunScopes(child))...)
	}

	return out
}

func phaseName(phase int) string {
	switch phase {
	case 0:
		return "Arrange"
	case 1:
		return "Act"
	default:
		return "Assert"
	}
}

func inside(pos token.Pos, spans []span) bool {
	for _, s := range spans {
		if pos > s.start && pos < s.end {
			return true
		}
	}

	return false
}

func commentText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "//")
	s = strings.TrimPrefix(s, "/*")
	s = strings.TrimSuffix(s, "*/")

	return strings.TrimSpace(s)
}

func CollectGoFiles(args []string) ([]string, error) {
	seen := map[string]bool{}

	var files []string

	add := func(path string) {
		if !seen[path] && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".gen.go") {
			seen[path] = true
			files = append(files, path)
		}
	}

	for _, arg := range args {
		if strings.HasSuffix(arg, "/...") || arg == "..." {
			root := strings.TrimSuffix(arg, "/...")
			if root == "" {
				root = "."
			}

			err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !info.IsDir() {
					add(path)
				}

				return nil
			})
			if err != nil {
				return nil, err
			}

			continue
		}

		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			entries, err := os.ReadDir(arg)
			if err != nil {
				return nil, err
			}

			for _, e := range entries {
				if !e.IsDir() {
					add(filepath.Join(arg, e.Name()))
				}
			}
		} else {
			add(arg)
		}
	}

	sort.Strings(files)

	if len(files) == 0 {
		return nil, fmt.Errorf("no Go files found")
	}

	return files, nil
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}
type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}
type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}
type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}
type sarifRule struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
}
type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}
type sarifMessage struct {
	Text string `json:"text"`
}
type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}
type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}
type sarifArtifact struct {
	URI string `json:"uri"`
}
type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

func WriteSARIF(w io.Writer, ds []Diagnostic) error {
	results := make([]sarifResult, 0, len(ds))
	for _, d := range ds {
		results = append(results, sarifResult{ruleID, "error", sarifMessage{d.Message}, []sarifLocation{{sarifPhysical{sarifArtifact{d.File}, sarifRegion{d.Line, d.Column}}}}})
	}

	log := sarifLog{"2.1.0", "https://json.schemastore.org/sarif-2.1.0.json", []sarifRun{{sarifTool{sarifDriver{"goaaa", []sarifRule{{ruleID, sarifMessage{"AAA marker order"}}}}}, results}}}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(log)
}
