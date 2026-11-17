package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

type sourceInput struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

func inspectSource(input sourceInput) []string {
	errors := []string{}
	f, err := parser.ParseFile(token.NewFileSet(), input.Path, input.Source, parser.ParseComments)
	if err != nil {
		return []string{"syntax"}
	}
	want := "racing"
	if strings.HasPrefix(input.Path, "internal/server/") {
		want = "server"
	}
	if strings.Contains(input.Path, "/build/") {
		want = "main"
	} else if strings.Contains(input.Path, "/tests/") {
		want = "racing_test"
	}
	if f.Name.Name != want {
		errors = append(errors, "package-directory")
	}
	for _, group := range f.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, "//go:embed ") {
				errors = append(errors, "no-comments")
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if strings.Contains(node.Name.Name, "_") {
				errors = append(errors, "camel-case-function")
			}
		case *ast.TypeSpec:
			if strings.Contains(node.Name.Name, "_") {
				errors = append(errors, "camel-case-type")
			}
		case *ast.Ident:
			if node.Obj != nil && node.Name != "_" && strings.Contains(node.Name, "_") {
				errors = append(errors, "camel-case-binding")
			}
		}
		return true
	})
	return errors
}

func main() {
	var inputs []sourceInput
	if err := json.NewDecoder(os.Stdin).Decode(&inputs); err != nil {
		panic(err)
	}
	out := make([][]string, len(inputs))
	for i, input := range inputs {
		out[i] = inspectSource(input)
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		panic(err)
	}
}
