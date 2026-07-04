package main

import (
	"bytes"
	"crypto/sha1"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/vpsfreecz/vpsf-status/internal/i18n/catalog"
)

const (
	i18nDir      = "i18n"
	activeSuffix = ".active.toml"
	sourceSuffix = ".toml"
	templateDir  = "templates"
)

type translationEntry struct {
	Other string `toml:"other"`
}

func main() {
	check := flag.Bool("check", false, "check generated translation files")
	flag.Parse()

	if err := run(!*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(update bool) error {
	messages := catalog.Messages()
	if err := validateCatalog(messages); err != nil {
		return err
	}
	if err := validateTemplateKeys(messages); err != nil {
		return err
	}
	if err := validateGoKeys(messages); err != nil {
		return err
	}

	generated, err := generateFiles(messages)
	if err != nil {
		return err
	}

	for path, body := range generated {
		if update {
			if err := os.WriteFile(path, body, 0o644); err != nil {
				return err
			}
			continue
		}

		current, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s is missing or unreadable; run make i18n-update", path)
		}
		if !bytes.Equal(current, body) {
			return fmt.Errorf("%s is stale; run make i18n-update", path)
		}
	}

	return nil
}

func validateCatalog(messages []*i18n.Message) error {
	seen := make(map[string]struct{}, len(messages))
	for _, msg := range messages {
		if msg.ID == "" {
			return fmt.Errorf("message with empty ID")
		}
		if msg.Other == "" {
			return fmt.Errorf("message %s has empty source text", msg.ID)
		}
		if _, ok := seen[msg.ID]; ok {
			return fmt.Errorf("duplicate message ID %s", msg.ID)
		}
		seen[msg.ID] = struct{}{}
	}
	return nil
}

func validateTemplateKeys(messages []*i18n.Message) error {
	known := make(map[string]struct{}, len(messages))
	for _, msg := range messages {
		known[msg.ID] = struct{}{}
	}

	keyRe := regexp.MustCompile(`\.Locale\.T[D]?\s+"([^"]+)"`)
	return filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range keyRe.FindAllSubmatch(data, -1) {
			id := string(match[1])
			if _, ok := known[id]; !ok {
				return fmt.Errorf("%s references unknown message %s", path, id)
			}
		}
		return nil
	})
}

func validateGoKeys(messages []*i18n.Message) error {
	known := make(map[string]struct{}, len(messages))
	for _, msg := range messages {
		known[msg.ID] = struct{}{}
	}

	constants, err := catalogConstants()
	if err != nil {
		return err
	}

	return filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch path {
			case ".git":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		return validateGoFileKeys(path, known, constants)
	})
}

func validateGoFileKeys(path string, known map[string]struct{}, constants map[string]string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}

	var ret error
	ast.Inspect(file, func(node ast.Node) bool {
		if ret != nil {
			return false
		}

		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "T" && selector.Sel.Name != "TD") || len(call.Args) == 0 {
			return true
		}

		id, ok, err := goMessageID(call.Args[0], constants)
		if err != nil {
			ret = fmt.Errorf("%s:%d: %w", path, fset.Position(call.Args[0].Pos()).Line, err)
			return false
		}
		if !ok {
			return true
		}
		if _, exists := known[id]; !exists {
			ret = fmt.Errorf(
				"%s:%d: references unknown message %s",
				path,
				fset.Position(call.Args[0].Pos()).Line,
				id,
			)
			return false
		}

		return true
	})

	return ret
}

func goMessageID(expr ast.Expr, constants map[string]string) (string, bool, error) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false, nil
		}
		id, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", false, err
		}
		return id, true, nil

	case *ast.SelectorExpr:
		ident, ok := v.X.(*ast.Ident)
		if !ok || ident.Name != "catalog" {
			return "", false, nil
		}
		id, ok := constants[v.Sel.Name]
		if !ok {
			return "", false, fmt.Errorf("unknown catalog message constant %s", v.Sel.Name)
		}
		return id, true, nil

	default:
		return "", false, nil
	}
}

func catalogConstants() (map[string]string, error) {
	path := filepath.Join("internal", "i18n", "catalog", "messages.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]string)
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}

		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}

				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}

				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					return nil, fmt.Errorf("%s:%d: %w", path, fset.Position(lit.Pos()).Line, err)
				}
				ret[name.Name] = value
			}
		}
	}

	return ret, nil
}

