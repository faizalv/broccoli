package mod

import (
	"go/types"
	"path/filepath"
	"strings"
)

const marker = "/modules/"

func GetNamedModName(named *types.Named) (string, bool) {
	path := named.Obj().Pkg().Path()
	return GetModuleName(path)
}

var moduleNameCache = make(map[string]string)

func GetModuleName(pkgID string) (string, bool) {
	pkgID = filepath.ToSlash(pkgID)
	if v, ok := moduleNameCache[pkgID]; ok {
		return v, true
	}
	i := strings.Index(pkgID, marker)
	if i == -1 {
		return "", false
	}

	rest := pkgID[i+len(marker):]

	// module name ends at next slash or as last item
	j := strings.IndexByte(rest, '/')
	if j == -1 {
		return rest, true
	}

	r := rest[:j]
	moduleNameCache[pkgID] = r

	return r, true
}
