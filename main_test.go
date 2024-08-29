package main

import (
	"os"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v5"
)

// Comparison based on fingerprinting
func compareSQL(t *testing.T, expectedSQL, actualSQL string) {

	expectedFingerprint, err := pg_query.Fingerprint(expectedSQL)
	if err != nil {
		t.Errorf("Error parsing expected SQL: %v", err)
	}

	actualFingerprint, err := pg_query.Fingerprint(actualSQL)
	if err != nil {
		t.Errorf("Error parsing actual SQL: %v", err)
	}

	if expectedFingerprint != actualFingerprint {
		t.Errorf(`
		Expected SQL did not match (at least, they do not have the samefingerprinting). 
		Got: %v
		
		Wanted: %v
		`,
			actualSQL, expectedSQL)
	}
}

func testOpenAPISpecToDDLSQL(t *testing.T, filename, expectedSQL string, flags Flags) {

	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading OpenAPI spec: %v", err)
	}

	// Parse the OpenAPI specification
	doc, err := parseOpenAPISpec(apiSpec)
	if err != nil {
		t.Errorf("Error parsing OpenAPI spec: %v", err)
	}

	sql, err := fromComponentsToSQL(doc.Components, flags)
	if err != nil {
		t.Errorf("Error transforming OpenAPI to SQL: %v", err)
	}
	compareSQL(t, expectedSQL, sql)
}

func testOpenAPISpecToPathSQL(t *testing.T, filename, expectedSQL string, flags Flags) {

	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading OpenAPI spec: %v", err)
	}

	// Parse the OpenAPI specification
	doc, err := parseOpenAPISpec(apiSpec)
	if err != nil {
		t.Errorf("Error parsing OpenAPI spec: %v", err)
	}

	sql, err := fromComponentPathToSQL(doc.Paths, flags)
	if err != nil {
		t.Errorf("Error transforming OpenAPI to SQL: %v", err)
	}
	compareSQL(t, expectedSQL, sql)
}

func testErrors(t *testing.T, filename string) {
	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading OpenAPI spec: %v", err)
	}

	// Parse the OpenAPI specification
	_, errParsing := parseOpenAPISpec(apiSpec)

	expectedErrorMsg := "Cannot create v3 model from document: 1 errors reported"

	if errParsing == nil || errParsing.Error() != expectedErrorMsg {
		t.Errorf("Expected error message: %s, got: %v", expectedErrorMsg, errParsing)
	}
}

func TestSimpleSchemaTransformation(t *testing.T) {

	testOpenAPISpecToDDLSQL(t, "tests/testdata/simple_schema.yaml", `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		username TEXT
	);`, Flags{})
}

func TestTagManagement(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/tag_management.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
		name TEXT
	);`, Flags{})
}

func TestCustomExtensions(t *testing.T) {

	// No table should be created for ignored schemas.
	testOpenAPISpecToDDLSQL(t, "tests/testdata/exclusion_extension.yaml", "", Flags{})
}

func TestComponentReferences(t *testing.T) {

	testOpenAPISpecToDDLSQL(t, "tests/testdata/component_references.yaml", `
	CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        address_id INTEGER REFERENCES addresses(id)
    );
    CREATE TABLE IF NOT EXISTS addresses (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        street TEXT,
        city TEXT
    );`, Flags{})
}

func TestComponentReferencesWithDeleteStatements(t *testing.T) {

	testOpenAPISpecToDDLSQL(t, "tests/testdata/component_references.yaml", `
	DROP TABLE IF EXISTS users CASCADE;
	DROP TABLE IF EXISTS addresses CASCADE;

	CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        address_id INTEGER REFERENCES addresses(id)
    );
    CREATE TABLE IF NOT EXISTS addresses (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        street TEXT,
        city TEXT
    );`, Flags{deleteStatements: true})
}

func TestDataTypes(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/data_types_conversion.yaml", `
	CREATE TABLE IF NOT EXISTS data_type_examples (
		smallInt INTEGER,
		bigInt BIGINT,
		booleanValue BOOLEAN,
		floatValue REAL,
		doubleValue DOUBLE PRECISION,
		simpleText TEXT,
		byteData BYTEA,
		binaryData BYTEA,
		fileData BYTEA,
		dateValue DATE,
		dateTimeValue TIMESTAMP,
		arrayValue JSON,
		objectValue JSON
	);`, Flags{})
}

func TestConstraintsTranslation(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/constraints.yaml", `
    CREATE TABLE IF NOT EXISTS products (
        productId INTEGER CHECK (productId >= 1.000000 AND productId <= 1000.000000),
        productName TEXT CHECK (char_length(productName) >= 1 AND char_length(productName) <= 100),
        productPrice NUMERIC CHECK (productPrice >= 0.010000 AND productPrice <= 9999.990000),
        productCode TEXT CHECK (productCode ~ '^[A-Z0-9]{10}$'),
        releaseDate DATE DEFAULT 2023-01-01
    );`, Flags{})
}

func TestCircularReferencesParsingError(t *testing.T) {

	// Should return an error if there are circular references, detected during parsing.
	testErrors(t, "tests/testdata/circular_references_parsing_error.yaml")
}

func TestCircularReferences(t *testing.T) {

	// No table should be created if the references are circular and cannot be resolved.
	testOpenAPISpecToDDLSQL(t, "tests/testdata/circular_references.yaml", "", Flags{})
}

func TestAllOfSchema(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/allOf_example.yaml", `
	CREATE TABLE IF NOT EXISTS animals (
        name TEXT NOT NULL,
        type TEXT NOT NULL
    );
	CREATE TABLE IF NOT EXISTS dogs (
        name TEXT  NOT NULL,
        type TEXT  NOT NULL,
        breed TEXT  NOT NULL,
        barkVolume INTEGER
    );`, Flags{})
}

func TestIdCreatedAtUpdatedAt(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/id_created_at_updated_at.yaml", `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		username TEXT
	);`, Flags{})
}

func TestArrayOfRef(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/array_of_ref.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT NOT NULL,
		tag_id INTEGER REFERENCES tags(id)
	);

	CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);`, Flags{})
}

