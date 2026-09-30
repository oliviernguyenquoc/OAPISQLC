package dbSchema_test

import (
	"testing"

	"github.com/oliviernguyenquoc/oapisqlc/dbSchema"
	"github.com/pb33f/libopenapi"
)

// schemaFrom builds the named schema out of a components-only OpenAPI document,
// so a test can state its input as the spec a user would actually write.
func schemaFrom(t *testing.T, name, componentsYAML string) *dbSchema.Key {
	t.Helper()

	spec := "openapi: 3.1.0\ninfo: {title: t, version: \"1\"}\ncomponents:\n  schemas:\n" + componentsYAML

	document, err := libopenapi.NewDocument([]byte(spec))
	if err != nil {
		t.Fatalf("cannot create document: %v", err)
	}

	model, err := document.BuildV3Model()
	if err != nil {
		t.Fatalf("cannot build model: %v", err)
	}

	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("schema %s not in the document", name)
	}

	key := dbSchema.KeyOf(name, proxy.Schema())

	return &key
}

func TestKeyOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schemaName string
		components string
		wantColumn string
		wantType   string
		wantFormat string
	}{
		{
			name:       "plain id",
			schemaName: "User",
			components: "    User:\n      properties:\n        id: {type: integer, format: int64}\n",
			wantColumn: "id",
			wantType:   "integer",
			wantFormat: "int64",
		},
		{
			name:       "id need not be required",
			schemaName: "User",
			components: "    User:\n      properties:\n        id: {type: integer}\n        name: {type: string}\n",
			wantColumn: "id",
			wantType:   "integer",
		},
		{
			name:       "natural key",
			schemaName: "ClassDType",
			components: "    ClassDType:\n      required: [sourcedId]\n      properties:\n        sourcedId: {type: string}\n",
			wantColumn: "sourcedId",
			wantType:   "string",
		},
		{
			name:       "id wins over sourcedId",
			schemaName: "Thing",
			components: "    Thing:\n      properties:\n        sourcedId: {type: string}\n        id: {type: integer}\n",
			wantColumn: "id",
			wantType:   "integer",
		},
		{
			name:       "schema-named key",
			schemaName: "Widget",
			components: "    Widget:\n      properties:\n        widgetId: {type: string, format: uuid}\n",
			wantColumn: "widgetId",
			wantType:   "string",
			wantFormat: "uuid",
		},
		{
			name:       "extension overrides the conventions",
			schemaName: "Thing",
			components: "    Thing:\n      x-primary-key: code\n      properties:\n        id: {type: integer}\n        code: {type: string}\n",
			wantColumn: "code",
			wantType:   "string",
		},
		{
			name:       "no identity at all",
			schemaName: "MetadataDType",
			components: "    MetadataDType:\n      properties:\n        note: {type: string}\n",
		},
		{
			name:       "an object cannot be a key",
			schemaName: "Thing",
			components: "    Thing:\n      properties:\n        id: {type: object, properties: {a: {type: string}}}\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			key := schemaFrom(t, test.schemaName, test.components)

			if key.Column != test.wantColumn {
				t.Errorf("Column = %q, want %q", key.Column, test.wantColumn)
			}

			if key.DataType != test.wantType {
				t.Errorf("DataType = %q, want %q", key.DataType, test.wantType)
			}

			if key.DataFormat != test.wantFormat {
				t.Errorf("DataFormat = %q, want %q", key.DataFormat, test.wantFormat)
			}

			if key.Declared() != (test.wantColumn != "") {
				t.Errorf("Declared() = %v, want %v", key.Declared(), test.wantColumn != "")
			}
		})
	}
}
