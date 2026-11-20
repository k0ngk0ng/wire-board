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
	File   string `json:"file"`
	Source string `json:"source"`
}

func inspectSource(input sourceInput) []string {
	issues := []string{}
	file, err := parser.ParseFile(token.NewFileSet(), input.File, input.Source, parser.ParseComments)
	if err != nil {
		return []string{"syntax"}
	}
	parts := strings.Split(input.File, "/")
	want := "hotlinemiami"
	if len(parts) > 2 {
		switch parts[2] {
		case "engine", "data":
			want = parts[2]
		case "build", "cmd":
			want = "main"
		case "tests":
			want = "hotlinemiami_test"
		}
	}
	if file.Name.Name != want && !(strings.HasSuffix(input.File, "_test.go") && file.Name.Name == want+"_test") {
		issues = append(issues, "package-directory")
	}
	if len(file.Comments) != 0 {
		issues = append(issues, "no-comments")
	}
	for _, imported := range file.Imports {
		name, _ := strconv.Unquote(imported.Path.Value)
		if strings.Contains(name, "/games/") && !strings.Contains(name, "/games/hotlinemiami/") {
			issues = append(issues, "no-cross-game-import")
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.Field:
			for _, name := range current.Names {
				if name.Name != "_" && strings.Contains(name.Name, "_") {
					issues = append(issues, "camel-case-binding")
				}
			}
		case *ast.Ident:
			if current.Obj != nil && current.Name != "_" && strings.Contains(current.Name, "_") {
				issues = append(issues, "camel-case-binding")
			}
			if current.Name == "float32" || current.Name == "float64" || current.Name == "complex64" || current.Name == "complex128" {
				issues = append(issues, "integer-business-number")
			}
		case *ast.BasicLit:
			if current.Kind == token.FLOAT || current.Kind == token.IMAG {
				issues = append(issues, "integer-business-number")
			}
			if current.Kind == token.STRING && !strings.Contains(input.File, "/tests/") && !strings.HasSuffix(input.File, "_test.go") {
				value, _ := strconv.Unquote(current.Value)
				for _, match := range regexp.MustCompile(`\.local/([a-z][a-z0-9_-]*)(?:/|$|\s)`).FindAllStringSubmatch(value, -1) {
					if match[1] != "hotlinemiami" {
						issues = append(issues, "local-output-directory")
					}
				}
				if regexp.MustCompile(`^(?:/tmp/.+|/private/tmp/.+|~/.+)`).MatchString(value) {
					issues = append(issues, "workspace-output-directory")
				}
			}
		case *ast.CallExpr:
			if selector, ok := current.Fun.(*ast.SelectorExpr); ok {
				if owner, ok := selector.X.(*ast.Ident); ok && owner.Name == "os" && (selector.Sel.Name == "TempDir" || selector.Sel.Name == "UserHomeDir") {
					issues = append(issues, "workspace-output-directory")
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
