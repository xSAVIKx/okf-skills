package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// concept is an extracted OKF concept (endpoint or schema) before it is written.
type concept struct {
	rel   string   // bundle-relative path without .md, e.g. "endpoints/get-orders"
	typ   string   // "API Endpoint" | "Schema"
	title string   // display title
	body  string   // markdown body (without the relationships section)
	links []string // bundle-relative concept paths this concept references
}

// loadSpec loads an OpenAPI 3.x or Swagger 2.0 document (JSON or YAML) into the v3
// model, converting 2.0 specs on the fly.
func loadSpec(path string) (*openapi3.T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	jsonData, err := toJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	var probe struct {
		Swagger string `json:"swagger"`
	}
	_ = json.Unmarshal(jsonData, &probe)
	if strings.HasPrefix(probe.Swagger, "2") {
		var doc2 openapi2.T
		if err := json.Unmarshal(jsonData, &doc2); err != nil {
			return nil, fmt.Errorf("parse swagger 2.0: %w", err)
		}
		return openapi2conv.ToV3(&doc2)
	}
	loader := openapi3.NewLoader()
	return loader.LoadFromData(jsonData)
}

// toJSON returns JSON bytes for a spec that may be JSON or YAML.
func toJSON(data []byte) ([]byte, error) {
	if t := bytes.TrimSpace(data); len(t) > 0 && (t[0] == '{' || t[0] == '[') {
		return data, nil
	}
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns an arbitrary string into a stable, filesystem-safe slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// refName extracts the trailing schema name from a $ref like
// "#/components/schemas/Order" or "#/definitions/Order".
func refName(ref string) string {
	if ref == "" {
		return ""
	}
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// schemaTypeString renders a property's type: the referenced schema name when it is a
// $ref, otherwise the JSON type (with array element type when available).
func schemaTypeString(ref *openapi3.SchemaRef) string {
	if ref == nil {
		return ""
	}
	if n := refName(ref.Ref); n != "" {
		return n
	}
	s := ref.Value
	if s == nil {
		return ""
	}
	if s.Type != nil && len(*s.Type) > 0 {
		t := strings.Join(*s.Type, "|")
		if t == "array" && s.Items != nil {
			return "array<" + schemaTypeString(s.Items) + ">"
		}
		return t
	}
	return "object"
}

// extractConcepts walks the spec and returns one concept per operation and per
// component schema, deterministically ordered.
func extractConcepts(doc *openapi3.T) []concept {
	var out []concept
	schemaExists := map[string]bool{}
	if doc.Components != nil {
		for name := range doc.Components.Schemas {
			schemaExists[name] = true
		}
	}

	// Schemas.
	if doc.Components != nil {
		names := make([]string, 0, len(doc.Components.Schemas))
		for name := range doc.Components.Schemas {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			ref := doc.Components.Schemas[name]
			out = append(out, schemaConcept(name, ref))
		}
	}

	// Endpoints.
	if doc.Paths != nil {
		paths := make([]string, 0)
		for p := range doc.Paths.Map() {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			item := doc.Paths.Map()[p]
			ops := item.Operations()
			methods := make([]string, 0, len(ops))
			for m := range ops {
				methods = append(methods, m)
			}
			sort.Strings(methods)
			for _, m := range methods {
				out = append(out, endpointConcept(m, p, ops[m], schemaExists))
			}
		}
	}
	return out
}

// schemaConcept builds a Schema concept with a # Columns property table.
func schemaConcept(name string, ref *openapi3.SchemaRef) concept {
	var b bytes.Buffer
	b.WriteString("# Columns\n\n")
	b.WriteString("| Name | Type | Required |\n")
	b.WriteString("| --- | --- | --- |\n")
	links := map[string]bool{}
	if ref != nil && ref.Value != nil {
		required := map[string]bool{}
		for _, r := range ref.Value.Required {
			required[r] = true
		}
		props := make([]string, 0, len(ref.Value.Properties))
		for pn := range ref.Value.Properties {
			props = append(props, pn)
		}
		sort.Strings(props)
		for _, pn := range props {
			pref := ref.Value.Properties[pn]
			typ := schemaTypeString(pref)
			req := "no"
			if required[pn] {
				req = "yes"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", sanitize(pn), sanitize(typ), req))
			if n := propRefName(pref); n != "" {
				links["schemas/"+n] = true
			}
		}
	}
	return concept{
		rel: "schemas/" + name, typ: "Schema", title: name,
		body: b.String(), links: sortedKeys(links),
	}
}

// endpointConcept builds an API Endpoint concept and collects schema cross-links.
func endpointConcept(method, path string, op *openapi3.Operation, schemaExists map[string]bool) concept {
	slug := ""
	if op.OperationID != "" {
		slug = slugify(op.OperationID)
	}
	if slug == "" {
		slug = slugify(method + "-" + path)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "# %s %s\n\n", strings.ToUpper(method), path)
	if op.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", op.Summary)
	}
	links := map[string]bool{}
	addLink := func(ref *openapi3.SchemaRef) {
		if ref == nil {
			return
		}
		if n := refName(ref.Ref); n != "" && schemaExists[n] {
			links["schemas/"+n] = true
		}
	}

	// Parameters.
	if len(op.Parameters) > 0 {
		b.WriteString("## Parameters\n\n| Name | In | Type | Required |\n| --- | --- | --- | --- |\n")
		for _, pref := range op.Parameters {
			if pref.Value == nil {
				continue
			}
			p := pref.Value
			req := "no"
			if p.Required {
				req = "yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", sanitize(p.Name), sanitize(p.In), sanitize(schemaTypeString(p.Schema)), req)
			addLink(p.Schema)
		}
		b.WriteString("\n")
	}

	// Request body schema link.
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for _, mt := range op.RequestBody.Value.Content {
			addLink(mt.Schema)
		}
	}
	// Responses.
	if op.Responses != nil {
		codes := make([]string, 0)
		for c := range op.Responses.Map() {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		b.WriteString("## Responses\n\n| Status | Schema |\n| --- | --- |\n")
		for _, c := range codes {
			r := op.Responses.Map()[c]
			schemaName := ""
			if r.Value != nil {
				for _, mt := range r.Value.Content {
					if n := refName(mt.Schema.Ref); n != "" {
						schemaName = n
						addLink(mt.Schema)
						break
					}
				}
			}
			fmt.Fprintf(&b, "| %s | %s |\n", sanitize(c), sanitize(schemaName))
		}
		b.WriteString("\n")
	}

	return concept{
		rel: "endpoints/" + slug, typ: "API Endpoint",
		title: strings.ToUpper(method) + " " + path,
		body:  b.String(), links: sortedKeys(links),
	}
}

// propRefName returns the referenced schema name of a property (direct $ref or array
// element $ref), or "".
func propRefName(ref *openapi3.SchemaRef) string {
	if ref == nil {
		return ""
	}
	if n := refName(ref.Ref); n != "" {
		return n
	}
	if ref.Value != nil && ref.Value.Items != nil {
		return refName(ref.Value.Items.Ref)
	}
	return ""
}

func sortedKeys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// sanitize keeps table cells from breaking the markdown table.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
