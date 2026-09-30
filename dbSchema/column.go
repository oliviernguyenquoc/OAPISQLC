package dbSchema

import (
	"fmt"
	"slices"
	"strings"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// Constraint is a rule a column's values must satisfy, rendered as a CHECK.
type Constraint interface {
	GetConstraint(columnName string) []string
}

// MinMaxConstraint bounds a numeric column. A nil bound is not enforced.
type MinMaxConstraint struct {
	Minimum *float64
	Maximum *float64
}

// CharLengthConstraint bounds a text column's length. A nil bound is not
// enforced.
type CharLengthConstraint struct {
	MinLength *int64
	MaxLength *int64
}

// PatternConstraint matches a text column against a regular expression. The
// zero value enforces nothing.
type PatternConstraint struct {
	Pattern string
}

// Column is one column of a table, as read from a single schema property.
type Column struct {
	Name                 string
	DataType             string
	DataFormat           string
	NotNull              bool
	DefaultValue         string
	PrimaryKey           bool
	MinMaxConstraint     MinMaxConstraint
	CharLengthConstraint CharLengthConstraint
	PatternConstraint    PatternConstraint
	Unique               bool
	customType           string
	Enum                 []string
	ForeignKey           string
	ForeignKeyColumn     string
}

// GetConstraint returns the CHECK conditions for the bounds that are set.
func (mm MinMaxConstraint) GetConstraint(columnName string) []string {
	conditions := make([]string, 0, 2)

	if mm.Minimum != nil {
		conditions = append(conditions, fmt.Sprintf("%s >= %f", columnName, *mm.Minimum))
	}

	if mm.Maximum != nil {
		conditions = append(conditions, fmt.Sprintf("%s <= %f", columnName, *mm.Maximum))
	}

	return conditions
}

// GetConstraint returns the CHECK conditions for the bounds that are set.
func (cl CharLengthConstraint) GetConstraint(columnName string) []string {
	conditions := make([]string, 0, 2)

	if cl.MinLength != nil {
		conditions = append(
			conditions,
			fmt.Sprintf("char_length(%s) >= %d", columnName, *cl.MinLength),
		)
	}

	if cl.MaxLength != nil {
		conditions = append(
			conditions,
			fmt.Sprintf("char_length(%s) <= %d", columnName, *cl.MaxLength),
		)
	}

	return conditions
}

// GetConstraint returns the CHECK condition for the pattern, or none when the
// pattern is empty.
func (pc PatternConstraint) GetConstraint(columnName string) []string {
	if pc.Pattern != "" {
		return []string{fmt.Sprintf("%s ~ '%s'", columnName, pc.Pattern)}
	}
	return []string{}
}

// GetConstraint returns the column's CHECK clause, or "" when nothing
// constrains it.
func (c *Column) GetConstraint() string {
	conditions := make([]string, 0, 5) // Pre-allocate with expected capacity

	constraints := []Constraint{c.MinMaxConstraint, c.CharLengthConstraint, c.PatternConstraint}
	for _, constraint := range constraints {
		conditions = append(conditions, constraint.GetConstraint(c.Name)...)
	}

	if len(conditions) > 0 {
		return " CHECK (" + strings.Join(conditions, " AND ") + ")"
	}
	return ""
}

// CreateSQLStatement renders the column's definition for a CREATE TABLE body.
// It returns EINVALID when the schema's type has no Postgres equivalent.
func (c *Column) CreateSQLStatement(dialect *Dialect) (string, error) {
	pgDataType, err := dialect.PostgresType(c.DataType, c.DataFormat)
	if err != nil {
		return "", &Error{Op: "dbSchema.Column.CreateSQLStatement", Err: err}
	}

	// The conventions below outrank what the schema asked for, so they are
	// applied to a copy and never leak back to the caller's column.
	column := *c
	pgDataType = column.applyConventions(pgDataType)

	var sb strings.Builder

	// Reserved words need quoting here for the same reason table names do.
	fmt.Fprintf(&sb, "%s %s", dialect.QuoteIdentifier(column.Name), pgDataType)

	column.writeModifiers(&sb, pgDataType)

	return sb.String(), nil
}

// applyConventions overrides the schema where the database knows better: an
// integer "id" is a generated key, the timestamp columns are managed by the
// database, and an enumerated string uses the type declared for it. It returns
// the Postgres type to render, having adjusted the column to match.
func (c *Column) applyConventions(pgDataType string) string {
	// Handle special case for id column
	if c.Name == "id" && c.DataType == "integer" {
		pgDataType = "BIGSERIAL"
		c.NotNull = true
	}

	// Handle special case for created_at and updated_at columns
	if c.Name == "created_at" || c.Name == "updated_at" || c.Name == "deleted_at" {
		pgDataType = "TIMESTAMP"
		c.NotNull = true
		c.DefaultValue = "NOW()"
	}

	// Handle special case for enum
	if c.DataType == "string" && len(c.Enum) > 0 && c.customType != "" {
		pgDataType = c.customType
	}

	return pgDataType
}

// writeModifiers appends the clauses that trail a column's type, in the order
// Postgres expects to read them.
func (c *Column) writeModifiers(sb *strings.Builder, pgDataType string) {
	if c.NotNull {
		sb.WriteString(" NOT NULL")
	}

	if c.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}

	// Handle constraints
	sb.WriteString(c.GetConstraint())

	if c.DefaultValue != "" {
		if pgDataType == "TEXT" {
			fmt.Fprintf(sb, " DEFAULT '%s'", c.DefaultValue)
		} else {
			fmt.Fprintf(sb, " DEFAULT %s", c.DefaultValue)
		}
	}

	if c.Unique {
		sb.WriteString(" UNIQUE")
	}

	if c.ForeignKey != "" {
		fmt.Fprintf(sb, " REFERENCES %s(%s)", c.ForeignKey, c.ForeignKeyColumn)
	}
}

