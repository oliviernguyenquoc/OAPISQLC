package tables

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

func NewTableFromSchema(tableName string, schema orderedmap.Pair[string, *highbase.SchemaProxy]) *Table {
	table := Table{
		Name: inflection.Plural(strings.ToLower(schema.Key())),
	}

	properties := schema.Value().Schema().Properties

	// Check if there is a custom extension x-database-entity
	if schema.Value().Schema().Extensions != nil {
		if val, ok := schema.Value().Schema().Extensions.Get("x-database-entity"); ok && val.Value == "false" {
			return &table
		}
	}

	requiredColums := schema.Value().Schema().Required

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
			table.ForeignKeys = append(table.ForeignKeys, columnName)
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

		table.ColumnDefinition = append(table.ColumnDefinition, Column{
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

	return &table
}
