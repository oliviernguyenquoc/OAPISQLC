package dbSchema

import (
	"fmt"
	"strings"
)

// Dialect holds what this package knows about one SQL dialect: which Postgres
// type each schema type and format maps to, and which identifiers Postgres
// reserves.
//
// Build one with NewDialect and share it for the life of a run. Its tables are
// populated at construction and never written afterwards, so a Dialect is safe
// to read from concurrently.
type Dialect struct {
	dataTypes map[string]string
	reserved  map[string]struct{}
}

// NewDialect returns the Postgres dialect.
func NewDialect() *Dialect {
	reserved := []string{
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

	dialect := Dialect{
		// Keyed by schema type and format joined by a colon; the empty format
		// is the entry used when a property declares none.
		dataTypes: map[string]string{
			"integer:":         "INTEGER",
			"integer:int32":    "INTEGER",
			"integer:int64":    "BIGINT",
			"boolean:":         "BOOLEAN",
			"number:":          "NUMERIC",
			"number:float":     "REAL",
			"number:double":    "DOUBLE PRECISION",
			"file:":            "BYTEA",
			"string:":          "TEXT",
			"string:byte":      "BYTEA",
			"string:binary":    "BYTEA",
			"string:date":      "DATE",
			"string:date-time": "TIMESTAMP",
			"string:uuid":      "UUID",
			"string:enum":      "TEXT",
			"array:":           "JSON",
			"object:":          "JSON",
			"\\Model\\User:":   "TEXT",
		},
		reserved: make(map[string]struct{}, len(reserved)),
	}

	for _, word := range reserved {
		dialect.reserved[word] = struct{}{}
	}

	return &dialect
}

// PostgresType maps a schema type and format onto a Postgres type. It returns
// EINVALID when the type has no equivalent.
func (d *Dialect) PostgresType(dataType, dataFormat string) (string, error) {
	if pgDataType, ok := d.dataTypes[dataType+":"+dataFormat]; ok {
		return pgDataType, nil
	}

	// An unrecognised format should not discard a known type: a string of
	// format "uri" is still TEXT.
	if pgDataType, ok := d.dataTypes[dataType+":"]; ok {
		return pgDataType, nil
	}

	return "", &Error{
		Code:    EINVALID,
		Message: fmt.Sprintf("no Postgres type for %s/%s", dataType, dataFormat),
	}
}

// QuoteIdentifier quotes name when Postgres would otherwise read it as a
// keyword.
func (d *Dialect) QuoteIdentifier(name string) string {
	if _, ok := d.reserved[strings.ToUpper(name)]; ok {
		return fmt.Sprintf("%q", name)
	}

	return name
}
