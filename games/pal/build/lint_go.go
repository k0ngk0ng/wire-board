package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
)

func main() {
	var input struct{ Name, Source string }
	issues := []string{}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		issues = append(issues, err.Error())
	} else {
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, input.Name, input.Source, parser.ParseComments|parser.AllErrors)
		if err != nil {
			issues = append(issues, err.Error())
		}
		if file != nil {
			for _, group := range file.Comments {
				for _, comment := range group.List {
					issues = append(issues, fmt.Sprintf("%s: prohibited code comment", positions.Position(comment.Pos())))
				}
			}
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(issues); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