func generateFiles(messages []*i18n.Message) (map[string][]byte, error) {
	ret := map[string][]byte{
		filepath.Join(i18nDir, "en"+activeSuffix): renderActiveFile("en", nil, messages),
	}

	sources, err := translationSourceFiles()
	if err != nil {
		return nil, err
	}
	for _, path := range sources {
		lang := strings.TrimSuffix(filepath.Base(path), sourceSuffix)
		translations, err := readTranslations(path)
		if err != nil {
			return nil, err
		}
		if err := validateTranslations(lang, translations, messages); err != nil {
			return nil, err
		}
		ret[filepath.Join(i18nDir, lang+activeSuffix)] = renderActiveFile(lang, translations, messages)
	}

	return ret, nil
}

func translationSourceFiles() ([]string, error) {
	entries, err := os.ReadDir(i18nDir)
	if err != nil {
		return nil, err
	}

	var ret []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, sourceSuffix) || strings.HasSuffix(name, activeSuffix) {
			continue
		}
		if name == "en.toml" {
			return nil, fmt.Errorf("English translations are generated from the Go catalog, not %s", filepath.Join(i18nDir, name))
		}
		ret = append(ret, filepath.Join(i18nDir, name))
	}
	sort.Strings(ret)
	return ret, nil
}

func readTranslations(path string) (map[string]translationEntry, error) {
	ret := make(map[string]translationEntry)
	if _, err := toml.DecodeFile(path, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func validateTranslations(lang string, translations map[string]translationEntry, messages []*i18n.Message) error {
	known := make(map[string]*i18n.Message, len(messages))
	for _, msg := range messages {
		known[msg.ID] = msg
		entry, ok := translations[msg.ID]
		if !ok {
			return fmt.Errorf("%s is missing translation for %s", lang, msg.ID)
		}
		if strings.TrimSpace(entry.Other) == "" {
			return fmt.Errorf("%s has empty translation for %s", lang, msg.ID)
		}
		if !reflect.DeepEqual(placeholders(msg.Other), placeholders(entry.Other)) {
			return fmt.Errorf("%s translation for %s has different placeholders", lang, msg.ID)
		}
	}

	for id := range translations {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("%s has unknown translation %s", lang, id)
		}
	}

	return nil
}

func renderActiveFile(lang string, translations map[string]translationEntry, messages []*i18n.Message) []byte {
	var buf bytes.Buffer
	if translations == nil {
		fmt.Fprintln(&buf, "# Code generated by make i18n-update; DO NOT EDIT.")
		fmt.Fprintln(&buf, "# English source strings live in internal/i18n/catalog/messages.go.")
	} else {
		fmt.Fprintln(&buf, "# Code generated by make i18n-update; DO NOT EDIT.")
		fmt.Fprintf(&buf, "# Edit translations in i18n/%s.toml, then run make i18n-update.\n", lang)
		fmt.Fprintln(&buf, "# English source strings live in internal/i18n/catalog/messages.go.")
	}
	fmt.Fprintln(&buf)

	for i, msg := range messages {
		if i > 0 {
			fmt.Fprintln(&buf)
		}
		other := msg.Other
		if translations != nil {
			other = translations[msg.ID].Other
		}
		fmt.Fprintf(&buf, "[%s]\n", strconv.Quote(msg.ID))
		if msg.Description != "" {
			fmt.Fprintf(&buf, "description = %s\n", strconv.Quote(msg.Description))
		}
		fmt.Fprintf(&buf, "hash = %s\n", strconv.Quote(messageHash(msg)))
		fmt.Fprintf(&buf, "other = %s\n", strconv.Quote(other))
	}

	return buf.Bytes()
}

func messageHash(msg *i18n.Message) string {
	sum := sha1.Sum([]byte(msg.ID + "\x00" + msg.Description + "\x00" + msg.Other))
	return fmt.Sprintf("sha1-%x", sum)
}

func placeholders(s string) []string {
	re := regexp.MustCompile(`{{\s*\.([A-Za-z0-9_]+)\s*}}`)
	matches := re.FindAllStringSubmatch(s, -1)
	ret := make([]string, 0, len(matches))
	for _, match := range matches {
		ret = append(ret, match[1])
	}
	sort.Strings(ret)
	return ret
}
