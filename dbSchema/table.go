package dbSchema

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jinzhu/inflection"
	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
)

var postgresReservedWords = []string{
	"ALL", "ANALYSE", "ANALYZE", "AND", "ANY", "ARRAY", "AS", "ASC", "ASYMMETRIC",
	"AUTHORIZATION", "BINARY", "BOTH", "CASE", "CAST", "CHECK", "COLLATE", "COLLATION",
	"COLUMN", "CONCURRENTLY", "CONSTRAINT", "CREATE", "CROSS", "CURRENT_CATALOG",
	"CURRENT_DATE", "CURRENT_ROLE", "CURRENT_SCHEMA", "CURRENT_TIME", "CURRENT_TIMESTAMP",
	"CURRENT_USER", "DEFAULT", "DEFERRABLE", "DESC", "DISTINCT", "DO", "ELSE", "END",
	"EXCEPT", "FALSE", "FETCH", "FOR", "FOREIGN", "FREEZE", "FROM", "FULL", "GRANT", "GROUP",
	"HAVING", "ILIKE", "IN", "INITIALLY", "INNER", "INTERSECT", "INTO", "IS", "ISNULL", "JOIN",
	"LATERAL", "LEADING", "LEFT", "LIKE", "LIMIT", "LOCALTIME", "LOCALTIMESTAMP", "NATURAL",
	"NOT", "NOTNULL", "NULL", "OFFSET", "ON", "ONLY", "OR", "ORDER", "OUTER", "OVERLAPS",
	"PLACING", "PRIMARY", "REFERENCES", "RETURNING", "RIGHT", "SELECT", "SESSION_USER",
	"SIMILAR", "SOME", "SYMMETRIC", "SYSTEM_USER", "TABLE", "TABLESAMPLE", "THEN", "TO",
	"TRAILING", "TRUE", "UNION", "UNIQUE", "USER", "USING", "VARIADIC", "VERBOSE", "WHEN",
	"WHERE", "WINDOW", "WITH",
}

type Table struct {
	DefaultDatabaseName string
	Name                string
	ColumnDefinition    []Column
}

func GenerateEnumSQL(enumName string, values []string) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("enum '%s' must have at least one value", enumName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TYPE %s AS ENUM (", enumName))
	for i, value := range values {
		sb.WriteString(fmt.Sprintf("'%s'", value))
		if i < len(values)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(");")
	return sb.String(), nil
}

func (t Table) createEnumSQLStatement() (string, error) {
	var sb strings.Builder

	for _, column := range t.ColumnDefinition {
		if len(column.Enum) > 0 {
			enumName := fmt.Sprintf("%s_%s", inflection.Singular(t.Name), column.Name)
			enumSQL, err := GenerateEnumSQL(enumName, column.Enum)
			if err != nil {
				return "", err // Handle the error appropriately, possibly accumulating errors or stopping at the first.
			}
			sb.WriteString(enumSQL + "\n")
		}
	}

	return sb.String(), nil
}

func isReservedWord(word string) bool {
	return slices.Contains(postgresReservedWords, strings.ToUpper(word))
}

func (t Table) CreateSQLStatement() (string, error) {
	var sb strings.Builder

	// Add enum types
	enumSQL, err := t.createEnumSQLStatement()
	if err != nil {
		return "", err
	}
	sb.WriteString(enumSQL + "\n")

	sb.WriteString("CREATE TABLE IF NOT EXISTS ")

	// Handle reserved words
	if isReservedWord(t.Name) {
		sb.WriteString(fmt.Sprintf("\"%s\"", t.Name))
	} else {
		sb.WriteString(t.Name)
	}

	sb.WriteString(" (\n")

	for i, column := range t.ColumnDefinition {
		statement, err := column.CreateSQLStatement()
		if err != nil {
			return "", err
		}

		sb.WriteString(statement)
		if i < len(t.ColumnDefinition)-1 {
			sb.WriteString(",\n")
		}
	}

	sb.WriteString("\n);")

	return sb.String(), nil

}

func toSnakeCase(s string) string {
	var result string

	for i, v := range s {
		if i > 0 && v >= 'A' && v <= 'Z' {
			result += "_"
		}

		result += string(v)
	}

	return strings.ToLower(result)
}

func BuildTableFromSchema(tableName string, schema *highbase.Schema) *Table {
	table := Table{
		Name: inflection.Plural(toSnakeCase(tableName)),
	}

	properties := schema.Properties
	if properties == nil && schema.AllOf == nil {
		fmt.Printf("No properties found for schema: %s\n", tableName)
		return &table
	}

	// Check if there is a custom extension x-database-entity
	if schema.Extensions != nil {
		if val, ok := schema.Extensions.Get("x-database-entity"); ok && val.Value == "false" {
			return &table
		}
	}

	requiredColumns := schema.Required

	// Check if there is allOf in the schema
	if schema.AllOf != nil {
		for _, item := range schema.AllOf {
			requiredColumns = append(requiredColumns, item.Schema().Required...)
			colDef, err := BuildColumnsFromSchema(tableName, *item.Schema().Properties, requiredColumns)
			if err != nil {
				fmt.Printf("Error building columns from schema: %v\n", err)
				return &table
			}

			table.ColumnDefinition = append(table.ColumnDefinition, colDef...)
		}
	} else {
		colDef, err := BuildColumnsFromSchema(tableName, *properties, requiredColumns)
		if err != nil {
			fmt.Printf("Error building columns from schema: %v\n", err)
			return &table
		}
		table.ColumnDefinition = colDef
	}

	return &table
}

func (t Table) DeleteSQLStatement() string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;\n", t.Name)
}
