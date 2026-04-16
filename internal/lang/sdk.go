package lang

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/faizalv/broccoli/internal/ir"
	"github.com/faizalv/broccoli/internal/sdkir"
)

var reSDKPathVar = regexp.MustCompile(`\{(\w+)\}|:(\w+)`)

// EmitSdk generates a single sdk.ts file containing typed fetch wrappers
// for all routes collected from swagger comments.
func (t TsEmmitter) EmitSdk(sdk sdkir.SDK, program ir.Program) []byte {
	if len(sdk.Routes) == 0 {
		return nil
	}

	typeIndex := buildTypeIndex(program)

	// collect imports across all routes
	imports := map[string]map[string]bool{}
	for _, route := range sdk.Routes {
		collectSdkImport(route.BodyType, typeIndex, imports)
		collectSdkImport(route.RespType, typeIndex, imports)
	}

	var buf bytes.Buffer
	buf.Write(doNotEdit)

	// re-export every type from every module so sdk.ts is the single import
	// point for the frontend — response shapes, request bodies, header structs, all of it
	for _, modName := range t.irInstance.ModuleNames(program) {
		module := program.Modules[modName]
		var names []string
		for _, obj := range module.Objects {
			if len(obj.Fields) > 0 {
				names = append(names, obj.Name)
			}
		}
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)
		fmt.Fprintf(&buf, "export type {%s} from './%s'\n", strings.Join(names, ","), modName)
	}
	buf.WriteByte('\n')

	// import types needed to compile the fetch wrappers
	if len(imports) > 0 {
		modules := make([]string, 0, len(imports))
		for m := range imports {
			modules = append(modules, m)
		}
		sort.Strings(modules)

		for _, mod := range modules {
			names := make([]string, 0, len(imports[mod]))
			for name := range imports[mod] {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Fprintf(&buf, "import type {%s} from './%s'\n", strings.Join(names, ","), mod)
		}
		buf.WriteByte('\n')
	}

	for _, route := range sdk.Routes {
		emitRoute(&buf, route)
	}

	return buf.Bytes()
}

func emitRoute(buf *bytes.Buffer, route sdkir.Route) {
	// build function parameters
	var sigParts []string
	for _, p := range route.PathParams {
		sigParts = append(sigParts, fmt.Sprintf("%s: %s", p.Name, goTypeToTs(p.GoType)))
	}
	if route.BodyType != "" {
		sigParts = append(sigParts, fmt.Sprintf("body: %s", route.BodyType))
	}
	for _, p := range route.QueryParams {
		if p.Required {
			sigParts = append(sigParts, fmt.Sprintf("%s: %s", p.Name, goTypeToTs(p.GoType)))
		} else {
			sigParts = append(sigParts, fmt.Sprintf("%s?: %s", p.Name, goTypeToTs(p.GoType)))
		}
	}
	sigParts = append(sigParts, "options?: RequestInit")

	respType := "void"
	if route.RespType != "" {
		respType = route.RespType
		if route.RespIsArray {
			respType = route.RespType + "[]"
		}
	}

	fmt.Fprintf(buf, "export async function %s(%s): Promise<%s> {\n",
		route.FuncName, strings.Join(sigParts, ", "), respType)

	// URL
	urlExpr := pathToTsTemplate(route.Path)
	hasQueryParams := len(route.QueryParams) > 0

	if hasQueryParams {
		buf.WriteString("  const _q = new URLSearchParams()\n")
		for _, p := range route.QueryParams {
			if p.Required {
				fmt.Fprintf(buf, "  _q.append('%s', String(%s))\n", p.Name, p.Name)
			} else {
				fmt.Fprintf(buf, "  if (%s !== undefined) _q.append('%s', String(%s))\n", p.Name, p.Name, p.Name)
			}
		}
		buf.WriteString("  const _qs = _q.toString()\n")
		urlExpr = fmt.Sprintf("_qs ? %s + '?' + _qs : %s", urlExpr, urlExpr)
	}

	// fetch call
	buf.WriteString("  const res = await fetch(")
	buf.WriteString(urlExpr)
	buf.WriteString(", {\n")
	fmt.Fprintf(buf, "    method: '%s',\n", string(route.Method))

	if route.BodyType != "" {
		buf.WriteString("    headers: {'Content-Type': 'application/json'},\n")
		buf.WriteString("    body: JSON.stringify(body),\n")
	}

	buf.WriteString("    ...options,\n")
	buf.WriteString("  })\n")

	if route.RespType != "" {
		buf.WriteString("  return res.json()\n")
	} else {
		buf.WriteString("  await res.body?.cancel()\n")
	}

	buf.WriteString("}\n\n")
}

// pathToTsTemplate converts a swagger path to a TypeScript template literal or plain string.
// /profiles/{id}/orders -> `/profiles/${id}/orders`
// /profiles/:id/orders -> `/profiles/${id}/orders`
func pathToTsTemplate(path string) string {
	result := reSDKPathVar.ReplaceAllStringFunc(path, func(s string) string {
		name := strings.Trim(s, "{}")
		if strings.HasPrefix(name, ":") {
			name = name[1:]
		}
		return "${" + name + "}"
	})

	if strings.Contains(result, "${") {
		return "`" + result + "`"
	}
	return "'" + result + "'"
}

// goTypeToTs maps a Go primitive type string (from swagger @Param) to a TypeScript type.
func goTypeToTs(goType string) string {
	switch strings.ToLower(goType) {
	case "string":
		return "string"
	case "bool", "boolean":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "number":
		return "number"
	default:
		return "string"
	}
}

// buildTypeIndex creates a map from type name -> module name from the existing Program.
func buildTypeIndex(program ir.Program) map[string]string {
	index := map[string]string{}
	for modName, module := range program.Modules {
		for _, obj := range module.Objects {
			index[obj.Name] = modName
		}
	}
	return index
}

func collectSdkImport(typeName string, typeIndex map[string]string, imports map[string]map[string]bool) {
	if typeName == "" {
		return
	}
	mod, ok := typeIndex[typeName]
	if !ok {
		return
	}
	if _, exists := imports[mod]; !exists {
		imports[mod] = map[string]bool{}
	}
	imports[mod][typeName] = true
}
