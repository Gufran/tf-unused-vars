package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
)

var (
	dir string
)

func init() {
	flag.StringVar(&dir, "dir", ".", "Path to the directory with terraform configuration")
	flag.Parse()
}

func main() {
	sources, err := getSources(dir)
	if err != nil {
		log.Fatalf("failed to collect source files. %s", err)
	}

	parser := hclparse.NewParser()

	var files []*hcl.File
	for _, s := range sources {
		f, diag := parser.ParseHCLFile(s)
		if diag.HasErrors() {
			log.Fatalf("failed to parse %q. %s", s, diag.Error())
		}

		files = append(files, f)
	}

	if len(files) == 0 {
		return
	}

	var (
		variables  = map[string]*hclsyntax.Block{}
		traversals []hcl.Traversal
	)

	for _, f := range files {
		for _, block := range f.Body.(*hclsyntax.Body).Blocks {
			if block.Type == "variable" {
				variables[block.Labels[0]] = block
			}

			for _, attr := range block.Body.Attributes {
				traversals = append(traversals, attr.Expr.Variables()...)
			}

			traversals = append(traversals, findExprInBlock(block.Body.Blocks)...)
		}
	}

	usages := map[string]hcl.Traversal{}
	for _, t := range traversals {
		if t.RootName() != "var" {
			continue
		}

		usages[t[1].(hcl.TraverseAttr).Name] = t
	}

	for name, block := range variables {
		_, ok := usages[name]
		if !ok {
			loc := block.Range()
			fmt.Printf("%s: %q is not used\n", loc, name)
		}
	}
}

func findExprInBlock(blocks hclsyntax.Blocks) []hcl.Traversal {
	if len(blocks) == 0 {
		return []hcl.Traversal{}
	}

	var result []hcl.Traversal
	for _, b := range blocks {
		for _, attr := range b.Body.Attributes {
			result = append(result, attr.Expr.Variables()...)
		}

		result = append(result, findExprInBlock(b.Body.Blocks)...)
	}

	return result
}

func getSources(p string) ([]string, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	if info.Mode().IsRegular() {
		return []string{p}, nil
	}

	files, err := ioutil.ReadDir(p)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		if filepath.Ext(f.Name()) != ".tf" {
			continue
		}

		result = append(result, path.Join(p, f.Name()))
	}

	return result, nil
}
