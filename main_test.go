package main

import (
	"os"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go"
)

// Comparison based on fingerprinting
func compareSQL(t *testing.T, expectedSQL, actualSQL string) {

	expectedTree, err := pg_query.Parse(expectedSQL)
	if err != nil {
		t.Fatalf("Error parsing expected SQL: %v", err)
	}

	actualTree, err := pg_query.Parse(actualSQL)
	if err != nil {
		t.Fatalf("Error parsing actual SQL: %v", err)
	}

	if actualTree.Fingerprint() != expectedTree.Fingerprint() {
		t.Errorf(`
		Expected SQL did not match (at least, they do not have the samefingerprinting). 
		Got %v, wanted %v
		`,
			actualSQL, expectedSQL)
	}
}

func testOpenAPISpecToSQL(t *testing.T, filename, expectedSQL string) {
	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Error reading OpenAPI spec: %v", err)
	}

	sql, err := OpenAPISpecToSQL(apiSpec)
	if err != nil {
		t.Fatalf("Error transforming OpenAPI to SQL: %v", err)
	}

	compareSQL(t, expectedSQL, sql)
}

func TestSimpleSchemaTransformation(t *testing.T) {

	testOpenAPISpecToSQL(t, "tests/testdata/simple_schema.yaml", `
	CREATE TABLE users (
		id BIGINT,
		username VARCHAR(255)
	);`)
}

func TestTagManagement(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/tag_management.yaml", `
	CREATE TABLE pets (
		id BIGINT,
		name VARCHAR(255)
	);`)
}

func TestCustomExtensions(t *testing.T) {

	// No table should be created for ignored schemas.
	testOpenAPISpecToSQL(t, "tests/testdata/exclusion_extension.yaml", "")
}

func TestComponentReferences(t *testing.T) {

	testOpenAPISpecToSQL(t, "tests/testdata/component_references.yaml", `
	CREATE TABLE users (
        id BIGINT,
        address_id BIGINT
    );
    CREATE TABLE addresses (
        id BIGINT,
        street VARCHAR(255),
        city VARCHAR(255)
    );`)
}

func TestDataTypeAndConstraints(t *testing.T) {

	testOpenAPISpecToSQL(t, "tests/testdata/data_types_and_constraints.yaml", `
	CREATE TABLE products (
        id BIGINT,
        price NUMERIC CHECK (price >= 0),
        status VARCHAR(50) CHECK (status IN ('available', 'pending', 'sold'))
    );`)
}

func TestCircularReferences(t *testing.T) {

	// No table should be created if the references are circular and cannot be resolved.
	testOpenAPISpecToSQL(t, "tests/testdata/circular_references.yaml", `
	CREATE TABLE products (
        id BIGINT,
        price NUMERIC CHECK (price >= 0),
        status VARCHAR(50) CHECK (status IN ('available', 'pending', 'sold'))
    );`)
}

func TestAllOfSchema(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/allOf_example.yaml", `CREATE TABLE dogs (
        id BIGINT,
        name VARCHAR(255),
        type VARCHAR(255),
        breed VARCHAR(255),
        bark_volume INTEGER
    );`)
}

func TestAnyOfSchema(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/anyOf_example.yaml", `CREATE TABLE text_or_numbers (
        id BIGINT,
        value TEXT CHECK (value ~* '^\d+$' OR LENGTH(value) <= 100)
    );`)
}
