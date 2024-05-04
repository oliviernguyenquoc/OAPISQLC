package tables

import (
	"fmt"
	"strings"
)

type Constraints struct {
	Minimum *float64
	Maximum *float64
	Enum    []string
}

type Column struct {
	Name         string
	DataType     string
	NotNull      bool
	DefaultValue string
	PrimaryKey   bool
	Constraints  Constraints
}

var datatypeMap = map[string]string{
	"integer":       "INTEGER",
	"int32":         "INTEGER",
	"int64":         "BIGINT",
	"boolean":       "BOOLEAN",
	"number":        "NUMERIC",
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

	var pgDataType string
	var ok bool

	pgDataType, ok = datatypeMap[c.DataType]
	if !ok {
		fmt.Printf("Unknown data type: %s\n", c.DataType)
		return "", fmt.Errorf("unknown data type: %s", c.DataType)
	}

	// Handle special case for id column
	if c.Name == "id" && pgDataType == "INTEGER" {
		pgDataType = "BIGSERIAL"
		c.NotNull = true
	}

	// Handle special case for created_at and updated_at columns
	if c.Name == "created_at" || c.Name == "updated_at" {
		pgDataType = "TIMESTAMP"
		c.NotNull = true
		c.DefaultValue = "NOW()"
	}

	sb.WriteString(fmt.Sprintf("%s %s", c.Name, pgDataType))

	if c.NotNull {
		sb.WriteString(" NOT NULL")
	}

	if c.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}

	// Handle constraints
	if len(c.Constraints.Enum) > 0 || c.Constraints.Minimum != nil || c.Constraints.Maximum != nil {
		sb.WriteString(" CHECK (")
		if c.Constraints.Minimum != nil {
			sb.WriteString(fmt.Sprintf("%s >= %f", c.Name, *c.Constraints.Minimum))
		}
		if c.Constraints.Minimum != nil && c.Constraints.Maximum != nil {
			sb.WriteString(" AND ")
		}
		if c.Constraints.Maximum != nil {
			sb.WriteString(fmt.Sprintf("%s <= %f", c.Name, *c.Constraints.Maximum))
		}
		if len(c.Constraints.Enum) > 0 {
			if c.Constraints.Minimum != nil || c.Constraints.Maximum != nil {
				sb.WriteString(" AND ")
			}
			sb.WriteString(fmt.Sprintf("%s IN (", c.Name))
			for i, v := range c.Constraints.Enum {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("'%s'", v))
			}
			sb.WriteString(")")
		}
		sb.WriteString(")")
	}

	if c.DefaultValue != "" {
		if pgDataType == "TEXT" {
			sb.WriteString(fmt.Sprintf(" DEFAULT '%s'", c.DefaultValue))
		} else {
			sb.WriteString(fmt.Sprintf(" DEFAULT %s", c.DefaultValue))
		}
	}

	return sb.String(), nil
}