func TestDefaultValues(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/default_values.yaml", `
    CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        username TEXT DEFAULT 'anonymous',
        signup_date DATE DEFAULT 2023-01-01
    );`, Flags{})
}

func TestUniqueConstraints(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/unique_constraints.yaml", `
    CREATE TABLE IF NOT EXISTS products (
        productId TEXT UNIQUE,
        serialNumber TEXT UNIQUE,
        name TEXT
    );`, Flags{})
}

func TestEnumSupport(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/enum_definition.yaml", `
    CREATE TYPE order_status AS ENUM ('pending', 'approved', 'shipped', 'cancelled');

    CREATE TABLE IF NOT EXISTS orders (
        orderId INTEGER,
        status order_status
    );`, Flags{})
}

func TestReadmeExample(t *testing.T) {
	testOpenAPISpecToDDLSQL(t, "tests/testdata/readme_example.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        category_id INTEGER REFERENCES categories(id),
        name TEXT NOT NULL,
        photoUrls JSON NOT NULL,
        tag_id INTEGER REFERENCES tags(id)
	);

	CREATE TABLE IF NOT EXISTS categories (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);

	CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);`, Flags{})
}

func TestWriteInFolder(t *testing.T) {
	writeInFolder("test", Flags{outputFolderPath: "tests/output"})

	// Check if the folder / file was created
	_, err := os.ReadFile("tests/output/schemas.sql")
	if err != nil {
		t.Errorf("Failed to write SQL to file: %v", err)
	}

	// Clean up
	err = os.Remove("tests/output/schemas.sql")
	if err != nil {
		t.Errorf("Failed to remove file: %v", err)
	}
	err = os.Remove("tests/output")
	if err != nil {
		t.Errorf("Failed to remove folder: %v", err)
	}
}

func TestGetMultipleEntities(t *testing.T) {
	testOpenAPISpecToPathSQL(t, "tests/testdata/get_multiple_entities.yaml", `
	-- name: GetAllUsers :many
	SELECT * FROM users;
	`, Flags{})
}

func TestGetSingleEntity(t *testing.T) {
	testOpenAPISpecToPathSQL(t, "tests/testdata/get_single_entity.yaml", `
	-- name: getUser :one
	SELECT * FROM users WHERE id = $1;
	`, Flags{})
}

func TestPostEntityWithRef(t *testing.T) {
	testOpenAPISpecToPathSQL(t, "tests/testdata/post_entity_with_ref.yaml", `
	-- name: addUser :one
	INSERT INTO users (id, username) VALUES ($1, $2);
	`, Flags{})
}

func TestPostEntityWithoutRef(t *testing.T) {
	testOpenAPISpecToPathSQL(t, "tests/testdata/post_entity_without_ref.yaml", `
	-- name: addUser :one
	INSERT INTO users (username, firstName, userStatus) VALUES ($1, $2, $3, $4);
	`, Flags{})
}

// Add test for returning post path

func TestPutEntity(t *testing.T) {
	testOpenAPISpecToPathSQL(t, "tests/testdata/put_entity.yaml", `
	-- name: updateUser :one
	UPDATE users SET username = $1 WHERE id = $2;
	`, Flags{})
}

func TestDeleteEntity(t *testing.T) {
	testOpenAPISpecToPathSQL(t, "tests/testdata/delete_entity.yaml", `
	-- name: deleteUser :one
	DELETE FROM users WHERE id = $1;
	`, Flags{})
}

func TestFindFinalResourceWithRegex(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/companies/{companyId}/departments/{departmentId}/employees/{empId}", "employees"},
		{"/companies/{companyId}/departments/{departmentId}/", "departments"},
		{"/companies/{companyId}/", "companies"},
		{"/users/{userId}/posts/{postId}/comments/{commentId}", "comments"},
		{"/users/", "users"},
		{"/", ""},
		{"", ""},
		{"/users/{userId}/settings", "settings"},
		{"/products/{productId}/reviews", "reviews"},
		{"/stores/{storeId}/inventory/{itemId}/details", "details"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			result := FindFinalResourceWithRegex(test.path)
			if result != test.expected {
				t.Errorf("For path '%s', expected '%s', but got '%s'", test.path, test.expected, result)
			}
		})
	}
}
