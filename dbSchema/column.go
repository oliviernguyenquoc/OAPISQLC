package dbSchema

import (
	"fmt"
	"strings"
)

type Constraints struct {
	Minimum *float64
	Maximum *float64
}

type Column struct {
	Name         string
	DataType     string
	DataFormat   string
	NotNull      bool
	DefaultValue string
	PrimaryKey   bool
	Constraints  Constraints
	Unique       bool
	customType   string
	Enum         []string
}

var datatypeMap = map[string]string{
	"integer:":         "INTEGER",
	"integer:int32":    "INTEGER",
	"integer:int64":    "BIGINT",
	"boolean:":         "BOOLEAN",
	"number:":          "REAL",
	"number:float":     "REAL",
	"number:double":    "DOUBLE PRECISION",
	"file:":            "BYTEA",
	"string:":          "TEXT",
	"string:byte":      "BYTEA",
	"string:binary":    "BYTEA",
	"string:date":      "DATE",
	"string:date-time": "TIMESTAMP",
	"string:enum":      "TEXT",
	"array:":           "JSON",
	"object:":          "JSON",
	"\\Model\\User:":   "TEXT",
}

func buildConstraint(name string, minimum *float64, maximum *float64) string {
	var conditions []string

	if minimum != nil {
		conditions = append(conditions, fmt.Sprintf("%s >= %f", name, *minimum))
	}

	if maximum != nil {
		conditions = append(conditions, fmt.Sprintf("%s <= %f", name, *maximum))
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

	pgDataType, ok = datatypeMap[c.DataType+":"+c.DataFormat]
	if !ok {
		fmt.Printf("Unknown data type: %s\n", c.DataType)
		return "", fmt.Errorf("unknown data type: %s", c.DataType)
	}

	// Handle special case for id column
	if c.Name == "id" && c.DataType == "integer" {
		pgDataType = "BIGSERIAL"
		c.NotNull = true
	}

	// Handle special case for created_at and updated_at columns
	if c.Name == "created_at" || c.Name == "updated_at" {
		pgDataType = "TIMESTAMP"
		c.NotNull = true
		c.DefaultValue = "NOW()"
	}

	// Handle special case for enum
	if c.DataType == "string" && len(c.Enum) > 0 && c.customType != "" {
		pgDataType = c.customType
	}

	sb.WriteString(fmt.Sprintf("%s %s", c.Name, pgDataType))

	if c.NotNull {
		sb.WriteString(" NOT NULL")
	}

	if c.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}

	// Handle constraints
	constraint := buildConstraint(c.Name, c.Constraints.Minimum, c.Constraints.Maximum)
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

	if c.Unique {
		sb.WriteString(" UNIQUE")
	}

	return sb.String(), nil
}
