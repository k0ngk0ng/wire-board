package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type sourceInput struct {
	File            string `json:"file"`
	Source          string `json:"source"`
	ExpectedPackage string `json:"expectedPackage"`
	ChangedLines    []int  `json:"changedLines"`
}

func inspectSource(input sourceInput) []string {
	issues := []string{}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, input.File, input.Source, parser.ParseComments)
	if err != nil {
		return []string{"syntax"}
	}
	changed := make(map[int]bool)
	for _, line := range input.ChangedLines {
		changed[line] = true
	}
	active := func(node ast.Node) bool {
		if input.ChangedLines == nil {
			return true
		}
		for line, last := positions.Position(node.Pos()).Line, positions.Position(node.End()).Line; line <= last; line++ {
			if changed[line] {
				return true
			}
		}
		return false
	}
	record := func(issue string, node ast.Node) {
		if active(node) {
			issues = append(issues, issue)
		}
	}
	parts := strings.Split(input.File, "/")
	want := input.ExpectedPackage
	if want == "" {
		want = "cs16"
		if len(parts) > 2 {
			switch parts[2] {
			case "engine", "data":
				want = parts[2]
			case "build", "cmd":
				want = "main"
			case "tests":
				want = "cs16_test"
			}
		}
	}
	if file.Name.Name != want && !(strings.HasSuffix(input.File, "_test.go") && file.Name.Name == want+"_test") {
		issues = append(issues, "package-directory")
	}
	for _, group := range file.Comments {
		record("no-comments", group)
	}
	packages := make(map[string]string)
	dotPackages := make(map[string]bool)
	for _, imported := range file.Imports {
		name, _ := strconv.Unquote(imported.Path.Value)
		alias := path.Base(name)
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if alias == "." {
			dotPackages[name] = true
		} else {
			packages[alias] = name
		}
		if name == "unsafe" {
			record("no-unsafe", imported)
		}
		if strings.Contains(name, "/games/") && (strings.HasPrefix(input.File, "internal/") || !strings.Contains(name, "/games/cs16/")) {
			record("no-cross-game-import", imported)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.Field:
			for _, name := range current.Names {
				if name.Name != "_" && strings.Contains(name.Name, "_") {
					record("camel-case-binding", name)
				}
			}
		case *ast.Ident:
			if current.Obj != nil && current.Name != "_" && strings.Contains(current.Name, "_") {
				record("camel-case-binding", current)
			}
			if current.Name == "float32" || current.Name == "float64" || current.Name == "complex64" || current.Name == "complex128" {
				record("integer-business-number", current)
			}
			if current.Obj == nil && (current.Name == "Register" && dotPackages["database/sql"] || current.Name == "RegisterConnectionHook" && dotPackages["modernc.org/sqlite"]) {
				record("no-global-database-registry", current)
			}
		case *ast.SelectorExpr:
			if owner, ok := current.X.(*ast.Ident); ok && (owner.Obj == nil || owner.Obj.Kind == ast.Pkg) {
				if packages[owner.Name] == "database/sql" && current.Sel.Name == "Register" || packages[owner.Name] == "modernc.org/sqlite" && current.Sel.Name == "RegisterConnectionHook" {
					record("no-global-database-registry", current)
				}
			}
		case *ast.BasicLit:
			if current.Kind == token.FLOAT || current.Kind == token.IMAG {
				record("integer-business-number", current)
			}
			if current.Kind == token.STRING && !strings.Contains(input.File, "/tests/") && !strings.HasSuffix(input.File, "_test.go") {
				value, _ := strconv.Unquote(current.Value)
				for _, match := range regexp.MustCompile(`\.local/([a-z][a-z0-9_-]*)(?:/|$|\s)`).FindAllStringSubmatch(value, -1) {
					if match[1] != "cs16" {
						record("local-output-directory", current)
					}
				}
				if regexp.MustCompile(`^(?:/tmp/.+|/private/tmp/.+|~/.+)`).MatchString(value) {
					record("workspace-output-directory", current)
				}
			}
		case *ast.CallExpr:
			if selector, ok := current.Fun.(*ast.SelectorExpr); ok {
				if owner, ok := selector.X.(*ast.Ident); ok && owner.Name == "math" && selector.Sel.Name != "MaxInt" && selector.Sel.Name != "MaxInt64" && selector.Sel.Name != "MinInt" && selector.Sel.Name != "MinInt64" {
					record("integer-business-number", current)
				}
				if owner, ok := selector.X.(*ast.Ident); ok && owner.Name == "os" && (selector.Sel.Name == "TempDir" || selector.Sel.Name == "UserHomeDir") {
					record("workspace-output-directory", current)
				}
			}
		}
		return true
	})
	if path.Base(input.File) == "lint_go.go" && file.Name.Name != "main" {
		issues = append(issues, "package-directory")
	}
	return issues
}

func main() {
	var inputs []sourceInput
	if err := json.NewDecoder(os.Stdin).Decode(&inputs); err != nil {
		panic(err)
	}
	results := make([][]string, len(inputs))
	for index, input := range inputs {
		results[index] = inspectSource(input)
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		panic(err)
	}
}
