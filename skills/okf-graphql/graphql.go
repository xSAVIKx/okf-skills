package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// concept is an extracted OKF concept (a type or an operation) before it is written.
type concept struct {
	rel   string   // bundle-relative path without .md, e.g. "types/Order"
	typ   string   // "GraphQL Type" | "GraphQL Query" | "GraphQL Mutation" | "GraphQL Subscription"
	title string   // display title
	body  string   // markdown body (without the relationships section)
	links []string // bundle-relative concept paths this concept references
}

// loadSchema parses a GraphQL SDL document into an AST schema.
func loadSchema(path string) (*ast.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return gqlparser.LoadSchema(&ast.Source{Name: filepathBase(path), Input: string(data)})
}

func filepathBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

var builtinScalars = map[string]bool{"String": true, "Int": true, "Float": true, "Boolean": true, "ID": true}

// underlying unwraps list nesting to the innermost named type.
func underlying(t *ast.Type) string {
	for t != nil && t.Elem != nil {
		t = t.Elem
	}
	if t == nil {
		return ""
	}
	return t.NamedType
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Trim(slugRE.ReplaceAllString(s, "-"), "-")
}

// extractConcepts walks the schema and returns one concept per user-defined type and
// per root operation field, deterministically ordered.
func extractConcepts(schema *ast.Schema) []concept {
	rootName := map[string]bool{}
	for _, d := range []*ast.Definition{schema.Query, schema.Mutation, schema.Subscription} {
		if d != nil {
			rootName[d.Name] = true
		}
	}

	// Which type names get their own concept (for cross-link existence checks).
	emitted := map[string]bool{}
	for name, def := range schema.Types {
		if def.BuiltIn || strings.HasPrefix(name, "__") || builtinScalars[name] || rootName[name] {
			continue
		}
		switch def.Kind {
		case ast.Object, ast.InputObject, ast.Interface, ast.Enum, ast.Union:
			emitted[name] = true
		}
	}

	var out []concept

	// Types.
	names := make([]string, 0, len(emitted))
	for n := range emitted {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, typeConcept(schema.Types[n], emitted))
	}

	// Operations.
	out = append(out, operationConcepts(schema.Query, "GraphQL Query", "queries", emitted)...)
	out = append(out, operationConcepts(schema.Mutation, "GraphQL Mutation", "mutations", emitted)...)
	out = append(out, operationConcepts(schema.Subscription, "GraphQL Subscription", "subscriptions", emitted)...)
	return out
}

// typeConcept builds a concept for an object/input/interface/enum/union type.
func typeConcept(def *ast.Definition, emitted map[string]bool) concept {
	var b strings.Builder
	links := map[string]bool{}

	switch def.Kind {
	case ast.Enum:
		b.WriteString("# Values\n\n")
		for _, v := range def.EnumValues {
			fmt.Fprintf(&b, "- %s\n", sanitizeLine(v.Name))
		}
	case ast.Union:
		b.WriteString("# Members\n\n")
		for _, m := range def.Types {
			fmt.Fprintf(&b, "- %s\n", sanitizeLine(m))
			if emitted[m] {
				links["types/"+m] = true
			}
		}
	default: // Object, Interface, InputObject
		b.WriteString("# Columns\n\n| Name | Type | Required |\n| --- | --- | --- |\n")
		for _, f := range def.Fields {
			if strings.HasPrefix(f.Name, "__") {
				continue
			}
			req := "no"
			if f.Type.NonNull {
				req = "yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", okf.SanitizeCell(f.Name), okf.SanitizeCell(f.Type.String()), req)
			if u := underlying(f.Type); emitted[u] {
				links["types/"+u] = true
			}
		}
	}
	return concept{
		rel: "types/" + def.Name, typ: "GraphQL Type", title: def.Name,
		body: b.String(), links: sortedKeys(links),
	}
}

// operationConcepts builds one concept per field of a root operation type.
func operationConcepts(root *ast.Definition, typ, subdir string, emitted map[string]bool) []concept {
	if root == nil {
		return nil
	}
	fields := make([]*ast.FieldDefinition, 0, len(root.Fields))
	for _, f := range root.Fields {
		if !strings.HasPrefix(f.Name, "__") {
			fields = append(fields, f)
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })

	var out []concept
	for _, f := range fields {
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", f.Name)
		fmt.Fprintf(&b, "Returns `%s`.\n", f.Type.String())
		links := map[string]bool{}
		if u := underlying(f.Type); emitted[u] {
			links["types/"+u] = true
		}
		if len(f.Arguments) > 0 {
			b.WriteString("\n## Arguments\n\n| Name | Type | Required |\n| --- | --- | --- |\n")
			for _, a := range f.Arguments {
				req := "no"
				if a.Type.NonNull {
					req = "yes"
				}
				fmt.Fprintf(&b, "| %s | %s | %s |\n", okf.SanitizeCell(a.Name), okf.SanitizeCell(a.Type.String()), req)
				if u := underlying(a.Type); emitted[u] {
					links["types/"+u] = true
				}
			}
		}
		out = append(out, concept{
			rel: subdir + "/" + slugify(f.Name), typ: typ, title: f.Name,
			body: b.String(), links: sortedKeys(links),
		})
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func sanitizeLine(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

// refName returns the trailing path element of a bundle-relative concept path.
func refName(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[i+1:]
	}
	return rel
}
