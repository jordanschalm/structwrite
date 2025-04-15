package structwrite

import (
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
	ConstructorRegex string `json:"constructorRegex"`
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

			composite, ok := n.(*ast.CompositeLit)
			if ok {
				typ := pass.TypesInfo.Types[composite].Type
				if typ == nil {
					return true
				}

				typ = deref(typ)

				named, ok := typ.(*types.Named)
				if !ok {
					return true
				}

				fullyQualified := named.String()
				if p.structs[fullyQualified] {
					funcDecl := findEnclosingFunc(file, n.Pos())
					if funcDecl == nil || !strings.HasPrefix(funcDecl.Name.Name, "New") {
						pass.Reportf(composite.Pos(),
							"construction of %s outside constructor",
							named.Obj().Name())
					}
				}
			}

			assignStmt, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}

			for i, lhs := range assignStmt.Lhs {

				selExpr, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}

				found, structName, fullyQualified := p.containsTrackedStruct(selExpr, pass)
				if !found {
					continue
				}

				// Find enclosing function
				funcDecl := findEnclosingFunc(file, n.Pos())
				if funcDecl == nil || !strings.HasPrefix(funcDecl.Name.Name, "New") {
					pass.Reportf(assignStmt.Lhs[i].Pos(), "write to %s field outside constructor: func=%s, named=%s", structName, funcDecl.Name.String(), fullyQualified)
					//fmt.Printf("write to %s field outside constructor: func=%s, named=%s", structName, funcDecl.Name.String(), fullyQualified)
					if funcDecl.Doc == nil {
						continue
					}
					//for i, comment := range funcDecl.Doc.List {
					//fmt.Println(i, comment.Text)
					//}
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

// containsTrackedStruct checks if any type in the selector chain matches a tracked struct.
// It returns the matched struct's name and fully qualified name if found.
func (p *PluginStructWrite) containsTrackedStruct(selExpr *ast.SelectorExpr, pass *analysis.Pass) (bool, string, string) {
	// Check for field selection via embedding
	if sel := pass.TypesInfo.Selections[selExpr]; sel != nil && sel.Kind() == types.FieldVal {
		// Walk the type chain using the index path
		typ := sel.Recv()
		for _, idx := range sel.Index() {
			structType, ok := deref(typ).Underlying().(*types.Struct)
			if !ok || idx >= structType.NumFields() {
				break
			}
			field := structType.Field(idx)
			typ = field.Type()

			// Check if it's one of the tracked structs
			if named, ok := deref(typ).(*types.Named); ok {
				fullyQualified := named.String()
				if p.structs[fullyQualified] {
					return true, named.Obj().Name(), fullyQualified
				}
			}
		}
	}

	// Fallback: direct access (no embedding)
	if tv, ok := pass.TypesInfo.Types[selExpr.X]; ok {
		typ := deref(tv.Type)
		if named, ok := typ.(*types.Named); ok {
			fullyQualified := named.String()
			if p.structs[fullyQualified] {
				return true, named.Obj().Name(), fullyQualified
			}
		}
	}

	return false, "", ""
}

func deref(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

func funcNameOrEmpty(fn *ast.FuncDecl) string {
	if fn != nil {
		return fn.Name.Name
	}
	return "(unknown)"
}
