package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	var input struct {
		Name                      string
		Source                    string
		ProhibitedEngineSelectors map[string][]string
	}
	issues := []string{}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		issues = append(issues, err.Error())
	} else {
		files := token.NewFileSet()
		file, err := parser.ParseFile(files, input.Name, input.Source, parser.ParseComments|parser.AllErrors)
		if err != nil {
			issues = append(issues, err.Error())
		}
		if file != nil {
			selectors := map[string]map[string]bool{}
			engine := strings.HasPrefix(input.Name, "games/pokepath/engine/") && !strings.HasSuffix(input.Name, "_test.go")
			for _, imported := range file.Imports {
				path, _ := strconv.Unquote(imported.Path.Value)
				if !engine || len(input.ProhibitedEngineSelectors[path]) == 0 {
					continue
				}
				name := path[strings.LastIndex(path, "/")+1:]
				if imported.Name != nil {
					name = imported.Name.Name
				}
				if name == "." {
					issues = append(issues, fmt.Sprintf("%s: prohibited engine dot import %s", files.Position(imported.Pos()), path))
				}
				selectors[name] = map[string]bool{}
				for _, selector := range input.ProhibitedEngineSelectors[path] {
					selectors[name][selector] = true
				}
			}
			for _, group := range file.Comments {
				issues = append(issues, fmt.Sprintf("%s: prohibited code comment", files.Position(group.Pos())))
			}
			pattern := regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9]*|_)$`)
			ast.Inspect(file, func(node ast.Node) bool {
				if selector, ok := node.(*ast.SelectorExpr); ok {
					if owner, ok := selector.X.(*ast.Ident); ok && selectors[owner.Name][selector.Sel.Name] {
						issues = append(issues, fmt.Sprintf("%s: prohibited engine selector %s.%s", files.Position(selector.Pos()), owner.Name, selector.Sel.Name))
					}
				}
				if value, ok := node.(*ast.Ident); ok && !pattern.MatchString(value.Name) {
					if value == file.Name && strings.HasSuffix(value.Name, "_test") && pattern.MatchString(strings.TrimSuffix(value.Name, "_test")) {
						return true
					}
					issues = append(issues, fmt.Sprintf("%s: invalid Go identifier %s", files.Position(value.Pos()), value.Name))
				}
				return true
			})
			for _, imported := range file.Imports {
				path, _ := strconv.Unquote(imported.Path.Value)
				if strings.Contains(path, "/wire-board/games/") && !strings.Contains(path, "/wire-board/games/pokepath/") {
					issues = append(issues, fmt.Sprintf("%s: prohibited cross-game import", files.Position(imported.Pos())))
				}
			}
		}
	}
	json.NewEncoder(os.Stdout).Encode(issues)
}
