package transpiler

import (
	"go/types"

	"github.com/faizalv/broccoli/internal/named"
)

// BuildQueue will fully build queue from expanded Underlying
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
				return
			}

			switch n.Obj().Name() {
			case "Time":
				continue
			default:
				enqueue(n, false)
			}
		}

	}
}
