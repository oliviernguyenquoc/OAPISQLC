package dbSchema

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jinzhu/inflection"
	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
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
	ForeignKeys         []string
}

func (t Table) GetSQL() (string, error) {
	var sb strings.Builder
	sb.WriteString("CREATE TABLE IF NOT EXISTS ")

	// Handle reserved words
	if slices.Contains(postgresReservedWords, strings.ToUpper(t.Name)) {
		sb.WriteString(fmt.Sprintf("\"%s\"", t.Name))
	} else {
		sb.WriteString(t.Name)
	}

	sb.WriteString(" (\n")

	for i, column := range t.ColumnDefinition {
		statement, err := column.getSQL()
		if err != nil {
			return "", err
		}
		// sb.WriteString("    ")
		sb.WriteString(statement)
		if i < len(t.ColumnDefinition)-1 {
			sb.WriteString(",\n")
		}
	}

	// Add foreign keys
	for _, fk := range t.ForeignKeys {
		sb.WriteString(fmt.Sprintf(",\n    FOREIGN KEY (%s_id) REFERENCES %s(id)", fk, inflection.Plural(fk)))
	}

	sb.WriteString("\n);")

	return sb.String(), nil

}

func getColumnDefinition(properties orderedmap.Map[string, *highbase.SchemaProxy], requiredColums []string) ([]Column, []string) {

	columnDefinition := []Column{}
	foreignKeys := []string{}

	for property := properties.First(); property != nil; property = property.Next() {
		columnName := property.Key()
		columnSchema := property.Value().Schema()

		var dataType string
		if len(columnSchema.Type) > 0 {
			dataType = columnSchema.Type[0]
		} else {
			fmt.Printf("No data type found for property: %s\n", columnName)
			continue
		}

		// Detect if the property is a $ref to another schema
		ref := property.Value().GetReference()
		if ref != "" {
			dataType = "integer"
			foreignKeys = append(foreignKeys, columnName)
			columnName = columnName + "_id"
			dataType = "integer"
		}

		// Handle default value
		defaultValue := ""
		if columnSchema.Default != nil {
			defaultValue = columnSchema.Default.Value
		}

		// Handle possible values for the column (Constraints)
		var enumValues []string
		for _, node := range columnSchema.Enum {
			enumValues = append(enumValues, node.Value)
		}

		columnDefinition = append(columnDefinition, Column{
			Name:         columnName,
			DataType:     dataType,
			PrimaryKey:   columnName == "id",
			NotNull:      (columnSchema.Nullable != nil && !*columnSchema.Nullable) || slices.Contains(requiredColums, columnName),
			DefaultValue: defaultValue,
			Constraints: Constraints{
				Minimum: columnSchema.Minimum,
				Maximum: columnSchema.Maximum,
				Enum:    enumValues,
			},
		})
	}
	return columnDefinition, foreignKeys
}

func NewTableFromSchema(tableName string, schema *highbase.Schema) *Table {
	table := Table{
		Name: inflection.Plural(strings.ToLower(tableName)),
	}

	properties := schema.Properties

	// Check if there is a custom extension x-database-entity
	if schema.Extensions != nil {
		if val, ok := schema.Extensions.Get("x-database-entity"); ok && val.Value == "false" {
			return &table
		}
	}

	requiredColums := schema.Required

	// Check if there is allOf in the schema
	if schema.AllOf != nil {
		for _, item := range schema.AllOf {
			requiredColums = append(requiredColums, item.Schema().Required...)
			colDef, fk := getColumnDefinition(*item.Schema().Properties, requiredColums)
			table.ColumnDefinition = append(table.ColumnDefinition, colDef...)
			table.ForeignKeys = append(table.ForeignKeys, fk...)
		}
	} else {
		table.ColumnDefinition, table.ForeignKeys = getColumnDefinition(*properties, requiredColums)
	}

	return &table
}
