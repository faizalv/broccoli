package ir

import (
	"go/types"
	"reflect"
	"sort"
	"strings"

	"github.com/faizalv/broccoli/internal/mod"
	"github.com/faizalv/broccoli/internal/named"
)

type IR struct {
	md mod.Mod
}

func NewIR(m mod.Mod) IR {
	return IR{md: m}
}

const GeneralModule = "general"

type TypeKind string

const (
	TypeAny       TypeKind = "any"
	TypeString    TypeKind = "string"
	TypeBool      TypeKind = "bool"
	TypeNumber    TypeKind = "number"
	TypeArray     TypeKind = "array"
	TypeMap       TypeKind = "map"
	TypeObjectRef TypeKind = "object_ref"
)

type Type struct {
	Kind TypeKind
	Name string

	RefModule string

	Elem  *Type
	Key   *Type
	Value *Type
}

type Field struct {
	JSONName string
	Optional bool
	Type     Type
}

type Object struct {
	Name   string
	Module string
	Fields []Field
}

type Module struct {
	Name    string
	Objects []Object
}

type Program struct {
	Modules map[string]Module
}

func newProgram(moduleNames []string) Program {
	modules := make(map[string]Module, len(moduleNames)+1)
	for _, name := range moduleNames {
		modules[name] = Module{Name: name, Objects: []Object{}}
	}
	if _, ok := modules[GeneralModule]; !ok {
		modules[GeneralModule] = Module{Name: GeneralModule, Objects: []Object{}}
	}
	return Program{Modules: modules}
}

func (ir IR) BuildProgram(nodes []*types.Named, moduleNames []string) Program {
	program := newProgram(moduleNames)

	for _, n := range nodes {
		st, ok := n.Underlying().(*types.Struct)
		if !ok {
			continue
		}

		moduleName, ok := ir.md.GetNamedModName(n)
		if !ok {
			moduleName = GeneralModule
		}

		obj := Object{
			Name:   n.Obj().Name(),
			Module: moduleName,
			Fields: ir.buildFields(st),
		}

		module := program.Modules[moduleName]
		module.Objects = append(module.Objects, obj)
		program.Modules[moduleName] = module
	}

	return program
}

func (ir IR) ModuleNames(program Program) []string {
	names := make([]string, 0, len(program.Modules))
	for name := range program.Modules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (ir IR) buildFields(st *types.Struct) []Field {
	visited := map[*types.Named]bool{}
	ownFields := map[string]bool{}
	emittedNames := map[string]bool{}
	fields := make([]Field, 0, st.NumFields())

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
		if name == "" {
			continue
		}
		ownFields[name] = true
	}

	ir.appendFields(&fields, st, visited, ownFields, emittedNames, false)

	return fields
}

func (ir IR) appendFields(
	out *[]Field,
	st *types.Struct,
	visited map[*types.Named]bool,
	ownFields map[string]bool,
	emittedNames map[string]bool,
	fromEmbedded bool,
) {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)

		if f.Anonymous() {
			n := named.FromType(f.Type())
			if n == nil || visited[n] {
				continue
			}
			visited[n] = true

			if nested, ok := n.Underlying().(*types.Struct); ok {
				ir.appendFields(out, nested, visited, ownFields, emittedNames, true)
			}
			continue
		}

		tag := st.Tag(i)
		jsonTag := reflect.StructTag(tag).Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		parts := strings.Split(jsonTag, ",")
		name := parts[0]
		if name == "" {
			continue
		}
		optional := len(parts) > 1 && parts[1] == "omitempty"

		if fromEmbedded && ownFields[name] {
			continue
		}
		if emittedNames[name] {
			continue
		}

		*out = append(*out, Field{
			JSONName: name,
			Optional: optional,
			Type:     ir.goTypeToIR(f.Type()),
		})
		emittedNames[name] = true
	}
}

func (ir IR) goTypeToIR(t types.Type) Type {
	switch tt := t.(type) {
	case *types.Named:
		if tt.Obj() != nil && tt.Obj().Pkg() != nil && tt.Obj().Pkg().Path() == "time" && tt.Obj().Name() == "Time" {
			return Type{Kind: TypeString}
		}

		switch tt.Underlying().(type) {
		case *types.Struct:
			refModule, ok := ir.md.GetNamedModName(tt)
			if !ok {
				refModule = GeneralModule
			}
			return Type{Kind: TypeObjectRef, Name: tt.Obj().Name(), RefModule: refModule}
		default:
			return ir.goTypeToIR(tt.Underlying())
		}

	case *types.Basic:
		switch tt.Kind() {
		case types.String:
			return Type{Kind: TypeString}
		case types.Bool:
			return Type{Kind: TypeBool}
		default:
			return Type{Kind: TypeNumber}
		}

	case *types.Pointer:
		return ir.goTypeToIR(tt.Elem())

	case *types.Slice:
		elem := ir.goTypeToIR(tt.Elem())
		return Type{Kind: TypeArray, Elem: &elem}

	case *types.Map:
		key := ir.goTypeToIR(tt.Key())
		value := ir.goTypeToIR(tt.Elem())
		return Type{Kind: TypeMap, Key: &key, Value: &value}

	case *types.Struct:
		return Type{Kind: TypeAny}

	default:
		return Type{Kind: TypeAny}
	}
}
