package analyzer

import (
	"go/ast"
	"go/types"

	"github.com/faizalv/broccoli/internal/named"
	"golang.org/x/tools/go/packages"
)

// CollectRootStructs finds top-level struct definitions.
func CollectRootStructs(pkgs []*packages.Package) []*types.Named {
	parentSeen := map[*types.Named]bool{}
	parentQueue := make([]*types.Named, 0, 64)

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				if _, ok := ts.Type.(*ast.StructType); !ok {
					return true
				}

				obj := pkg.TypesInfo.Defs[ts.Name]
				if obj == nil {
					return false
				}

				namedObj, ok := obj.Type().(*types.Named)
				if !ok {
					return false
				}

				if !parentSeen[namedObj] {
					parentSeen[namedObj] = true
					parentQueue = append(parentQueue, namedObj)
				}

				return false
			})
		}
	}

	return parentQueue
}

// BuildQueue expands required named structs by traversing struct fields recursively.
func BuildQueue(n *types.Named, enqueue func(n *types.Named, embedded bool)) {
	st, ok := n.Underlying().(*types.Struct)
	if !ok {
		return
	}

	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)

		if f.Anonymous() {
			n := named.FromType(f.Type())
			if n == nil {
				continue
			}

			if _, ok := n.Underlying().(*types.Struct); ok {
				enqueue(n, true)
				BuildQueue(n, enqueue)
			}
			continue
		}

		if n := named.FromType(f.Type()); n != nil {
			if _, ok := n.Underlying().(*types.Struct); !ok {
				continue
			}

			if n.Obj() != nil && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "time" && n.Obj().Name() == "Time" {
				continue
			}

			enqueue(n, false)
		}
	}
}
