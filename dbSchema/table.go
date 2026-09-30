// Package dbSchema turns OpenAPI schemas into PostgreSQL table definitions.
package dbSchema

import (
	"fmt"
	"strings"

	"github.com/jinzhu/inflection"
)

// Table is one database table, as read from a single schema.
type Table struct {
	DefaultDatabaseName string
	Name                string
	Key                 Key
	// CompositeKey names the columns of a key spanning more than one column,
	// which has to be declared for the table rather than on a column.
	CompositeKey     []string
	ColumnDefinition []Column
}

// TableName is the table a schema of this name becomes.
func TableName(schemaName string) string {
	return inflection.Plural(toSnakeCase(schemaName))
}

// GenerateEnumSQL renders a CREATE TYPE ... AS ENUM statement. It reports an
// error when values is empty, which Postgres would reject.
func GenerateEnumSQL(enumName string, values []string) (string, error) {
	if len(values) == 0 {
		return "", &Error{
			Code:    EINVALID,
			Message: fmt.Sprintf("enum %s has no values", enumName),
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "CREATE TYPE %s AS ENUM (", enumName)
	for i, value := range values {
		fmt.Fprintf(&sb, "'%s'", value)
		if i < len(values)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(");")
	return sb.String(), nil
}

// CreateSQLStatement renders the table's CREATE TABLE statement, preceded by a
// CREATE TYPE for each of its enum columns. It returns EINVALID when a column
// has no Postgres equivalent.
func (t *Table) CreateSQLStatement(dialect *Dialect) (string, error) {
	const op = "dbSchema.Table.CreateSQLStatement"

	var sb strings.Builder

	// Add enum types
	enumSQL, err := t.createEnumSQLStatement()
	if err != nil {
		return "", &Error{Op: op, Err: err}
	}
	sb.WriteString(enumSQL + "\n")

	sb.WriteString("CREATE TABLE IF NOT EXISTS ")

	// Handle reserved words
	sb.WriteString(dialect.QuoteIdentifier(t.Name))

	sb.WriteString(" (\n")

	for i := range t.ColumnDefinition {
		statement, err := t.ColumnDefinition[i].CreateSQLStatement(dialect)
		if err != nil {
			return "", &Error{Op: op, Err: err}
		}

		sb.WriteString(statement)
		if i < len(t.ColumnDefinition)-1 {
			sb.WriteString(",\n")
		}
	}

	if len(t.CompositeKey) > 0 {
		fmt.Fprintf(&sb, ",\nPRIMARY KEY (%s)", strings.Join(t.CompositeKey, ", "))
	}

	sb.WriteString("\n);")

	return sb.String(), nil
}

func toSnakeCase(s string) string {
	var result strings.Builder

	for i, v := range s {
		if i > 0 && v >= 'A' && v <= 'Z' {
			result.WriteRune('_')
		}

		result.WriteRune(v)
	}

	return strings.ToLower(result.String())
}

// DeleteSQLStatement renders the DROP TABLE statement for the table.
func (t *Table) DeleteSQLStatement() string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;\n", t.Name)
}

func (t *Table) createEnumSQLStatement() (string, error) {
	var sb strings.Builder

	for i := range t.ColumnDefinition {
		column := &t.ColumnDefinition[i]
		if len(column.Enum) > 0 {
			enumName := fmt.Sprintf("%s_%s", inflection.Singular(t.Name), column.Name)
			enumSQL, err := GenerateEnumSQL(enumName, column.Enum)
			if err != nil {
				return "", &Error{Op: "dbSchema.Table.createEnumSQLStatement", Err: err}
			}
			sb.WriteString(enumSQL + "\n")
		}
	}

	return sb.String(), nil
}

// markKeyColumn flags the column the table is identified by. A primary key is
// necessarily NOT NULL, whether or not the schema said so.
func (t *Table) markKeyColumn() {
	if !t.Key.Declared() {
		return
	}

	for i := range t.ColumnDefinition {
		if t.ColumnDefinition[i].Name == t.Key.Column {
			t.ColumnDefinition[i].PrimaryKey = true
			t.ColumnDefinition[i].NotNull = true

			return
		}
	}
}
