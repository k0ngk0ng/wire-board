package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type sourceInput struct {
	File   string
	Source string
}

func main() {
	var inputs []sourceInput
	if err := json.NewDecoder(os.Stdin).Decode(&inputs); err != nil {
		panic(err)
	}
	results := make([][]string, len(inputs))
	pattern := regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9]*|_)$`)
	for index, input := range inputs {
		issues := []string{}
		file, err := parser.ParseFile(token.NewFileSet(), input.File, input.Source, parser.ParseComments|parser.AllErrors)
		if err != nil {
			issues = append(issues, "invalid Go syntax")
		}
		if file != nil {
			directory := strings.Split(input.File, "/")[2]
			packages := map[string]string{"server": "yuyuhakusho", "engine": "engine", "sessiontick": "sessiontick", "platformtest": "platformtest", "build": "main"}
			if file.Name.Name != packages[directory] {
				issues = append(issues, "package differs from directory")
			}
			if len(file.Comments) != 0 {
				issues = append(issues, "prohibited code comment")
			}
			ast.Inspect(file, func(node ast.Node) bool {
				if identifier, ok := node.(*ast.Ident); ok && !pattern.MatchString(identifier.Name) {
					issues = append(issues, "invalid Go identifier: "+identifier.Name)
				}
				return true
			})
			for _, imported := range file.Imports {
				name, _ := strconv.Unquote(imported.Path.Value)
				if name == "unsafe" || imported.Name != nil && imported.Name.Name == "." {
					issues = append(issues, "prohibited import")
				}
				if strings.Contains(name, "/wire-board/games/") && !strings.Contains(name, "/wire-board/games/yuyuhakusho/") {
					issues = append(issues, "prohibited cross-game import")
				}
			}
		}
		results[index] = issues
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		panic(err)
	}
}
