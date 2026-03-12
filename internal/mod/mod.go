package mod

import (
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

type Mod struct {
	marker                string
	moduleNameCache       map[string]string
	normalizedBaseModPath string
}

func New(baseModPath string) Mod {
	return Mod{
		marker:                filepath.Base(baseModPath),
		normalizedBaseModPath: normalizeModulePattern(baseModPath),
		moduleNameCache:       make(map[string]string),
	}
}

func (m Mod) GetNamedModName(named *types.Named) (string, bool) {
	path := named.Obj().Pkg().Path()
	return m.GetModuleName(path)
}

func (m Mod) GetNormalizedBaseModPath() string {
	return m.normalizedBaseModPath
}

func (m Mod) GetModuleName(pkgID string) (string, bool) {
	pkgID = filepath.ToSlash(pkgID)
	if v, ok := m.moduleNameCache[pkgID]; ok {
		return v, true
	}
	i := strings.Index(pkgID, m.marker)
	if i == -1 {
		return "", false
	}

	rest := strings.TrimPrefix(pkgID[i+len(m.marker):], "/")

	// module name ends at next slash or as last item
	j := strings.IndexByte(rest, '/')
	if j == -1 {
		return rest, true
	}

	r := rest[:j]
	m.moduleNameCache[pkgID] = r

	return r, true
}

func (m Mod) CollectModuleNames(pkgs []*packages.Package) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		moduleName, ok := m.GetModuleName(pkg.ID)
		if !ok || seen[moduleName] || strings.Contains(moduleName, "...") {
			continue
		}
		seen[moduleName] = true
		out = append(out, moduleName)
	}
	return out
}

func normalizeModulePattern(module string) string {
	module = filepath.ToSlash(strings.TrimSpace(module))
	module = strings.TrimSuffix(module, "/")
	if strings.HasSuffix(module, "...") {
		return module
	}
	if strings.HasPrefix(module, ".") || strings.HasPrefix(module, "/") {
		return module + "/..."
	}
	return "./" + module + "/..."
}
