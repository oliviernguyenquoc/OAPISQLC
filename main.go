package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/oliviernguyenquoc/oas2pgschema/dbSchema"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	pg_query "github.com/pganalyze/pg_query_go/v5"
)

// parseOpenAPISpec takes the path to an OpenAPI YAML file, parses it using the libopenapi library,
// and returns the parsed data structure or an error if something goes wrong.
func parseOpenAPISpec(openAPISpec []byte) (*v3.Components, error) {

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

	return v3Model.Model.Components, nil
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

func OpenAPISpecToSQL(openAPISpec []byte, flags Flags) (string, error) {

	// Parse the OpenAPI specification
	doc, err := parseOpenAPISpec(openAPISpec)
	if err != nil {
		fmt.Printf("Failed to parse OpenAPI spec: %v\n", err)
		return "", err
	}

	// Generate SQL statement based on the OpenAPI spec
	sqlStatement, err := fromComponentsToSQL(doc, flags)
	if err != nil {
		fmt.Printf("Failed to generate SQL: %v\n", err)
		return "", err
	}

	return sqlStatement, nil
}

// flags
type Flags struct {
	deleteStatements bool
	outputFilePath   string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path_to_yaml_file>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Parse flags
	deleteStatements := flag.Bool("deleteStatements", true, "Add delete statements to SQL output")
	outputFilePath := flag.String("o", "", "Path to output file")

	flags := Flags{
		deleteStatements: *deleteStatements,
		outputFilePath:   *outputFilePath,
	}

	// load an OpenAPI 3.1 specification from bytes
	openAPISpec, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Failed to read OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	sqlStatement, err := OpenAPISpecToSQL(openAPISpec, flags)
	if err != nil {
		fmt.Printf("Failed to transform OpenAPI spec to SQL: %v\n", err)
		os.Exit(1)
	}

	if flags.outputFilePath != "" {
		err = os.WriteFile(flags.outputFilePath, []byte(sqlStatement), 0644)
		if err != nil {
			fmt.Printf("Failed to write SQL to file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SQL written to %s\n", flags.outputFilePath)
	} else {
		fmt.Println("Generated SQL Statement:", sqlStatement)
	}
}
