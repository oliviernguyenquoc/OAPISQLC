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

	// Add enum types
	for _, column := range t.ColumnDefinition {
		if len(column.Enum) > 0 {
			sb.WriteString(fmt.Sprintf("CREATE TYPE %s AS ENUM (", inflection.Singular(t.Name)+"_"+column.Name))
			for i, enumValue := range column.Enum {
				sb.WriteString(fmt.Sprintf("'%s'", enumValue))
				if i < len(column.Enum)-1 {
					sb.WriteString(", ")
				}
			}
			sb.WriteString(");\n")
		}
	}

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

		sb.WriteString(statement)
		if i < len(t.ColumnDefinition)-1 {
			sb.WriteString(",\n")
		}
	}

	// Add foreign keys
	for _, fk := range t.ForeignKeys {
		sb.WriteString(fmt.Sprintf(",\n    FOREIGN KEY (%s_id) REFERENCES %s(id)", inflection.Singular(fk), inflection.Plural(fk)))
	}

	sb.WriteString("\n);")

	return sb.String(), nil

}

func getColumnDefinition(tableName string, properties orderedmap.Map[string, *highbase.SchemaProxy], requiredColums []string) ([]Column, []string) {

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

		var dataFormat string
		if len(columnSchema.Format) > 0 {
			dataFormat = columnSchema.Format
		}

		// Detect if the property is a $ref to another schema
		ref := property.Value().GetReference()

		if ref != "" || (dataType == "array" && columnSchema.Items != nil && columnSchema.Items.A.Schema().Properties != nil) {
			foreignKeys = append(foreignKeys, columnName)
			columnName = inflection.Singular(columnName) + "_id"
			dataType = "integer"
		}

		// Handle default value
		defaultValue := ""
		if columnSchema.Default != nil {
			defaultValue = columnSchema.Default.Value
		}

		// Handle possible values for the column (Constraints)
		var unique bool
		if columnSchema.UniqueItems != nil && *columnSchema.UniqueItems {
			unique = true
		}

		// Handle enum values
		var enum []string
		var enumType string
		if columnSchema.Enum != nil {
			for _, item := range columnSchema.Enum {
				enum = append(enum, item.Value)
			}
			enumType = tableName + "_" + columnName
		}

		columnDefinition = append(columnDefinition, Column{
			Name:         columnName,
			DataType:     dataType,
			DataFormat:   dataFormat,
			PrimaryKey:   columnName == "id",
			NotNull:      (columnSchema.Nullable != nil && !*columnSchema.Nullable) || slices.Contains(requiredColums, columnName),
			DefaultValue: defaultValue,
			MinMaxConstraint: MinMaxConstraint{
				Minimum: columnSchema.Minimum,
				Maximum: columnSchema.Maximum,
			},
			CharLengthConstaint: CharLengthConstaint{
				MinLength: columnSchema.MinLength,
				MaxLength: columnSchema.MaxLength,
			},
			PatternConstraint: columnSchema.Pattern,
			Unique:            unique,
			customType:        enumType,
			Enum:              enum,
		})
	}
	return columnDefinition, foreignKeys
}

func ToSnakeCase(s string) string {
	var result string

	for i, v := range s {
		if i > 0 && v >= 'A' && v <= 'Z' {
			result += "_"
		}

		result += string(v)
	}

	return strings.ToLower(result)
}

func NewTableFromSchema(tableName string, schema *highbase.Schema) *Table {
	table := Table{
		Name: inflection.Plural(ToSnakeCase(tableName)),
	}

	properties := schema.Properties
	if properties == nil {
		fmt.Printf("No properties found for schema: %s\n", tableName)
		return &table
	}

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
			colDef, fk := getColumnDefinition(tableName, *item.Schema().Properties, requiredColums)
			table.ColumnDefinition = append(table.ColumnDefinition, colDef...)
			table.ForeignKeys = append(table.ForeignKeys, fk...)
		}
	} else {
		table.ColumnDefinition, table.ForeignKeys = getColumnDefinition(tableName, *properties, requiredColums)
	}

	return &table
}
