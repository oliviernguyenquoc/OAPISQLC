package dbSchema

import (
	"errors"
	"fmt"

	"github.com/jinzhu/inflection"
	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// errToMany marks a property that cannot be a column because it points at many
// rows. It never leaves this package: the Builder turns it into a junction
// table instead.
var errToMany = errors.New("relationship is to-many")

// Builder turns a document's schemas into tables.
//
// It exists because a table cannot be built from its own schema alone: a
// property that points at another schema becomes a foreign key, and typing
// that key needs to know what the other schema is keyed by. The Builder holds
// that document-wide knowledge so it does not have to be handed down through
// every call.
type Builder struct {
	classification Classification
}

// NewBuilder returns a Builder for a classified document.
func NewBuilder(classification Classification) *Builder {
	return &Builder{classification: classification}
}

// Tables builds a table for every schema the document owns, in declaration
// order.
//
// A schema the document only points at becomes a foreign key on the tables
// that reference it, not a table of its own. A schema that yields no columns
// is not a table either.
//
// A schema that cannot be expressed is reported in skipped with code EINVALID.
// The schemas either side of it are still built.
func (b *Builder) Tables(
	schemas *orderedmap.Map[string, *highbase.SchemaProxy],
) (tables []Table, skipped []error) {
	if schemas == nil {
		return nil, nil
	}

	for pair := schemas.First(); pair != nil; pair = pair.Next() {
		schemaName := pair.Key()

		// A reference is how another table points at an entity; it is not
		// itself something rows are stored in.
		if b.classification.IsReference(schemaName) {
			continue
		}

		table, err := b.Table(schemaName, pair.Value().Schema())
		if err != nil {
			skipped = append(skipped, fmt.Errorf("schema %s: %w", schemaName, err))

			continue
		}

		if len(table.ColumnDefinition) > 0 {
			tables = append(tables, *table)
		}

		tables = append(tables, b.junctions(schemaName, pair.Value().Schema())...)
	}

	return tables, skipped
}

// Table turns one schema into a table, pluralising its name.
//
// A schema that describes no columns — one with no properties, or one marked
// x-database-entity: false — yields a table with no columns and no error.
//
// It returns EINVALID when a property cannot be expressed as a column.
func (b *Builder) Table(schemaName string, schema *highbase.Schema) (*Table, error) {
	const op = "dbSchema.Builder.Table"

	table := Table{
		Name: TableName(schemaName),
		Key:  KeyOf(schemaName, schema),
	}

	properties := schema.Properties
	if properties == nil && schema.AllOf == nil {
		return &table, nil
	}

	// Check if there is a custom extension x-database-entity
	if schema.Extensions != nil {
		if value, ok := schema.Extensions.Get("x-database-entity"); ok && value.Value == "false" {
			return &table, nil
		}
	}

	required := schema.Required

	if schema.AllOf != nil {
		for _, member := range schema.AllOf {
			required = append(required, member.Schema().Required...)

			columns, err := b.columns(schemaName, *member.Schema().Properties, required)
			if err != nil {
				return nil, &Error{Op: op, Err: err}
			}

			table.ColumnDefinition = append(table.ColumnDefinition, columns...)
		}

		table.markKeyColumn()

		return &table, nil
	}

	columns, err := b.columns(schemaName, *properties, required)
	if err != nil {
		return nil, &Error{Op: op, Err: err}
	}

	table.ColumnDefinition = columns
	table.markKeyColumn()

	return &table, nil
}

// junctions builds the table that carries each to-many relationship a schema
// declares. A column on the declaring table could hold only one row of the
// other side, so the pair of keys becomes a table of its own.
func (b *Builder) junctions(schemaName string, schema *highbase.Schema) []Table {
	if schema == nil || schema.Properties == nil {
		return nil
	}

	key := KeyOf(schemaName, schema)
	if !key.Declared() {
		// With nothing to identify a row of this schema, there is no key to
		// put on the other side of the relationship.
		return nil
	}

	var junctions []Table

	for property := schema.Properties.First(); property != nil; property = property.Next() {
		target, many, ok := referencedSchemaName(property)
		if !ok || !many {
			continue
		}

		relation := b.classification.RelationTo(target)
		if relation.Table == "" {
			continue
		}

		junctions = append(junctions, junctionTable(schemaName, key, property.Key(), relation))
	}

	return junctions
}

// columns turns a schema's properties into columns, in the order the schema
// lists them. Names in required become NOT NULL.
func (b *Builder) columns(
	schemaName string,
	properties orderedmap.Map[string, *highbase.SchemaProxy],
	required []string,
) ([]Column, error) {
	const op = "dbSchema.Builder.columns"

	var columns []Column

	for property := properties.First(); property != nil; property = property.Next() {
		column, err := b.column(schemaName, property, required)

		switch {
		// The relationship is carried by a junction table instead, so this
		// table has no column for it.
		case errors.Is(err, errToMany):
			continue
		case err != nil:
			return nil, &Error{Op: op, Err: err}
		}

		columns = append(columns, column)
	}

	return columns, nil
}

// relate decides what a property pointing at another schema becomes.
//
// One row of a table with identity becomes a foreign key, named for the
// property and the column it points at and typed to match. Many rows of one
// belong in a junction table instead, so relate reports errToMany rather than
// inventing a column that could hold only a single row.
//
// Anything with no identity to point at is held inline as JSON: a value object,
// or — for a to-many relationship — a reference whose entity this document does
// not carry, where there is neither a table to join nor room for the values in
// one column.
func (b *Builder) relate(columnName, target string, many bool) (Column, error) {
	relation := b.classification.RelationTo(target)

	if relation.Embed || !relation.Key.Declared() || (many && relation.Table == "") {
		return Column{Name: columnName, DataType: "object"}, nil
	}

	if many {
		return Column{}, errToMany
	}

	return Column{
		Name:             inflection.Singular(columnName) + "_" + relation.Key.Column,
		DataType:         relation.Key.DataType,
		DataFormat:       relation.Key.DataFormat,
		ForeignKey:       relation.Table,
		ForeignKeyColumn: relation.Key.Column,
	}, nil
}

// junctionTable is the table holding one to-many relationship: a row for each
// pair, keyed by both sides so a pair cannot be recorded twice.
func junctionTable(schemaName string, key Key, propertyName string, relation Relation) Table {
	owner := Column{
		Name:             inflection.Singular(TableName(schemaName)) + "_" + key.Column,
		DataType:         key.DataType,
		DataFormat:       key.DataFormat,
		NotNull:          true,
		ForeignKey:       TableName(schemaName),
		ForeignKeyColumn: key.Column,
	}

	other := Column{
		Name:             inflection.Singular(propertyName) + "_" + relation.Key.Column,
		DataType:         relation.Key.DataType,
		DataFormat:       relation.Key.DataFormat,
		NotNull:          true,
		ForeignKey:       relation.Table,
		ForeignKeyColumn: relation.Key.Column,
	}

	return Table{
		Name:             TableName(schemaName) + "_" + toSnakeCase(propertyName),
		CompositeKey:     []string{owner.Name, other.Name},
		ColumnDefinition: []Column{owner, other},
	}
}
