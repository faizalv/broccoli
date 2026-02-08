package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"
)

// prototype

type lang string

const (
	langTypescript lang = "typescript"
	langDart       lang = "dart"
)

var doNotEdit = []byte("// AUTO-GENERATED. DO NOT EDIT.\n\n")

func main() {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedFiles |
			packages.NeedDeps |
			packages.NeedImports,
	}

	pkgs, err := packages.Load(cfg, "./internal/modules/...")
	if err != nil {
		panic(err)
	}

	seen := map[*types.Named]bool{}
	parentSeen := map[*types.Named]bool{}

	// queue is for emitting
	// parentQueue act as the starting point
	var queue, parentQueue []*types.Named
	emitted := map[*types.Named]bool{}

	enqueue := func(n *types.Named, embedded bool) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true

		// remove embedded type from generator queue
		if !embedded {
			queue = append(queue, n)
		}
	}

	// build content per language for each module
	moduleContent := map[lang]map[string][]byte{
		langTypescript: {},
		langDart:       {},
	}

	// general will handle stdlib and external lib types
	moduleContent[langTypescript]["general"] = []byte{}

	// initial module mapping
	for _, pkg := range pkgs {
		modname, ok := getModuleName(pkg.ID)
		if !ok {
			continue
		}

		moduleContent[langTypescript][modname] = []byte{}
	}

	// package
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			// from fileset, get current file start position
			filename := pkg.Fset.Position(file.Pos()).Filename
			if !isEntityFile(filename) {
				continue
			}

			// inspect AST node for current file
			// not including stdlib or third party yet
			ast.Inspect(file, func(n ast.Node) bool {
				// make sure it's an AST type node
				// if not, keep walk
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				// skip non-struct type on parent nodes, keep walking
				if _, ok := ts.Type.(*ast.StructType); !ok {
					return true
				}

				// match definition from package/types
				// so we can use types instead of AST for the next step
				// if none found, we can stop walking
				obj := pkg.TypesInfo.Defs[ts.Name]
				if obj == nil {
					return false
				}

				// StructType zone with definitions

				// make sure it's named
				named, ok := obj.Type().(*types.Named)
				if !ok {
					return false
				}

				if !seen[named] {
					parentQueue = append(parentQueue, named)
					parentSeen[named] = true
				}

				// we have our struct, exiting this subtree
				return false
			})
		}
	}

	generalType := map[string][]*types.Named{}

	for i := 0; i < len(parentQueue); i++ {
		// buildQueue will expand named type required for emitting
		// this include any struct imported by parent named type
		enqueue(parentQueue[i], false)
		buildQueue(parentQueue[i], enqueue)
	}

	for i := 0; i < len(queue); i++ {
		mod, ok := getNamedModName(queue[i])
		if !ok {
			mod = "general"
			generalType["general"] = append(generalType["general"], queue[i])
		}
		content := moduleContent[langTypescript][mod]
		if len(content) == 0 {
			content = append(content, doNotEdit...)
		}
		emitTypescriptObject(&content, queue[i], emitted)
		moduleContent[langTypescript][mod] = content
	}

	// todo handle other language
	_ = os.MkdirAll("contract/typescript", 0755)
	for module, content := range moduleContent[langTypescript] {
		// handle imports
		var needImports [][]byte
		for _, named := range generalType["general"] {
			if module == "general" {
				break
			}

			// look for general type being referenced
			objName := []byte(named.Obj().Name())
			i := bytes.Index(content, objName)
			if i == -1 {
				continue
			}

			needImports = append(needImports, objName)
		}

		if len(needImports) > 0 {
			dne := bytes.Index(content, doNotEdit)
			if dne != -1 {
				impSpecPos := dne + len(doNotEdit)
				impSpec := []byte("import {")
				impSpec = append(impSpec, append(bytes.Join(needImports, []byte{','}), []byte("} from './general' \n\n")...)...)

				content = append(content[:impSpecPos], append(impSpec, content[impSpecPos:]...)...)
			}
		}

		path := fmt.Sprintf("contract/typescript/%s.ts", module)
		if err := os.WriteFile(path, content, 0644); err != nil {
			panic(err)
		}

		fmt.Println("Generated " + path)
	}
}

func getNamedModName(named *types.Named) (string, bool) {
	path := named.Obj().Pkg().Path()
	return getModuleName(path)
}

