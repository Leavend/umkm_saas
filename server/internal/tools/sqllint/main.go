package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	sqlMarkerPattern  = regexp.MustCompile(`(?i)\b(select|insert|update|delete|with)\b`)
	uuidMarkerPattern = regexp.MustCompile(`^--sql [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type violation struct {
	file    string
	name    string
	line    int
	message string
}

func main() {
	flag.Parse()
	targets := flag.Args()
	if len(targets) == 0 {
		targets = []string{"."}
	}

	var violations []violation

	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sqllint: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			walkErr := filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					if strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				if filepath.Ext(path) != ".go" {
					return nil
				}
				vs, err := lintFile(path)
				if err != nil {
					return err
				}
				violations = append(violations, vs...)
				return nil
			})
			if walkErr != nil {
				fmt.Fprintf(os.Stderr, "sqllint: %v\n", walkErr)
				os.Exit(1)
			}
		} else if filepath.Ext(target) == ".go" {
			vs, err := lintFile(target)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sqllint: %v\n", err)
				os.Exit(1)
			}
			violations = append(violations, vs...)
		}
	}

	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "sqllint: missing SQL audit markers")
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "  %s:%d %s (%s)\n", v.file, v.line, v.message, v.name)
		}
		os.Exit(1)
	}
}

func lintFile(path string) ([]violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var violations []violation
	ast.Inspect(file, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, value := range vs.Values {
			bl, ok := value.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				continue
			}
			raw, err := unquote(bl.Value)
			if err != nil {
				continue
			}
			if !sqlMarkerPattern.MatchString(raw) {
				continue
			}
			marker := firstLine(raw)
			if !uuidMarkerPattern.MatchString(marker) {
				pos := fset.Position(bl.Pos())
				v := violation{
					file:    path,
					line:    pos.Line,
					name:    joinNames(vs.Names),
					message: "missing or invalid --sql <uuid> marker",
				}
				violations = append(violations, v)
			}
		}
		return true
	})
	return violations, nil
}

func firstLine(s string) string {
	s = strings.TrimLeft(s, "\n\r \t")
	if idx := strings.IndexAny(s, "\n\r"); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func unquote(v string) (string, error) {
	if len(v) == 0 {
		return v, nil
	}
	if v[0] == '`' {
		return v[1 : len(v)-1], nil
	}
	return strconv.Unquote(v)
}

func joinNames(idents []*ast.Ident) string {
	parts := make([]string, 0, len(idents))
	for _, ident := range idents {
		if ident == nil {
			continue
		}
		parts = append(parts, ident.Name)
	}
	return strings.Join(parts, ",")
}
