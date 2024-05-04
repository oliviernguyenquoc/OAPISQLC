package tables

import (
	"fmt"
	"strings"
)

type Column struct {
	Name       string
	DataType   string
	NotNull    bool
	IsDefault  bool
	IsString   bool
	PrimaryKey bool
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
	var sb strings.Builder

	var pgDataType, dataType string
	var ok bool

	pgDataType, ok = datatypeMap[c.DataType]
	if !ok {
		fmt.Printf("Unknown data type: %s\n", dataType)
		return "", fmt.Errorf("unknown data type: %s", dataType)
	}

	// Handle special case for id column
	if c.Name == "id" && pgDataType == "INTEGER" {
		pgDataType = "BIGSERIAL"
		c.NotNull = true
	}

	sb.WriteString(fmt.Sprintf("%s %s", c.Name, pgDataType))

	if c.NotNull {
		sb.WriteString(" NOT NULL")
	}

	if c.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}

	if c.IsDefault {
		if c.IsString {
			sb.WriteString(fmt.Sprintf(" DEFAULT '%s'", c.IsDefault))
		} else {
			sb.WriteString(fmt.Sprintf(" DEFAULT %s", c.IsDefault))
		}
	}

	return sb.String(), nil
}
