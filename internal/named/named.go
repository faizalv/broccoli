package named

import "go/types"

func FromType(t types.Type) *types.Named {
	for {
		switch tt := t.(type) {
		case *types.Pointer:
			t = tt.Elem()
		case *types.Slice:
			t = tt.Elem()
		case *types.Named:
			return tt
		default:
			return nil
		}
	}
}
