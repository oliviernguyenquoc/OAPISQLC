package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/oliviernguyenquoc/oapisqlc/dbSchema"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	pg_query "github.com/pganalyze/pg_query_go/v5"
)

// parseOpenAPISpec takes the path to an OpenAPI YAML file, parses it using the libopenapi library,
// and returns the parsed data structure or an error if something goes wrong.
func parseOpenAPISpec(openAPISpec []byte) (*v3.Document, error) {

	// create a new document from specification bytes
	document, err := libopenapi.NewDocument(openAPISpec)
	if err != nil {
		return nil, fmt.Errorf("cannot create document from OpenAPI spec: %v", err)
	}

	// because we know this is a v3 spec, we can build a ready to go model from it.
	v3Model, errors := document.BuildV3Model()
	if len(errors) > 0 {
		for i := range errors {
			fmt.Printf("error: %e\n", errors[i])
		}

		return nil, fmt.Errorf("cannot create v3 model from document: %d errors reported", len(errors))
	}

	// Get a count of the number of paths and schemas.
	var nbPaths int
	if v3Model.Model.Paths == nil {
		nbPaths = 0
	} else {
		nbPaths = v3Model.Model.Paths.PathItems.Len()
	}

	var nbSchemas int
	if v3Model.Model.Components.Schemas == nil {
		nbSchemas = 0
	} else {
		nbSchemas = v3Model.Model.Components.Schemas.Len()
	}

	// Print the number of paths and schemas in the document
	slog.Info("There are %s paths and %d schemas in the document", strconv.Itoa(nbPaths), nbSchemas)

	return &v3Model.Model, nil
}

// fromComponentsToSQL takes a parsed OpenAPI document and generates a SQL statement.
func fromComponentsToSQL(doc *v3.Components, flags Flags) (string, error) {

	schemas := doc.Schemas

	var tableDefinitions []dbSchema.Table

	for schema := schemas.First(); schema != nil; schema = schema.Next() {
		tableName := schema.Key()
		table := dbSchema.BuildTableFromSchema(tableName, schema.Value().Schema())

		// If there is no column, no need to create a table
		if len(table.ColumnDefinition) != 0 {
			tableDefinitions = append(tableDefinitions, *table)
		}
	}

	var query string

	// Add delete statements at the beginning of the output file
	for _, table := range tableDefinitions {
		if flags.deleteStatements {
			deleteStatement := table.DeleteSQLStatement()
			query += deleteStatement
		}
	}

	for _, table := range tableDefinitions {
		statement, err := table.CreateSQLStatement()
		if err != nil {
			return "", err
		}
		query += "\n\n"
		query += statement
	}

	normalizedQuery, err := pg_query.Normalize(query)
	if err != nil {
		slog.Error("Error checking and normalizing query %s", query, err)
		return "", err
	}

	// Placeholder SQL generation logic
	return normalizedQuery, nil
}

func fromComponentPathToSQL(doc *v3.Paths, flags Flags) ([]string, error) {
	paths := doc.PathItems

	var pathSQLStatements []string

	for path := paths.First(); path != nil; path = path.Next() {
		pathItem := path.Value()

		if pathItem.Get != nil {
			operation := pathItem.Get

			var operationSQLStatement string
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
				operationSQLStatement = fmt.Sprintf("-- name: %s :many \n", operation.OperationId)
			} else {
				operationSQLStatement = fmt.Sprintf("-- name: %s :one \n", operation.OperationId)
			}
			operationSQLStatement += fmt.Sprintf("SELECT * FROM %s", operation.OperationId)
			operationSQLStatement += "\n\n"
		}
	}

	return pathSQLStatements, nil
}

func writeInFolder(sqlStatement string, flags Flags) error {
	// Create folder if not exist
	err := os.MkdirAll(flags.outputFolderPath, 0755)
	if err != nil {
		fmt.Printf("Failed to create output folder: %v\n", err)
		return err
	}

	err = os.WriteFile(filepath.Join(flags.outputFolderPath, "schemas.sql"), []byte(sqlStatement), 0644)
	if err != nil {
		fmt.Printf("Failed to write SQL to file: %v\n", err)
		return err
	}
	fmt.Printf("SQL written in folder %s\n", flags.outputFolderPath)

	return nil
}

// flags
type Flags struct {
	deleteStatements bool
	outputFolderPath string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path_to_yaml_file>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Parse flags
	deleteStatements := flag.Bool("deleteStatements", false, "Add delete statements to SQL output")
	outputFolderPath := flag.String("outputFolder", "", "Path to output folder")

	flags := Flags{
		deleteStatements: *deleteStatements,
		outputFolderPath: *outputFolderPath,
	}

	// load an OpenAPI 3.1 specification from bytes
	openAPISpec, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Failed to read OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	// Parse the OpenAPI specification
	doc, err := parseOpenAPISpec(openAPISpec)
	if err != nil {
		fmt.Printf("Failed to parse OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	// Generate SQL statement based on the OpenAPI spec
	DDLSQLStatement, err := fromComponentsToSQL(doc.Components, flags)
	if err != nil {
		fmt.Printf("Failed to generate SQL: %v\n", err)
		os.Exit(1)
	}

	PathSQLStatements, err := fromComponentPathToSQL(doc.Paths, flags)
	if err != nil {
		fmt.Printf("Failed to generate SQL: %v\n", err)
		os.Exit(1)
	}

	if flags.outputFolderPath != "" {
		err := writeInFolder(DDLSQLStatement, flags)
		if err != nil {
			os.Exit(1)
		}
	} else {
		fmt.Println("Generated SQL Statement:", DDLSQLStatement)
		fmt.Print("\n\n")
		fmt.Println("Generated Path SQL Statements:", PathSQLStatements)
	}
}