// column turns one property into a column. A property that points at another
// schema becomes a relationship rather than a value; see relate.
//
// It returns EINVALID when the property declares no usable type.
func (b *Builder) column(
	schemaName string,
	property orderedmap.Pair[string, *highbase.SchemaProxy],
	required []string,
) (Column, error) {
	columnName := property.Key()
	columnSchema := property.Value().Schema()

	column := Column{
		Name:       columnName,
		DataType:   propertyDataType(columnSchema),
		DataFormat: columnSchema.Format,
	}

	if target, many, ok := referencedSchemaName(property); ok {
		column, err := b.relate(columnName, target, many)
		if err != nil {
			return Column{}, err
		}

		b.describe(&column, columnSchema, schemaName, required)

		return column, nil
	}

	if column.DataType == "" {
		return Column{}, &Error{
			Code:    EINVALID,
			Message: fmt.Sprintf("no data type for property %s", columnName),
		}
	}

	b.describe(&column, columnSchema, schemaName, required)

	return column, nil
}

// describe fills in what the schema says about a column beyond its type: its
// default, its constraints, and whether a value is required.
func (b *Builder) describe(
	column *Column,
	schema *highbase.Schema,
	schemaName string,
	required []string,
) {
	if schema.Default != nil {
		column.DefaultValue = schema.Default.Value
	}

	column.NotNull = (schema.Nullable != nil && !*schema.Nullable) ||
		slices.Contains(required, column.Name)
	column.Unique = schema.UniqueItems != nil && *schema.UniqueItems
	column.Enum, column.customType = enumValues(schemaName, column.Name, schema)
	column.MinMaxConstraint = MinMaxConstraint{
		Minimum: schema.Minimum,
		Maximum: schema.Maximum,
	}
	column.CharLengthConstraint = CharLengthConstraint{
		MinLength: schema.MinLength,
		MaxLength: schema.MaxLength,
	}
	column.PatternConstraint = PatternConstraint{Pattern: schema.Pattern}
}

// referencedSchemaName returns the component schema a property points at, and
// whether it points at many of them. A to-many relationship cannot be a column
// on this table; see Builder.junction.
func referencedSchemaName(
	property orderedmap.Pair[string, *highbase.SchemaProxy],
) (name string, many, ok bool) {
	if name, ok := schemaNameFromRef(property.Value().GetReference()); ok {
		return name, false, true
	}

	schema := property.Value().Schema()
	if schema == nil || schema.Items == nil || !schema.Items.IsA() {
		return "", false, false
	}

	name, ok = schemaNameFromRef(schema.Items.A.GetReference())

	return name, ok, ok
}

// propertyDataType reads a property's own type, falling back to the branches of
// a union when it declares none: a choice between an enumerated string and a
// patterned string is still a string column.
func propertyDataType(schema *highbase.Schema) string {
	if len(schema.Type) > 0 {
		return schema.Type[0]
	}

	return unionDataType(schema)
}

// enumValues lists a property's permitted values together with the name of the
// Postgres enum type declared for them, or nothing when it has no enum. The
// name has to match what createEnumSQLStatement declares.
func enumValues(tableName, columnName string, schema *highbase.Schema) ([]string, string) {
	if schema.Enum == nil {
		return nil, ""
	}

	enum := make([]string, 0, len(schema.Enum))
	for _, item := range schema.Enum {
		enum = append(enum, item.Value)
	}

	return enum, toSnakeCase(tableName) + "_" + columnName
}

// unionDataType returns the one type shared by every branch of a union, or ""
// when the branches disagree or the schema holds no union. A property written
// as a choice between an enumerated string and a patterned string is still a
// string column.
func unionDataType(schema *highbase.Schema) string {
	var found string

	for _, branches := range [][]*highbase.SchemaProxy{schema.AnyOf, schema.OneOf, schema.AllOf} {
		for _, branch := range branches {
			types := branch.Schema().Type
			if len(types) == 0 || (found != "" && types[0] != found) {
				return ""
			}
			found = types[0]
		}
	}

	return found
}
