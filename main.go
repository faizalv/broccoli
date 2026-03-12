package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/faizalv/broccoli/internal/analyzer"
	"github.com/faizalv/broccoli/internal/ir"
	"github.com/faizalv/broccoli/internal/lang"
	"github.com/faizalv/broccoli/internal/mod"
	"github.com/faizalv/broccoli/internal/queue"
	"golang.org/x/tools/go/packages"
)

type targetLang string

const (
	langTypescript targetLang = "typescript"
	langDart       targetLang = "dart"
)

func main() {
	modular := flag.Bool("modular", false, "enable modular code reading mode")
	moduleDir := flag.String("moddir", "", "a directory where modules are located, this directory will act as a base directory to determine which module a type belongs to")
	flag.Parse()

	if !*modular {
		return
	}

	if strings.TrimSpace(*moduleDir) == "" {
		fmt.Fprintln(os.Stderr, "--moddir is required when --modular is set")
		os.Exit(1)
	}

	md := mod.New(*moduleDir)

	modulePattern := md.GetNormalizedBaseModPath()

	cfg := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedFiles |
			packages.NeedDeps |
			packages.NeedImports,
	}

	pkgs, err := packages.Load(cfg, modulePattern)
	if err != nil {
		panic(err)
	}

	moduleNames := md.CollectModuleNames(pkgs)
	roots := analyzer.CollectRootStructs(pkgs)

	q := queue.New()
	for _, root := range roots {
		q.Enqueue(root, false)
		analyzer.BuildQueue(root, q.Enqueue)
	}

	irInstance := ir.NewIR(md)

	program := irInstance.BuildProgram(q.Nodes(), moduleNames)

	ts := lang.NewTsEmmitter(irInstance)

	outputs := map[targetLang]map[string][]byte{
		langTypescript: ts.EmitTypescript(program),
		langDart:       {},
	}

	if err := writeOutputs(langTypescript, outputs[langTypescript]); err != nil {
		panic(err)
	}
}

func writeOutputs(language targetLang, files map[string][]byte) error {
	dir := filepath.Join("contract", string(language))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	for moduleName, content := range files {
		path := filepath.Join(dir, fmt.Sprintf("%s.ts", moduleName))
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
		fmt.Println("Generated " + path)
	}

	return nil
}
