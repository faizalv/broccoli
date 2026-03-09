package lang

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/faizalv/broccoli/internal/ir"
)

var doNotEdit = []byte("// AUTO-GENERATED. DO NOT EDIT.\n\n")

type TsEmmitter struct {
	irInstance ir.IR
}

func NewTsEmmitter(irInstance ir.IR) TsEmmitter {
	return TsEmmitter{
		irInstance: irInstance,
	}
}

func (t TsEmmitter) EmitTypescript(program ir.Program) map[string][]byte {
	out := map[string][]byte{}

	moduleNames := t.irInstance.ModuleNames(program)
	for _, moduleName := range moduleNames {
		module := program.Modules[moduleName]

		var buf []byte
		buf = append(buf, doNotEdit...)

		imports := collectImports(module)
		if len(imports) > 0 {
			importModuleNames := make([]string, 0, len(imports))
			for m := range imports {
				importModuleNames = append(importModuleNames, m)
			}
			sort.Strings(importModuleNames)

			for _, importModule := range importModuleNames {
				names := make([]string, 0, len(imports[importModule]))
				for name := range imports[importModule] {
					names = append(names, name)
				}
				sort.Strings(names)
				buf = append(buf, fmt.Sprintf("import {%s} from './%s'\n", bytes.Join(stringSliceToBytes(names), []byte(",")), importModule)...)
			}
			buf = append(buf, '\n')
		}

		for _, obj := range module.Objects {
			if len(obj.Fields) == 0 {
				continue
			}

			buf = append(buf, fmt.Sprintf("export interface %s {\n", obj.Name)...)
			for _, f := range obj.Fields {
				if f.Optional {
					buf = append(buf, fmt.Sprintf("  %s?: %s\n", f.JSONName, toTypescriptType(f.Type))...)
				} else {
					buf = append(buf, fmt.Sprintf("  %s: %s\n", f.JSONName, toTypescriptType(f.Type))...)
				}
			}
			buf = append(buf, "}\n\n"...)
		}

		out[moduleName] = buf
	}

	return out
}

func toTypescriptType(t ir.Type) string {
	switch t.Kind {
	case ir.TypeString:
		return "string"
	case ir.TypeBool:
		return "boolean"
	case ir.TypeNumber:
		return "number"
	case ir.TypeArray:
		if t.Elem == nil {
			return "any[]"
		}
		return toTypescriptType(*t.Elem) + "[]"
	case ir.TypeMap:
		key := "any"
		value := "any"
		if t.Key != nil {
			key = toTypescriptType(*t.Key)
		}
		if t.Value != nil {
			value = toTypescriptType(*t.Value)
		}
		return fmt.Sprintf("Record<%s, %s>", key, value)
	case ir.TypeObjectRef:
		if t.Name == "" {
			return "any"
		}
		return t.Name
	default:
		return "any"
	}
}

func collectImports(module ir.Module) map[string]map[string]bool {
	imports := map[string]map[string]bool{}
	for _, obj := range module.Objects {
		for _, f := range obj.Fields {
			collectTypeImports(module.Name, f.Type, imports)
		}
	}
	return imports
}

func collectTypeImports(currentModule string, t ir.Type, imports map[string]map[string]bool) {
	switch t.Kind {
	case ir.TypeObjectRef:
		if t.RefModule == "" || t.RefModule == currentModule || t.Name == "" {
			return
		}
		if _, ok := imports[t.RefModule]; !ok {
			imports[t.RefModule] = map[string]bool{}
		}
		imports[t.RefModule][t.Name] = true
	case ir.TypeArray:
		if t.Elem != nil {
			collectTypeImports(currentModule, *t.Elem, imports)
		}
	case ir.TypeMap:
		if t.Key != nil {
			collectTypeImports(currentModule, *t.Key, imports)
		}
		if t.Value != nil {
			collectTypeImports(currentModule, *t.Value, imports)
		}
	}
}

func stringSliceToBytes(values []string) [][]byte {
	out := make([][]byte, 0, len(values))
	for _, v := range values {
		out = append(out, []byte(v))
	}
	return out
}
