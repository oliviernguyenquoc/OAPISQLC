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
		t.Errorf("Error parsing expected SQL: %v", err)
	}

	actualTree, err := pg_query.Parse(actualSQL)
	if err != nil {
		t.Errorf("Error parsing actual SQL: %v", err)
	}

	if actualTree.Fingerprint() != expectedTree.Fingerprint() {
		t.Errorf(`
		Expected SQL did not match (at least, they do not have the samefingerprinting). 
		Got: %v
		
		Wanted: %v
		`,
			actualSQL, expectedSQL)
	}
}

func testOpenAPISpecToSQL(t *testing.T, filename, expectedSQL string) {
	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading OpenAPI spec: %v", err)
	}

	sql, err := OpenAPISpecToSQL(apiSpec)
	if err != nil {
		t.Errorf("Error transforming OpenAPI to SQL: %v", err)
	}

	compareSQL(t, expectedSQL, sql)
}

func TestSimpleSchemaTransformation(t *testing.T) {

	testOpenAPISpecToSQL(t, "tests/testdata/simple_schema.yaml", `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		username TEXT
	);`)
}

func TestTagManagement(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/tag_management.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
		name TEXT
	);`)
}

func TestCustomExtensions(t *testing.T) {

	// No table should be created for ignored schemas.
	testOpenAPISpecToSQL(t, "tests/testdata/exclusion_extension.yaml", "")
}

func TestComponentReferences(t *testing.T) {

	testOpenAPISpecToSQL(t, "tests/testdata/component_references.yaml", `
	CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        address_id INTEGER,
		FOREIGN KEY (address_id) REFERENCES addresses(id)
    );
    CREATE TABLE IF NOT EXISTS addresses (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        street TEXT,
        city TEXT
    );`)
}

func TestDataTypeAndConstraints(t *testing.T) {

	testOpenAPISpecToSQL(t, "tests/testdata/data_types_and_constraints.yaml", `
	CREATE TABLE IF NOT EXISTS products (
        price NUMERIC CHECK (price >= 0 AND price <= 100000),
        status TEXT CHECK (status IN ('available', 'pending', 'sold')),
		quantity INTEGER NOT NULL
    );`)
}

func TestCircularReferences(t *testing.T) {

	// No table should be created if the references are circular and cannot be resolved.
	testOpenAPISpecToSQL(t, "tests/testdata/circular_references.yaml", "")
}

func TestAllOfSchema(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/allOf_example.yaml", `
	CREATE TABLE IF NOT EXISTS animals (
        name TEXT NOT NULL,
        type TEXT NOT NULL
    );
	CREATE TABLE IF NOT EXISTS dogs (
        name TEXT  NOT NULL,
        type TEXT  NOT NULL,
        breed TEXT  NOT NULL,
        barkVolume INTEGER
    );`)
}

func TestIdCreatedAtUpdatedAt(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/id_created_at_updated_at.yaml", `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		username TEXT
	);`)
}

func TestArrayOfRef(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/array_of_ref.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT NOT NULL,
		tag_id INTEGER,
		FOREIGN KEY (tag_id) REFERENCES tags(id)
	);

	CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);`)
}
