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

func testErrors(t *testing.T, filename string) {
	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading OpenAPI spec: %v", err)
	}

	_, errParsing := OpenAPISpecToSQL(apiSpec)

	expectedErrorMsg := "cannot create v3 model from document: 1 errors reported"

	if errParsing == nil || errParsing.Error() != expectedErrorMsg {
		t.Errorf("Expected error message: %s, got: %v", expectedErrorMsg, errParsing)
	}
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

func TestDataTypes(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/data_types_conversion.yaml", `
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
	);`)
}

func TestConstraintsTranslation(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/constraints.yaml", `
    CREATE TABLE IF NOT EXISTS products (
        productId INTEGER CHECK (productId >= 1.000000 AND productId <= 1000.000000),
        productName TEXT CHECK (char_length(productName) >= 1 AND char_length(productName) <= 100),
        productPrice NUMERIC CHECK (productPrice >= 0.010000 AND productPrice <= 9999.990000),
        productCode TEXT CHECK (productCode ~ '^[A-Z0-9]{10}$'),
        releaseDate DATE DEFAULT 2023-01-01
    );`)
}

func TestCircularReferencesParsingError(t *testing.T) {

	// Should return an error if there are circular references, detected during parsing.
	testErrors(t, "tests/testdata/circular_references_parsing_error.yaml")
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

func TestDefaultValues(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/default_values.yaml", `
    CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        username TEXT DEFAULT 'anonymous',
        signup_date DATE DEFAULT 2023-01-01
    );`)
}

func TestUniqueConstraints(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/unique_constraints.yaml", `
    CREATE TABLE IF NOT EXISTS products (
        productId TEXT UNIQUE,
        serialNumber TEXT UNIQUE,
        name TEXT
    );`)
}

func TestEnumSupport(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/enum_definition.yaml", `
    CREATE TYPE order_status AS ENUM ('pending', 'approved', 'shipped', 'cancelled');

    CREATE TABLE IF NOT EXISTS orders (
        orderId INTEGER,
        status order_status
    );`)
}

func TestReadmeExample(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/readme_example.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        category_id INTEGER,
        name TEXT NOT NULL,
        photoUrls JSON NOT NULL,
        tag_id INTEGER,
        FOREIGN KEY (category_id) REFERENCES categories(id),
        FOREIGN KEY (tag_id) REFERENCES tags(id)
	);

	CREATE TABLE IF NOT EXISTS categories (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);

	CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);`)
}
