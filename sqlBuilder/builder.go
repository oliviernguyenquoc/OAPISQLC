package sqlBuilder

import (
	"fmt"
	"strings"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func GETSQLStatement(resource string) string {
	var GETStatement string

	GETStatement += fmt.Sprintf("SELECT * FROM %s", resource)
	GETStatement += "\n\n"

	return GETStatement
}

func generateValuePlaceholders(n int) string {
	var placeholders string
	for i := 0; i < n; i++ {
		placeholders += fmt.Sprintf("$%d", i+1)
		if i != n-1 {
			placeholders += ", "
		}
	}

	return placeholders
}

func POSTSQLStatement(operation *v3.Operation, resource string) string {
	var POSTStatement string

	// Table name to insert
	json := operation.RequestBody.Content.Value("application/json")
	requestBodySchema := json.Schema.Schema()
	insertColumns := requestBodySchema.Properties

	var columnArray []string

	for column := insertColumns.First(); column != nil; column = column.Next() {
		columnArray = append(columnArray, column.Key())
	}

	if len(columnArray) == 0 {
		return ""
	}

	POSTStatement += fmt.Sprintf(
		"INSERT INTO %s (%v) VALUES (%s);",
		resource,
		strings.Join(columnArray, ", "),
		generateValuePlaceholders(insertColumns.Len()),
	)
	POSTStatement += "\n\n"

	return POSTStatement
}

func CommentSQLC(operation *v3.Operation) string {
	var commentSQLCStatement string
	var isMany = false

	// Check if response is an array based on the schema type
	if operation.Responses != nil && operation.Responses.Codes != nil && operation.Responses.Codes.Value("200") != nil {
		response := operation.Responses.Codes.Value("200")
		if response.Content != nil && response.Content.Value("application/json") != nil && response.Content.Value("application/json").Schema != nil {
			schema := response.Content.Value("application/json").Schema.Schema()
			if schema.Type != nil && schema.Type[0] == "array" {
				isMany = true
			}
		}
	}

	if isMany {
		commentSQLCStatement = fmt.Sprintf("-- name: %s :many \n", operation.OperationId)
	} else {
		commentSQLCStatement = fmt.Sprintf("-- name: %s :one \n", operation.OperationId)
	}

	return commentSQLCStatement
}

func PUTSQLStatement(operation *v3.Operation, resource string) string {
	var PUTStatement string

	// Table name to insert
	json := operation.RequestBody.Content.Value("application/json")
	requestBodySchema := json.Schema.Schema()
	insertColumns := requestBodySchema.Properties

	var columnArray []string
	var valuePlaceholders []string

	for column := insertColumns.First(); column != nil; column = column.Next() {
		columnArray = append(columnArray, column.Key())
		valuePlaceholders = append(valuePlaceholders, fmt.Sprintf("%s=$%d", column.Key(), len(columnArray)))
	}

	if len(columnArray) == 0 {
		return ""
	}

	// TODO: Adapt the WHERE clause to the actual path parameters
	PUTStatement += fmt.Sprintf(
		"UPDATE %s SET %s WHERE id = $%d",
		resource,
		strings.Join(valuePlaceholders, ", "),
		len(columnArray)+1,
	)
	PUTStatement += "\n\n"

	return PUTStatement
}

func DELETESQLStatement(operation *v3.Operation, resource string) string {
	var DELETEStatement string

	DELETEStatement += fmt.Sprintf("DELETE FROM %s WHERE id = $1", resource)
	DELETEStatement += "\n\n"

	return DELETEStatement
}
