package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	yaml "github.com/pb33f/go-yaml"
)

const (
	// openAPIVersion is stamped on documents synthesised from a bare JSON
	// Schema. 3.1 is the first OpenAPI release whose Schema Object is JSON
	// Schema, so definitions carry over without translation.
	openAPIVersion = "3.1.0"

	// componentsPrefix is where OpenAPI keeps the schemas that JSON Schema
	// keeps under #/definitions or #/$defs.
	componentsPrefix = "#/components/schemas/"
)

// errNoSchemas reports a document that carries nothing to build tables from.
var errNoSchemas = errors.New("no schemas found under components.schemas")

// normalizeSpec returns spec as an OpenAPI document. An OpenAPI document is
// returned unchanged; a bare JSON Schema is wrapped in a minimal OpenAPI 3.1
// document whose components.schemas holds its definitions.
func normalizeSpec(spec []byte) ([]byte, error) {
	// JSON is a subset of YAML, so one decode accepts both input formats.
	var doc map[string]any
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse spec: %w", err)
	}

	if !isJSONSchema(doc) {
		return spec, nil
	}

	return jsonSchemaToOpenAPI(doc)
}

// isJSONSchema reports whether doc is a bare JSON Schema rather than an OpenAPI
// document.
func isJSONSchema(doc map[string]any) bool {
	// An OpenAPI document keeps its schemas under components and its
	// operations under paths. A JSON Schema has neither.
	for _, key := range []string{"components", "paths"} {
		if _, ok := doc[key]; ok {
			return false
		}
	}

	// An "openapi" key is not evidence either way: oneroster.yaml carries
	// "openapi: 3.1.0" above what is otherwise a plain draft-07 schema.
	for _, key := range []string{"$schema", "definitions", "$defs", "properties"} {
		if _, ok := doc[key]; ok {
			return true
		}
	}

	return false
}

// jsonSchemaToOpenAPI wraps a JSON Schema in an OpenAPI 3.1 document, promoting
// its definitions to components.schemas so the rest of the tool sees the shape
// it already understands.
func jsonSchemaToOpenAPI(doc map[string]any) ([]byte, error) {
	// Repoint references before parsing, so every $ref the document carries
	// is rewritten regardless of which keyword nests it.
	rewriteRefs(doc)

	// Re-encode as JSON: the decode above accepted YAML, which jsonschema-go
	// does not read.
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("cannot re-encode spec as JSON: %w", err)
	}

	var root jsonschema.Schema
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("cannot parse JSON Schema: %w", err)
	}

	// definitions is the draft-07 spelling, $defs the 2020-12 one. A document
	// may use either, so read both.
	schemas := make(map[string]*jsonschema.Schema, len(root.Definitions)+len(root.Defs))
	maps.Copy(schemas, root.Definitions)
	maps.Copy(schemas, root.Defs)

	// When a schema defines nothing but itself, the root is the entity. When it
	// carries definitions, those are the entities and the root is the envelope
	// wrapping them for a particular response — a table of no interest.
	envelope := &root
	if len(schemas) == 0 && len(root.Properties) > 0 {
		schemas[rootSchemaName(&root)] = &root
		envelope = nil
	}

	if len(schemas) == 0 {
		return nil, errNoSchemas
	}

	title := root.Title
	if title == "" {
		title = "JSON Schema"
	}

	document, err := json.Marshal(map[string]any{
		"openapi": openAPIVersion,
		"info": map[string]any{
			"title":   title,
			"version": "1.0.0",
		},
		"paths":      responsePath(envelope),
		"components": map[string]any{"schemas": schemas},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode generated OpenAPI document: %w", err)
	}

	return document, nil
}

// rootSchemaName names the table built from a self-contained JSON Schema. $id
// is preferred over title because titles are prose — "IMS Final Release JSON
// Schema Binding (...)" — while $id is already identifier shaped.
func rootSchemaName(root *jsonschema.Schema) string {
	// An $id is conventionally a file name or URL, so keep only its stem. A
	// title is prose and has no stem to take: "Inventory v1.0" would lose the
	// ".0" to the same treatment.
	name := strings.TrimSuffix(path.Base(root.ID), path.Ext(root.ID))

	if name == "" || name == "." || name == "/" {
		name = root.Title
	}

	if name == "" {
		return "Root"
	}

	return name
}

// rewriteRefs points every local JSON Schema reference at the OpenAPI
// components section, so #/definitions/Foo becomes #/components/schemas/Foo.
func rewriteRefs(node any) {
	switch node := node.(type) {
	case map[string]any:
		for key, child := range node {
			if key == "$ref" {
				if ref, ok := child.(string); ok {
					node[key] = rewriteRef(ref)
				}
				continue
			}
			rewriteRefs(child)
		}
	case []any:
		for _, child := range node {
			rewriteRefs(child)
		}
	}
}

// rewriteRef repoints one reference, and leaves anything that is not a local
// definition alone.
func rewriteRef(ref string) string {
	for _, prefix := range []string{"#/definitions/", "#/$defs/"} {
		if target, ok := strings.CutPrefix(ref, prefix); ok {
			return componentsPrefix + target
		}
	}

	return ref
}

// responsePath presents the envelope as what it is: the body of the one
// response this schema describes. Recording it as a path rather than as
// another component keeps the envelope out of the tables while still telling
// the rest of the tool which schemas the payload carries — the distinction
// between what a document owns and what it merely points at.
//
// A schema that is its own entity has no envelope, and needs no path.
func responsePath(envelope *jsonschema.Schema) map[string]any {
	if envelope == nil {
		return map[string]any{}
	}

	// The envelope's own definitions are components now, so send the shape
	// alone.
	body := *envelope
	body.Definitions, body.Defs, body.ID, body.Schema = nil, nil, "", ""

	return map[string]any{
		"/": map[string]any{
			"get": map[string]any{
				"operationId": "root",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "the payload this schema describes",
						"content": map[string]any{
							"application/json": map[string]any{"schema": body},
						},
					},
				},
			},
		},
	}
}
