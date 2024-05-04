package dbSchema

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

func buildConstraint(name string, minimum *float64, maximum *float64, enum []string) string {
	var conditions []string

	if minimum != nil {
		conditions = append(conditions, fmt.Sprintf("%s >= %f", name, *minimum))
	}

	if maximum != nil {
		conditions = append(conditions, fmt.Sprintf("%s <= %f", name, *maximum))
	}

	if len(enum) > 0 {
		quotedEnums := make([]string, len(enum))
		for i, v := range enum {
			quotedEnums[i] = fmt.Sprintf("'%s'", v)
		}
		conditions = append(conditions, fmt.Sprintf("%s IN (%s)", name, strings.Join(quotedEnums, ", ")))
	}

	if len(conditions) > 0 {
		return " CHECK (" + strings.Join(conditions, " AND ") + ")"
	}
	return ""
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
	constraint := buildConstraint(c.Name, c.Constraints.Minimum, c.Constraints.Maximum, c.Constraints.Enum)
	if constraint != "" {
		sb.WriteString(constraint)
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
