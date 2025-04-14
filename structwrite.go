package structwrite

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("structwrite", New)
}

type Settings struct {
	// Structs is a list of struct types for which immutability outside the constructor is enforced.
	// Each element should be a fully qualified
	Structs []string `json:"structs"`
	//
}

type PluginStructWrite struct {
	structs map[string]bool
}

func New(cfg any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](cfg)
	if err != nil {
		return nil, err
	}

	structMap := make(map[string]bool)
	for _, name := range s.Structs {
		structMap[name] = true
	}

	return &PluginStructWrite{structs: structMap}, nil
}

func (p *PluginStructWrite) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	a := &analysis.Analyzer{
		Name: "structwrite",
		Doc:  "flags writes to specified struct fields outside constructor functions",
		Run:  p.run,
	}
	return []*analysis.Analyzer{a}, nil
}

func (p *PluginStructWrite) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

func (p *PluginStructWrite) run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			assignStmt, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}

			for i, lhs := range assignStmt.Lhs {
				selExpr, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}

				typ := pass.TypesInfo.Types[selExpr.X].Type
				if typ == nil {
					continue
				}

				// Unwrap pointer if needed
				if ptr, ok := typ.(*types.Pointer); ok {
					typ = ptr.Elem()
				}

				named, ok := typ.(*types.Named)
				if !ok {
					continue
				}

				structName := named.Obj().Name()
				fullyQualifiedStructName := named.String()
				fmt.Println(fullyQualifiedStructName)
				if !p.structs[fullyQualifiedStructName] {
					continue
				}

				// Find enclosing function
				funcDecl := findEnclosingFunc(file, n.Pos())
				if funcDecl == nil || !strings.HasPrefix(funcDecl.Name.Name, "New") {
					pass.Reportf(assignStmt.Lhs[i].Pos(), "write to %s field outside constructor: func=%s, named=%s", structName, funcDecl.Name.String(), named.String())
					fmt.Printf("write to %s field outside constructor: func=%s, named=%s", structName, funcDecl.Name.String(), named.String())
					if funcDecl.Doc == nil {
						continue
					}
					for i, comment := range funcDecl.Doc.List {
						fmt.Println(i, comment.Text)
					}
				}
			}

			return true
		})
	}

	return nil, nil
}

func findEnclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			if fn.Body.Pos() <= pos && pos <= fn.Body.End() {
				return fn
			}
		}
	}
	return nil
}
