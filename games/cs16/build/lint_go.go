package main

import (
	"encoding/json"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
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
	StrictIntegers  bool   `json:"strictIntegers"`
}

var numericImporter = importer.Default()

func checkNumericComparisons(file *ast.File, positions *token.FileSet, packages map[string]string, record func(string, ast.Node)) {
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object)}
	config := types.Config{Importer: numericImporter, Error: func(error) {}}
	_, _ = config.Check(file.Name.Name, positions, []*ast.File{file}, info)
	floatType := func(value types.Type) bool {
		if value == nil {
			return false
		}
		basic, ok := value.Underlying().(*types.Basic)
		return ok && basic.Info()&(types.IsFloat|types.IsComplex) != 0
	}
	var functionName func(ast.Expr, int) string
	functionName = func(expression ast.Expr, depth int) string {
		if depth > 24 {
			return ""
		}
		switch current := expression.(type) {
		case *ast.SelectorExpr:
			if owner, ok := current.X.(*ast.Ident); ok && packages[owner.Name] != "" {
				return packages[owner.Name] + "." + current.Sel.Name
			}
		case *ast.Ident:
			if current.Obj == nil {
				return "builtin." + current.Name
			}
			switch declaration := current.Obj.Decl.(type) {
			case *ast.ValueSpec:
				for index, name := range declaration.Names {
					if name.Name == current.Name && index < len(declaration.Values) {
						return functionName(declaration.Values[index], depth+1)
					}
				}
			case *ast.AssignStmt:
				for index, left := range declaration.Lhs {
					if name, ok := left.(*ast.Ident); ok && name.Name == current.Name && index < len(declaration.Rhs) {
						return functionName(declaration.Rhs[index], depth+1)
					}
				}
			}
		case *ast.IndexExpr:
			return functionName(current.X, depth+1)
		case *ast.IndexListExpr:
			return functionName(current.X, depth+1)
		case *ast.ParenExpr:
			return functionName(current.X, depth+1)
		}
		return ""
	}
	type numericWrite struct {
		value ast.Expr
		end   token.Pos
	}
	writes := make(map[*ast.Object][]numericWrite)
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.ValueSpec:
			for index, name := range current.Names {
				if name.Obj != nil && index < len(current.Values) {
					writes[name.Obj] = append(writes[name.Obj], numericWrite{current.Values[index], current.End()})
				}
			}
		case *ast.AssignStmt:
			for index, left := range current.Lhs {
				if name, ok := left.(*ast.Ident); ok && name.Obj != nil && index < len(current.Rhs) {
					writes[name.Obj] = append(writes[name.Obj], numericWrite{current.Rhs[index], current.End()})
				}
			}
		}
		return true
	})
	var localFunction func(ast.Expr, int) (*ast.FuncType, *ast.BlockStmt)
	localFunction = func(expression ast.Expr, depth int) (*ast.FuncType, *ast.BlockStmt) {
		if depth > 24 {
			return nil, nil
		}
		switch current := expression.(type) {
		case *ast.FuncLit:
			return current.Type, current.Body
		case *ast.Ident:
			if current.Obj != nil {
				switch declared := current.Obj.Decl.(type) {
				case *ast.FuncDecl:
					return declared.Type, declared.Body
				case *ast.ValueSpec:
					for index, name := range declared.Names {
						if name.Name == current.Name && index < len(declared.Values) {
							return localFunction(declared.Values[index], depth+1)
						}
					}
				case *ast.AssignStmt:
					for index, left := range declared.Lhs {
						if name, ok := left.(*ast.Ident); ok && name.Name == current.Name && index < len(declared.Rhs) {
							return localFunction(declared.Rhs[index], depth+1)
						}
					}
				}
			}
		case *ast.IndexExpr:
			return localFunction(current.X, depth+1)
		case *ast.IndexListExpr:
			return localFunction(current.X, depth+1)
		case *ast.ParenExpr:
			return localFunction(current.X, depth+1)
		}
		return nil, nil
	}
	var numericValue func(ast.Expr, int, map[*ast.Object]bool) bool
	numericValue = func(expression ast.Expr, depth int, bindings map[*ast.Object]bool) bool {
		if expression == nil || depth > 32 {
			return false
		}
		if value := info.TypeOf(expression); value != nil {
			if floatType(value) {
				return true
			}
			if basic, ok := value.Underlying().(*types.Basic); ok && basic.Kind() != types.Invalid {
				return false
			}
		}
		switch current := expression.(type) {
		case *ast.BasicLit:
			return current.Kind == token.FLOAT || current.Kind == token.IMAG
		case *ast.Ident:
			for index := len(writes[current.Obj]) - 1; index >= 0; index-- {
				write := writes[current.Obj][index]
				if write.end < current.Pos() {
					return numericValue(write.value, depth+1, bindings)
				}
			}
			return bindings[current.Obj]
		case *ast.ParenExpr:
			return numericValue(current.X, depth+1, bindings)
		case *ast.UnaryExpr:
			return numericValue(current.X, depth+1, bindings)
		case *ast.BinaryExpr:
			return numericValue(current.X, depth+1, bindings) || numericValue(current.Y, depth+1, bindings)
		case *ast.CallExpr:
			if info.Types[current.Fun].IsType() && len(current.Args) == 1 {
				return numericValue(current.Args[0], depth+1, bindings)
			}
			name := functionName(current.Fun, 0)
			if strings.HasPrefix(name, "math.") && !strings.HasPrefix(strings.TrimPrefix(name, "math."), "Is") {
				return true
			}
			parameters, body := localFunction(current.Fun, 0)
			if body != nil {
				next := make(map[*ast.Object]bool)
				for name, value := range bindings {
					next[name] = value
				}
				argument := 0
				for _, field := range parameters.Params.List {
					for _, parameter := range field.Names {
						if argument < len(current.Args) {
							next[parameter.Obj] = numericValue(current.Args[argument], depth+1, bindings)
						}
						argument++
					}
				}
				found := false
				ast.Inspect(body, func(node ast.Node) bool {
					if _, nested := node.(*ast.FuncLit); nested {
						return false
					}
					if returned, ok := node.(*ast.ReturnStmt); ok {
						for _, value := range returned.Results {
							found = found || numericValue(value, depth+1, next)
						}
					}
					return !found
				})
				return found
			}
		}
		return false
	}
	floating := func(expression ast.Expr, depth int) bool {
		return numericValue(expression, depth, nil)
	}
	floatElements := func(expression ast.Expr) bool {
		value := info.TypeOf(expression)
		if value == nil {
			return false
		}
		switch collection := value.Underlying().(type) {
		case *types.Slice:
			return floatType(collection.Elem())
		case *types.Array:
			return floatType(collection.Elem())
		}
		return false
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.BinaryExpr:
			switch current.Op {
			case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
				if floating(current.X, 0) || floating(current.Y, 0) {
					record("integer-business-comparison", current)
				}
			}
		case *ast.SwitchStmt:
			if current.Tag != nil && floating(current.Tag, 0) {
				record("integer-business-comparison", current)
			}
		case *ast.CallExpr:
			name := functionName(current.Fun, 0)
			switch name {
			case "builtin.min", "builtin.max", "math.Min", "math.Max", "cmp.Compare", "cmp.Less":
				for _, argument := range current.Args {
					if floating(argument, 0) {
						record("integer-business-comparison", current)
						break
					}
				}
			case "sort.Float64s", "sort.Float64sAreSorted", "sort.SearchFloat64s":
				record("integer-business-comparison", current)
			case "slices.Sort", "slices.IsSorted", "slices.Min", "slices.Max", "slices.BinarySearch":
				if len(current.Args) > 0 && floatElements(current.Args[0]) {
					record("integer-business-comparison", current)
				}
			}
		}
		return true
	})
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
			if input.StrictIntegers && (current.Name == "float32" || current.Name == "float64" || current.Name == "complex64" || current.Name == "complex128") {
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
			if input.StrictIntegers && (current.Kind == token.FLOAT || current.Kind == token.IMAG) {
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
				if owner, ok := selector.X.(*ast.Ident); ok && input.StrictIntegers && owner.Name == "math" && selector.Sel.Name != "MaxInt" && selector.Sel.Name != "MaxInt64" && selector.Sel.Name != "MinInt" && selector.Sel.Name != "MinInt64" {
					record("integer-business-number", current)
				}
				if owner, ok := selector.X.(*ast.Ident); ok && owner.Name == "os" && (selector.Sel.Name == "TempDir" || selector.Sel.Name == "UserHomeDir") {
					record("workspace-output-directory", current)
				}
			}
		}
		return true
	})
	if !input.StrictIntegers {
		checkNumericComparisons(file, positions, packages, record)
	}
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
