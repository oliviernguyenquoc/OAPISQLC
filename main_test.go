package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliviernguyenquoc/oapisqlc/dbSchema"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// testLogger discards what the code under test logs; the tests assert on the
// SQL it produces, not on its diagnostics.
func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// Comparison based on fingerprinting
func compareSQL(t *testing.T, expectedSQL, actualSQL string) {
	t.Helper()

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

func testOpenAPISpecToSQL(t *testing.T, filename, expectedSQL string, flags Flags) {
	t.Helper()

	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading OpenAPI spec: %v", err)
	}

	// Parse the OpenAPI specification
	doc, err := parseOpenAPISpec(t.Context(), testLogger(), apiSpec)
	if err != nil {
		t.Errorf("Error parsing OpenAPI spec: %v", err)
	}

	sql, skipped, err := fromComponentsToSQL(dbSchema.NewDialect(), doc, flags)
	if err != nil {
		t.Errorf("Error transforming OpenAPI to SQL: %v", err)
	}

	for _, problem := range skipped {
		t.Errorf("Unexpectedly skipped a schema: %v", problem)
	}
	compareSQL(t, expectedSQL, sql)
}

func testErrors(t *testing.T, filename string) {
	t.Helper()

	apiSpec, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading OpenAPI spec: %v", err)
	}

	// Parse the OpenAPI specification
	_, errParsing := parseOpenAPISpec(t.Context(), testLogger(), apiSpec)

	const wantSubstring = "infinite circular reference detected"

	if errParsing == nil || !strings.Contains(errParsing.Error(), wantSubstring) {
		t.Errorf("Expected error containing: %s, got: %v", wantSubstring, errParsing)
	}
}

func TestSimpleSchemaTransformation(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/simple_schema.yaml", `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		username TEXT
	);`, Flags{})
}

func TestTagManagement(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/tag_management.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
		name TEXT
	);`, Flags{})
}

func TestCustomExtensions(t *testing.T) {
	// No table should be created for ignored schemas.
	testOpenAPISpecToSQL(t, "tests/testdata/exclusion_extension.yaml", "", Flags{})
}

func TestComponentReferences(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/component_references.yaml", `
	CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        address_id BIGINT REFERENCES addresses(id)
    );
    CREATE TABLE IF NOT EXISTS addresses (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        street TEXT,
        city TEXT
    );`, Flags{})
}

func TestComponentReferencesWithDeleteStatements(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/component_references.yaml", `
	DROP TABLE IF EXISTS users CASCADE;
	DROP TABLE IF EXISTS addresses CASCADE;

	CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        address_id BIGINT REFERENCES addresses(id)
    );
    CREATE TABLE IF NOT EXISTS addresses (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        street TEXT,
        city TEXT
    );`, Flags{deleteStatements: true})
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
	);`, Flags{})
}

func TestConstraintsTranslation(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/constraints.yaml", `
    CREATE TABLE IF NOT EXISTS products (
        productId INTEGER NOT NULL PRIMARY KEY CHECK (productId >= 1.000000 AND productId <= 1000.000000),
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
	// Neither schema declares an identity, so there is nothing for a foreign
	// key to point at and each side holds the other inline. The cycle is no
	// longer a special case — it falls out of the schemas having no key.
	testOpenAPISpecToSQL(t, "tests/testdata/circular_references.yaml", `
	CREATE TABLE IF NOT EXISTS ones (
		thing JSON NOT NULL
	);
	CREATE TABLE IF NOT EXISTS twos (
		testThing JSON
	);`, Flags{})
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
    );`, Flags{})
}

func TestIdCreatedAtUpdatedAt(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/id_created_at_updated_at.yaml", `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		username TEXT
	);`, Flags{})
}

func TestArrayOfRef(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/array_of_ref.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS pets_tags (
		pet_id BIGINT NOT NULL REFERENCES pets(id),
		tag_id BIGINT NOT NULL REFERENCES tags(id),
		PRIMARY KEY (pet_id, tag_id)
	);

	CREATE TABLE IF NOT EXISTS tags (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);`, Flags{})
}

