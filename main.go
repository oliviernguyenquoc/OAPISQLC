package main

import (
	"fmt"
	"os"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// parseOpenAPISpec takes the path to an OpenAPI YAML file, parses it using the libopenapi library,
// and returns the parsed data structure or an error if something goes wrong.
func parseOpenAPISpec(openAPISpec []byte) (*v3.Components, error) {

	// create a new document from specification bytes
	document, err := libopenapi.NewDocument(openAPISpec)
	if err != nil {
		panic(fmt.Sprintf("cannot create new document: %e", err))
	}

	// because we know this is a v3 spec, we can build a ready to go model from it.
	v3Model, errors := document.BuildV3Model()

	// if anything went wrong when building the v3 model, a slice of errors will be returned
	if len(errors) > 0 {
		for i := range errors {
			fmt.Printf("error: %e\n", errors[i])
		}
		panic(fmt.Sprintf("cannot create v3 model from document: %d errors reported",
			len(errors)))
	}

	// get a count of the number of paths and schemas.
	paths := v3Model.Model.Paths.PathItems
	schemas := v3Model.Model.Components.Schemas

	// print the number of paths and schemas in the document
	fmt.Printf("There are %d paths and %d schemas in the document", paths.Len(), schemas.Len())

	return v3Model.Model.Components, nil
}

// fromComponentsToSQL takes a parsed OpenAPI document and generates a SQL statement.
// This is a placeholder function that should be replaced with actual logic based on the OpenAPI spec.
func fromComponentsToSQL(doc *v3.Components) (string, error) {

	schemas := doc.Schemas
	print(schemas)
	// Placeholder SQL generation logic
	return "SELECT * FROM information_schema.tables;", nil
}

func OpenAPISpecToSQL(openAPISpec []byte) (string, error) {

	// Parse the OpenAPI specification
	doc, err := parseOpenAPISpec(openAPISpec)
	if err != nil {
		fmt.Printf("Failed to parse OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	// Generate SQL statement based on the OpenAPI spec
	sqlStatement, err := fromComponentsToSQL(doc)
	if err != nil {
		fmt.Printf("Failed to generate SQL: %v\n", err)
		os.Exit(1)
	}

	return sqlStatement, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <path_to_yaml_file>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// load an OpenAPI 3 specification from bytes
	openAPISpec, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Failed to read OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	sqlStatement, err := OpenAPISpecToSQL(openAPISpec)
	if err != nil {
		fmt.Printf("Failed to transform OpenAPI spec to SQL: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated SQL Statement:", sqlStatement)

}
