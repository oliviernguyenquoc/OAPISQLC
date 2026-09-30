package dbSchema

import (
	"strings"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
)

// keyExtension lets a schema name its own primary key when the conventions
// below cannot find it.
const keyExtension = "x-primary-key"

// Key is the column a table is identified by, carried separately from the
// columns because a table's identity is not one of its attributes.
//
// The zero Key means the schema declared no identity. Such a schema is an
// owned child — something another table embeds — rather than an entity, and
// nothing can reference it.
type Key struct {
	Column     string // the property that identifies a row
	DataType   string // the property's schema type, for typing foreign keys
	DataFormat string // the property's schema format, if it declared one
}

// Declared reports whether the schema named an identity at all.
func (k Key) Declared() bool { return k.Column != "" }

// KeyOf resolves the property that identifies a schema's rows.
//
// It prefers an explicit x-primary-key extension, then the conventional names
// — "id", "sourcedId", and "<schemaName>Id" — in that order. Being required is
// not part of the test: specs routinely leave a surrogate "id" optional, and
// demanding it only loses keys that are obviously keys.
//
// It returns the zero Key when the schema declares no identity.
func KeyOf(schemaName string, schema *highbase.Schema) Key {
	if schema == nil || schema.Properties == nil {
		return Key{}
	}

	for _, name := range keyCandidates(schemaName, schema) {
		property, ok := schema.Properties.Get(name)
		if !ok {
			continue
		}

		// A key has to be a single scalar column, so a property that is
		// itself an object or an array cannot serve as one.
		resolved := property.Schema()
		if resolved == nil {
			continue
		}

		dataType := propertyDataType(resolved)
		if dataType == "" || dataType == "object" || dataType == "array" {
			continue
		}

		return Key{Column: name, DataType: dataType, DataFormat: resolved.Format}
	}

	return Key{}
}

// keyCandidates lists the property names that could identify the schema, most
// specific first.
func keyCandidates(schemaName string, schema *highbase.Schema) []string {
	var declared []string

	if schema.Extensions != nil {
		if value, ok := schema.Extensions.Get(keyExtension); ok && value.Value != "" {
			declared = append(declared, value.Value)
		}
	}

	return append(declared, "id", "sourcedId", lowerFirst(schemaName)+"Id")
}

// lowerFirst lowercases the first rune only, turning a schema name into the
// camelCase spelling a property of that name would use.
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}

	return strings.ToLower(s[:1]) + s[1:]
}