func TestDefaultValues(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/default_values.yaml", `
    CREATE TABLE IF NOT EXISTS users (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        username TEXT DEFAULT 'anonymous',
        signup_date DATE DEFAULT 2023-01-01
    );`, Flags{})
}

func TestUniqueConstraints(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/unique_constraints.yaml", `
    CREATE TABLE IF NOT EXISTS products (
        productId TEXT NOT NULL PRIMARY KEY UNIQUE,
        serialNumber TEXT UNIQUE,
        name TEXT
    );`, Flags{})
}

func TestEnumSupport(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/enum_definition.yaml", `
    CREATE TYPE order_status AS ENUM ('pending', 'approved', 'shipped', 'cancelled');

    CREATE TABLE IF NOT EXISTS orders (
        orderId INTEGER NOT NULL PRIMARY KEY,
        status order_status
    );`, Flags{})
}

func TestReadmeExample(t *testing.T) {
	testOpenAPISpecToSQL(t, "tests/testdata/readme_example.yaml", `
	CREATE TABLE IF NOT EXISTS pets (
        id BIGSERIAL NOT NULL PRIMARY KEY,
        category_id BIGINT REFERENCES categories(id),
        name TEXT NOT NULL,
        photoUrls JSON NOT NULL
	);

	CREATE TABLE IF NOT EXISTS pets_tags (
		pet_id BIGINT NOT NULL REFERENCES pets(id),
		tag_id BIGINT NOT NULL REFERENCES tags(id),
		PRIMARY KEY (pet_id, tag_id)
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
	if err := writeInFolder(
		io.Discard,
		"test",
		Flags{outputFolderPath: "tests/output"},
	); err != nil {
		t.Fatalf("Failed to write SQL to folder: %v", err)
	}

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

func TestJSONSchemaDefinitions(t *testing.T) {
	// A draft-07 JSON Schema: definitions become tables, and the root schema is
	// the envelope wrapping them rather than a table of its own. "homepage"
	// covers a format with no mapping of its own, "kind" a union of string
	// variants, and "owner" a reference carrying a sibling keyword.
	testOpenAPISpecToSQL(t, "tests/testdata/json_schema_definitions.json", `
	CREATE TABLE IF NOT EXISTS items (
		homepage TEXT,
		kind TEXT,
		owner JSON,
		sourcedId TEXT NOT NULL PRIMARY KEY
	);
	CREATE TABLE IF NOT EXISTS owners (
		name TEXT
	);`, Flags{})
}

func TestJSONSchemaWithoutDefinitions(t *testing.T) {
	// A schema that defines nothing but itself is the table, named after $id.
	testOpenAPISpecToSQL(t, "tests/testdata/json_schema_defs.yaml", `
	CREATE TABLE IF NOT EXISTS products (
		homepage TEXT,
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT NOT NULL
	);`, Flags{})
}

func TestJSONSchemaLabelledAsOpenAPI(t *testing.T) {
	// An "openapi" key does not make a document an OpenAPI document, and a
	// JSON Schema wearing one still has to be read as a JSON Schema.
	testOpenAPISpecToSQL(t, "tests/testdata/json_schema_labelled_openapi.yaml", `
	CREATE TABLE IF NOT EXISTS widgets (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		label TEXT NOT NULL
	);`, Flags{})
}

func TestNoSchemasIsAnError(t *testing.T) {
	// A document with nothing to build tables from should say so rather than
	// report success and write an empty file.
	apiSpec, err := os.ReadFile("tests/testdata/no_schemas.yaml")
	if err != nil {
		t.Fatalf("Error reading OpenAPI spec: %v", err)
	}

	doc, err := parseOpenAPISpec(t.Context(), testLogger(), apiSpec)
	if err != nil {
		t.Fatalf("Error parsing OpenAPI spec: %v", err)
	}

	if _, _, err := fromComponentsToSQL(
		dbSchema.NewDialect(),
		doc,
		Flags{},
	); !errors.Is(
		err,
		errNoSchemas,
	) {
		t.Errorf("Expected %v, got: %v", errNoSchemas, err)
	}
}

func TestUnbuildableSchemaIsSkippedNotFatal(t *testing.T) {
	// One schema that cannot be expressed as a table must not cost the run the
	// schemas either side of it.
	apiSpec, err := os.ReadFile("tests/testdata/unbuildable_schema.yaml")
	if err != nil {
		t.Fatalf("Error reading OpenAPI spec: %v", err)
	}

	doc, err := parseOpenAPISpec(t.Context(), testLogger(), apiSpec)
	if err != nil {
		t.Fatalf("Error parsing OpenAPI spec: %v", err)
	}

	sql, skipped, err := fromComponentsToSQL(dbSchema.NewDialect(), doc, Flags{})
	if err != nil {
		t.Fatalf("Expected the run to continue, got: %v", err)
	}

	// Bad fails while its columns are built; Unmappable only fails when the
	// table is rendered. Both cost one table and nothing more.
	if len(skipped) != 2 {
		t.Fatalf("Expected 2 skipped schemas, got %d: %v", len(skipped), skipped)
	}

	if got := dbSchema.ErrorCode(skipped[0]); got != dbSchema.EINVALID {
		t.Errorf("Expected code %s, got %s", dbSchema.EINVALID, got)
	}

	if want := "no data type for property mystery"; dbSchema.ErrorMessage(skipped[0]) != want {
		t.Errorf("Expected message %q, got %q", want, dbSchema.ErrorMessage(skipped[0]))
	}

	if !strings.Contains(skipped[0].Error(), "schema Bad") {
		t.Errorf("Expected the skipped schema to be named, got: %v", skipped[0])
	}

	compareSQL(t, `
	CREATE TABLE IF NOT EXISTS goods (
		id BIGSERIAL NOT NULL PRIMARY KEY,
		name TEXT
	);
	CREATE TABLE IF NOT EXISTS also_goods (
		label TEXT
	);`, sql)
}

func TestRunWritesSQLToStdout(t *testing.T) {
	// The whole point of splitting run out of main: drive it with buffers.
	var stdout, stderr bytes.Buffer

	args := []string{"oapisqlc", "tests/testdata/simple_schema.yaml"}
	if err := run(t.Context(), args, &stdout, &stderr); err != nil {
		t.Fatalf("run returned: %v", err)
	}

	if !strings.Contains(stdout.String(), "CREATE TABLE IF NOT EXISTS users") {
		t.Errorf("Expected the users table on stdout, got: %s", stdout.String())
	}
}

func TestRunReportsSkippedSchemasOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer

	args := []string{"oapisqlc", "tests/testdata/unbuildable_schema.yaml"}
	if err := run(t.Context(), args, &stdout, &stderr); err != nil {
		t.Fatalf("Expected the run to finish, got: %v", err)
	}

	if !strings.Contains(stderr.String(), "schema Bad") {
		t.Errorf("Expected the skipped schema named on stderr, got: %s", stderr.String())
	}

	if !strings.Contains(stdout.String(), "CREATE TABLE IF NOT EXISTS goods") {
		t.Errorf("Expected the buildable tables on stdout, got: %s", stdout.String())
	}
}

func TestRunWithoutSpecPath(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run(
		t.Context(),
		[]string{"oapisqlc"},
		&stdout,
		&stderr,
	); !errors.Is(
		err,
		errNoSpecPath,
	) {
		t.Errorf("Expected %v, got: %v", errNoSpecPath, err)
	}

	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("Expected usage on stderr, got: %s", stderr.String())
	}
}

