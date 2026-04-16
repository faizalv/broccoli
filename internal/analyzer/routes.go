package analyzer

import (
	"go/ast"
	"regexp"
	"strings"

	"github.com/faizalv/broccoli/internal/sdkir"
	"golang.org/x/tools/go/packages"
)

var (
	reRouter  = regexp.MustCompile(`(?i)@Router\s+(\S+)\s+\[(\w+)\]`)
	reSummary = regexp.MustCompile(`(?i)@Summary\s+(.+)`)
	reParam   = regexp.MustCompile(`(?im)^@Param\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)`)
	reSuccess = regexp.MustCompile(`(?i)@Success\s+\d+\s+\{(object|array)\}\s+(\S+)`)
	rePathVar = regexp.MustCompile(`\{(\w+)\}|:(\w+)`)
)

// CollectRoutes finds handler functions annotated with swaggo-style comments.
func CollectRoutes(pkgs []*packages.Package) sdkir.SDK {
	var routes []sdkir.Route

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}
				if fn.Doc == nil {
					return true
				}

				route, ok := parseSwaggerDoc(fn.Doc.Text())
				if !ok {
					return true
				}

				routes = append(routes, route)
				return true
			})
		}
	}

	return sdkir.SDK{Routes: routes}
}

func parseSwaggerDoc(doc string) (sdkir.Route, bool) {
	routerMatch := reRouter.FindStringSubmatch(doc)
	if routerMatch == nil {
		return sdkir.Route{}, false
	}
	path := routerMatch[1]
	method := strings.ToUpper(routerMatch[2])

	summaryMatch := reSummary.FindStringSubmatch(doc)
	if summaryMatch == nil {
		return sdkir.Route{}, false
	}
	funcName := summaryToFuncName(strings.TrimSpace(summaryMatch[1]))

	// collect all @Param entries
	paramMatches := reParam.FindAllStringSubmatch(doc, -1)
	pathParamsByName := map[string]sdkir.Param{}
	var queryParams []sdkir.Param
	var bodyType string

	for _, m := range paramMatches {
		name := m[1]
		location := strings.ToLower(m[2])
		goType := m[3]
		required := strings.ToLower(m[4]) == "true"

		switch location {
		case "body":
			bodyType = goType
		case "path":
			pathParamsByName[name] = sdkir.Param{
				Name:     name,
				Location: sdkir.ParamPath,
				GoType:   goType,
				Required: true, // path params are always required
			}
		case "query":
			queryParams = append(queryParams, sdkir.Param{
				Name:     name,
				Location: sdkir.ParamQuery,
				GoType:   goType,
				Required: required,
			})
		case "header":
			// header params are not included in the generated function signature
		}
	}

	// order path params by their occurrence in the URL
	pathParams := orderPathParams(path, pathParamsByName)

	var respType string
	var respIsArray bool
	if m := reSuccess.FindStringSubmatch(doc); m != nil {
		respIsArray = strings.ToLower(m[1]) == "array"
		respType = m[2]
	}

	return sdkir.Route{
		FuncName:    funcName,
		Method:      sdkir.HTTPMethod(method),
		Path:        path,
		PathParams:  pathParams,
		QueryParams: queryParams,
		BodyType:    bodyType,
		RespType:    respType,
		RespIsArray: respIsArray,
	}, true
}

func orderPathParams(path string, byName map[string]sdkir.Param) []sdkir.Param {
	matches := rePathVar.FindAllStringSubmatch(path, -1)
	out := make([]sdkir.Param, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if p, ok := byName[name]; ok {
			out = append(out, p)
		}
	}
	return out
}

// summaryToFuncName converts a @Summary string to a camelCase function name.
// e.g. "Create User Profile" -> "createUserProfile"
func summaryToFuncName(summary string) string {
	words := strings.Fields(summary)
	if len(words) == 0 {
		return "unknownFunc"
	}
	result := strings.ToLower(words[0])
	for _, w := range words[1:] {
		if w == "" {
			continue
		}
		result += strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return result
}
