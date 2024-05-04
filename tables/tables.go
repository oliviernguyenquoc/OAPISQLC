package tables

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jinzhu/inflection"
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

type Column struct {
	Name       string
	DataType   string
	NotNull    bool
	IsDefault  bool
	IsString   bool
	PrimaryKey bool
}

type Table struct {
	DefaultDatabaseName string
	Name                string
	ColumnDefinition    []Column
	ForeignKeys         []string
}

var datatypeMap = map[string]string{
	"integer":       "INTEGER",
	"int32":         "INTEGER",
	"int64":         "BIGINT",
	"boolean":       "BOOLEAN",
	"number":        "REAL",
	"string":        "TEXT",
	"byte":          "BYTEA",
	"binary":        "BYTEA",
	"file":          "BYTEA",
	"date":          "DATE",
	"date-time":     "TIMESTAMP",
	"enum":          "TEXT",
	"array":         "JSON",
	"object":        "JSON",
	"\\Model\\User": "TEXT",
}

func (c Column) getSQL() (string, error) {
	var query string
	var pgDataType, dataType string
	var ok bool

	pgDataType, ok = datatypeMap[c.DataType]
	if !ok {
		fmt.Printf("Unknown data type: %s\n", dataType)
		return "", fmt.Errorf("unknown data type: %s", dataType)
	}

	// Handle special case for id column
	if c.Name == "id" && pgDataType == "INTEGER" {
		pgDataType = "bigserial"
		c.NotNull = true
	}

	query = fmt.Sprintf("%s %s", c.Name, pgDataType)

	if c.NotNull {
		query += " NOT NULL"
	}

	if c.PrimaryKey {
		query += " PRIMARY KEY"
	}

	if c.IsDefault {
		if c.IsString {
			query += fmt.Sprintf(" DEFAULT '%s'", c.IsDefault)
		} else {
			query += fmt.Sprintf(" DEFAULT %s", c.IsDefault)
		}
	}

	return query, nil
}

func (t Table) GetSQL() (string, error) {
	var columns []string
	for _, column := range t.ColumnDefinition {
		statement, err := column.getSQL()
		if err != nil {
			return "", err
		}
		columns = append(columns, statement)
	}

	// Handle reserved words
	if slices.Contains(postgresReservedWords, strings.ToUpper(t.Name)) {
		t.Name = fmt.Sprintf("\"%s\"", t.Name)
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n %s", t.Name, strings.Join(columns, ",\n"))

	// Add foreign keys
	for _, fk := range t.ForeignKeys {
		query += fmt.Sprintf(",\n FOREIGN KEY (%s_id) REFERENCES %s(id)", fk, inflection.Plural(fk))
	}

	query += "\n);"

	return query, nil

}