func TestRunHelpIsNotAFailure(t *testing.T) {
	// main turns flag.ErrHelp into exit 0, so run has to hand it back
	// unwrapped for errors.Is to see it.
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), []string{"oapisqlc", "-h"}, &stdout, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("Expected flag.ErrHelp, got: %v", err)
	}
}

func TestRunWritesSQLToFolder(t *testing.T) {
	var stdout, stderr bytes.Buffer

	folder := t.TempDir()
	args := []string{
		"oapisqlc",
		"-outputFolder", folder,
		"-deleteStatements",
		"tests/testdata/simple_schema.yaml",
	}

	if err := run(t.Context(), args, &stdout, &stderr); err != nil {
		t.Fatalf("run returned: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(folder, "schemas.sql"))
	if err != nil {
		t.Fatalf("Error reading generated SQL: %v", err)
	}

	// -deleteStatements is read from the flag set, which main never parsed
	// before run existed.
	if !strings.Contains(string(written), "DROP TABLE IF EXISTS users") {
		t.Errorf("Expected a drop statement, got: %s", written)
	}
}

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		want       int
		wantStderr bool
	}{
		{name: "success", err: nil, want: 0},
		{name: "help", err: flag.ErrHelp, want: 0},
		{name: "wrapped help", err: fmt.Errorf("%w: %w", errUsage, flag.ErrHelp), want: 0},
		{name: "no spec path", err: fmt.Errorf("%w: %w", errUsage, errNoSpecPath), want: 2},
		{name: "anything else", err: errors.New("boom"), want: 1, wantStderr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			if got := exitCode(test.err, &stderr); got != test.want {
				t.Errorf("exitCode(%v) = %d, want %d", test.err, got, test.want)
			}

			// A reason the flag set already printed must not be printed twice.
			if gotStderr := stderr.Len() > 0; gotStderr != test.wantStderr {
				t.Errorf("wrote to stderr = %v, want %v (%q)",
					gotStderr, test.wantStderr, stderr.String())
			}
		})
	}
}