// buildQueue will fully build queue from expanded Underlying
func buildQueue(n *types.Named, enqueue func(n *types.Named, embedded bool)) {
	st, ok := n.Underlying().(*types.Struct)
	if !ok {
		return
	}

	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)

		if f.Anonymous() {
			n := namedFromType(f.Type())
			if n == nil {
				continue
			}

			if _, ok := n.Underlying().(*types.Struct); ok {
				enqueue(n, true)
				buildQueue(n, enqueue)
			}
			continue
		}

		if n := namedFromType(f.Type()); n != nil {
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

func emitTypescriptObject(out *[]byte, n *types.Named, emitted map[*types.Named]bool) {
	if emitted[n] {
		return
	}
	emitted[n] = true

	st, ok := n.Underlying().(*types.Struct)
	if !ok {
		return
	}

	var buf []byte
	buf = append(buf, fmt.Sprintf("export interface %s {\n", n.Obj().Name())...)

	count := 0
	visited := map[*types.Named]bool{}

	ownFields := map[string]bool{}

	// process own fields
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if f.Anonymous() {
			continue
		}

		tag := st.Tag(i)
		jsonTag := reflect.StructTag(tag).Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		name := strings.Split(jsonTag, ",")[0]

		ownFields[name] = true
	}

	appendFields(&buf, st, &count, visited, ownFields)

	if count > 0 {
		buf = append(buf, "}\n\n"...)
		*out = append(*out, buf...)
	}
}

func appendFields(
	buf *[]byte,
	st *types.Struct,
	count *int,
	visited map[*types.Named]bool,
	ownFields map[string]bool,
) {
	appendFieldsInner(buf, st, count, visited, ownFields, false)
}

// todo support dart here
func appendFieldsInner(
	buf *[]byte,
	st *types.Struct,
	count *int,
	visited map[*types.Named]bool,
	ownFields map[string]bool,
	fromEmbedded bool,
) {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)

		if f.Anonymous() {
			n := namedFromType(f.Type())
			if n == nil || visited[n] {
				continue
			}
			visited[n] = true

			if nested, ok := n.Underlying().(*types.Struct); ok {
				// expand unnamed embed's fields, e.g. response.Response
				appendFieldsInner(buf, nested, count, visited, ownFields, true)
			}
			continue
		}

		// get tag
		tag := st.Tag(i)
		jsonTag := reflect.StructTag(tag).Get("json")

		// if empty continue
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		parts := strings.Split(jsonTag, ",")
		name := parts[0]
		optional := len(parts) > 1 && parts[1] == "omitempty"

		// remove duplicate from embedded
		if fromEmbedded && ownFields[name] {
			continue
		}

		tsType := goTypeToTypescript(f.Type())

		if optional {
			*buf = append(*buf, fmt.Sprintf("  %s?: %s\n", name, tsType)...)
		} else {
			*buf = append(*buf, fmt.Sprintf("  %s: %s\n", name, tsType)...)
		}

		*count++
	}
}

func namedFromType(t types.Type) *types.Named {
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

func goTypeToTypescript(t types.Type) string {
	switch tt := t.(type) {

	case *types.Named:
		// time type only, treat as string
		if tt.Obj().Name() == "Time" {
			return "string"
		}

		// field with named type, check the underlying type first
		switch tt.Underlying().(type) {
		case *types.Struct:
			// if it's struct, emit the name
			return tt.Obj().Name()
		default:
			// non struct type
			// emit the underlying, e.g. map[string]interface{}
			return goTypeToTypescript(tt.Underlying())
		}

	case *types.Basic:
		switch tt.Kind() {
		case types.String:
			return "string"
		case types.Bool:
			return "boolean"
		default:
			return "number"
		}

	case *types.Pointer:
		// todo mark pointer as optional too
		return goTypeToTypescript(tt.Elem())

	case *types.Slice:
		// example number[]
		return goTypeToTypescript(tt.Elem()) + "[]"

	case *types.Map:
		// map as Record
		return fmt.Sprintf(
			"Record<%s, %s>",
			goTypeToTypescript(tt.Key()),
			goTypeToTypescript(tt.Elem()),
		)

	case *types.Struct:
		// unnamed struct, put any
		return "any"

	default:
		return "any"
	}
}

const marker = "/modules/"

var moduleNameCache = make(map[string]string)

func getModuleName(pkgID string) (string, bool) {
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

func isEntityFile(path string) bool {
	path = filepath.ToSlash(path)
	i := strings.Index(path, "/entity/")
	if i == -1 {
		return false
	}
	return true
}
