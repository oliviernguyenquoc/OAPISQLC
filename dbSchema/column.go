package dbSchema

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jinzhu/inflection"
	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"golang.org/x/exp/slog"
)

// Constraint interface to illustrate the concept of column constraints
type Constraint interface {
	GetConstraint(columnName string) []string
}

type MinMaxConstraint struct {
	Minimum *float64
	Maximum *float64
}

type CharLengthConstraint struct {
	MinLength *int64
	MaxLength *int64
}

type PatternConstraint struct {
	Pattern string
}

type Column struct {
	Name                 string
	DataType             string
	DataFormat           string
	NotNull              bool
	DefaultValue         string
	PrimaryKey           bool
	MinMaxConstraint     MinMaxConstraint
	CharLengthConstraint CharLengthConstraint
	PatternConstraint    PatternConstraint
	Unique               bool
	customType           string
	Enum                 []string
	ForeignKey           string
}

var datatypeMap = map[string]string{
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
	"string:enum":      "TEXT",
	"array:":           "JSON",
	"object:":          "JSON",
	"\\Model\\User:":   "TEXT",
}

func (mm MinMaxConstraint) GetConstraint(columnName string) []string {
	conditions := make([]string, 0, 2)

	if mm.Minimum != nil {
		conditions = append(conditions, fmt.Sprintf("%s >= %f", columnName, *mm.Minimum))
	}

	if mm.Maximum != nil {
		conditions = append(conditions, fmt.Sprintf("%s <= %f", columnName, *mm.Maximum))
	}

	return conditions
}

func (cl CharLengthConstraint) GetConstraint(columnName string) []string {
	conditions := make([]string, 0, 2)

	if cl.MinLength != nil {
		conditions = append(conditions, fmt.Sprintf("char_length(%s) >= %d", columnName, *cl.MinLength))
	}

	if cl.MaxLength != nil {
		conditions = append(conditions, fmt.Sprintf("char_length(%s) <= %d", columnName, *cl.MaxLength))
	}

	return conditions
}

func (pc PatternConstraint) GetConstraint(columnName string) []string {
	if pc.Pattern != "" {
		return []string{fmt.Sprintf("%s ~ '%s'", columnName, pc.Pattern)}
	}
	return []string{}
}

func (c Column) GetConstraint() string {
	conditions := make([]string, 0, 5) // Pre-allocate with expected capacity

	constraints := []Constraint{c.MinMaxConstraint, c.CharLengthConstraint, c.PatternConstraint}
	for _, constraint := range constraints {
		conditions = append(conditions, constraint.GetConstraint(c.Name)...)
	}

	if len(conditions) > 0 {
		return " CHECK (" + strings.Join(conditions, " AND ") + ")"
	}
	return ""
}

func (c Column) CreateSQLStatement() (string, error) {
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
	sb.WriteString(c.GetConstraint())

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

	if c.ForeignKey != "" {
		sb.WriteString(fmt.Sprintf(" REFERENCES %s(id)", c.ForeignKey))
	}

	return sb.String(), nil
}

func buildColumnFromProperty(tableName string, property orderedmap.Pair[string, *highbase.SchemaProxy], requiredColumns []string) (Column, error) {
	columnName := property.Key()
	columnSchema := property.Value().Schema()

	var dataType string
	if len(columnSchema.Type) > 0 {
		dataType = columnSchema.Type[0]
	} else {
		return Column{}, fmt.Errorf("no data type found for property: %s", columnName)
	}

	var dataFormat string
	if len(columnSchema.Format) > 0 {
		dataFormat = columnSchema.Format
	}

	// Detect if the property is a $ref to another schema
	// This is used to determine if the column is a foreign key
	ref := property.Value().GetReference()
	var foreignKey string

	if ref != "" || (dataType == "array" && columnSchema.Items != nil && columnSchema.Items.A.Schema().Properties != nil) {
		foreignKey = inflection.Plural(columnName)
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

	return Column{
		Name:         columnName,
		DataType:     dataType,
		DataFormat:   dataFormat,
		PrimaryKey:   columnName == "id",
		NotNull:      (columnSchema.Nullable != nil && !*columnSchema.Nullable) || slices.Contains(requiredColumns, columnName),
		DefaultValue: defaultValue,
		MinMaxConstraint: MinMaxConstraint{
			Minimum: columnSchema.Minimum,
			Maximum: columnSchema.Maximum,
		},
		CharLengthConstraint: CharLengthConstraint{
			MinLength: columnSchema.MinLength,
			MaxLength: columnSchema.MaxLength,
		},
		PatternConstraint: PatternConstraint{
			Pattern: columnSchema.Pattern,
		},
		Unique:     unique,
		customType: enumType,
		Enum:       enum,
		ForeignKey: foreignKey,
	}, nil
}

func BuildColumnsFromSchema(tableName string, properties orderedmap.Map[string, *highbase.SchemaProxy], requiredColumns []string) ([]Column, error) {

	var columns []Column

	for property := properties.First(); property != nil; property = property.Next() {
		column, err := buildColumnFromProperty(tableName, property, requiredColumns)
		if err != nil {
			slog.Error("error building column for %s: %v", property.Key(), err)
			return nil, fmt.Errorf("could not build column for %s", property.Key())

		}
		columns = append(columns, column)
	}
	return columns, nil
}