func TestReferenceWrappersBecomeForeignKeys(t *testing.T) {
	// The shape OneRoster uses: an envelope owning entities, which reach each
	// other only through wrapper schemas carrying {href, sourcedId, type}.
	// Every line below is a decision the wrapper handling makes.
	testOpenAPISpecToSQL(t, "tests/testdata/reference_wrappers.json", `
	CREATE TABLE IF NOT EXISTS class_d_types (
		course_sourcedId TEXT REFERENCES course_d_types(sourcedId),
		metadata JSON,
		sourcedId TEXT NOT NULL PRIMARY KEY,
		terms JSON,
		title TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS course_d_types (
		sourcedId TEXT NOT NULL PRIMARY KEY,
		title TEXT
	);

	CREATE TABLE IF NOT EXISTS metadata_d_types (
		note TEXT
	);`, Flags{})
}

func TestReferenceWrappersAreNotTables(t *testing.T) {
	apiSpec, err := os.ReadFile("tests/testdata/reference_wrappers.json")
	if err != nil {
		t.Fatalf("Error reading spec: %v", err)
	}

	doc, err := parseOpenAPISpec(t.Context(), testLogger(), apiSpec)
	if err != nil {
		t.Fatalf("Error parsing spec: %v", err)
	}

	sql, skipped, err := fromComponentsToSQL(dbSchema.NewDialect(), doc, Flags{})
	if err != nil {
		t.Fatalf("Error generating SQL: %v", err)
	}

	for _, problem := range skipped {
		t.Errorf("Unexpectedly skipped: %v", problem)
	}

	// A wrapper is how one table points at another, not a thing rows are
	// stored in.
	for _, wrapper := range []string{"course_g_u_i_d_ref_d_types", "acad_session_g_u_i_d_ref_d_types"} {
		if strings.Contains(sql, wrapper) {
			t.Errorf("%s should not be a table:\n%s", wrapper, sql)
		}
	}

	// And the envelope is not one either.
	if strings.Contains(sql, "roster_d_types") {
		t.Errorf("the envelope should not be a table:\n%s", sql)
	}
}
