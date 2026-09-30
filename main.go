// Command oapisqlc generates PostgreSQL DDL from the schemas of an OpenAPI
// document or a bare JSON Schema.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/oliviernguyenquoc/oapisqlc/dbSchema"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Exit codes. main is the only place that turns an error into one of these.
const (
	exitOK    = 0
	exitFail  = 1
	exitUsage = 2
)

var (
	// errNoSpecPath reports a command line with no file to read.
	errNoSpecPath = errors.New("no spec file given")

	// errUsage marks a command line the flag set has already complained about
	// on stderr, so main can set an exit code without repeating it.
	errUsage = errors.New("bad usage")
)

// Flags carries the command line options through the generation steps.
type Flags struct {
	deleteStatements bool
	outputFolderPath string
}

// parseOpenAPISpec parses an OpenAPI document, or a bare JSON Schema, into the
// v3 model. It reports what it found through logger rather than to a package
// level default, so a caller decides where that goes.
func parseOpenAPISpec(
	ctx context.Context,
	logger *slog.Logger,
	openAPISpec []byte,
) (*v3.Document, error) {
	// libopenapi reads OpenAPI documents only, so a bare JSON Schema has to be
	// given an OpenAPI shape before it can be parsed at all.
	openAPISpec, err := normalizeSpec(openAPISpec)
	if err != nil {
		return nil, err
	}

	// create a new document from specification bytes
	document, err := libopenapi.NewDocument(openAPISpec)
	if err != nil {
		return nil, fmt.Errorf("cannot create document from OpenAPI spec: %w", err)
	}

	// because we know this is a v3 spec, we can build a ready to go model from it.
	v3Model, err := document.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("cannot create v3 model from document: %w", err)
	}

	// Get a count of the number of paths and schemas.
	var nbPaths int
	if v3Model.Model.Paths == nil {
		nbPaths = 0
	} else {
		nbPaths = v3Model.Model.Paths.PathItems.Len()
	}

	var nbSchemas int
	if v3Model.Model.Components == nil || v3Model.Model.Components.Schemas == nil {
		nbSchemas = 0
	} else {
		nbSchemas = v3Model.Model.Components.Schemas.Len()
	}

	// Print the number of paths and schemas in the document
	logger.InfoContext(ctx, "parsed document", "paths", nbPaths, "schemas", nbSchemas)

	return &v3Model.Model, nil
}

// fromComponentsToSQL generates the DDL for a document's schemas.
//
// A schema that cannot be expressed as a table is returned in skipped and left
// out of the SQL; the run continues with the schemas that remain. Only a
// failure that invalidates the whole document is returned as err.
func fromComponentsToSQL(
	dialect *dbSchema.Dialect,
	doc *v3.Document,
	flags Flags,
) (sql string, skipped []error, err error) {
	if doc == nil || doc.Components == nil ||
		doc.Components.Schemas == nil || doc.Components.Schemas.Len() == 0 {
		return "", nil, errNoSchemas
	}

	schemas := doc.Components.Schemas

	classification := dbSchema.Classify(schemas, payloadRoots(doc))
	tableDefinitions, skipped := dbSchema.NewBuilder(classification).Tables(schemas)

	query, unrenderable, err := renderDDL(dialect, tableDefinitions, flags.deleteStatements)
	skipped = append(skipped, unrenderable...)

	if err != nil {
		return "", skipped, err
	}

	normalizedQuery, err := pg_query.Normalize(query)
	if err != nil {
		return "", skipped, fmt.Errorf("cannot normalize query: %w", err)
	}

	return normalizedQuery, skipped, nil
}

// skippable reports whether a failure concerns a single schema rather than the
// whole document, and so should cost that schema and nothing else.
func skippable(err error) bool {
	return dbSchema.ErrorCode(err) == dbSchema.EINVALID
}

// payloadRoots names the schemas a document actually returns, which is where
// classification starts: what a payload carries directly it owns, and what it
// reaches only through one of those it merely points at.
//
// For a document converted from a bare JSON Schema the root is the schema the
// conversion put at the top. For an OpenAPI document the roots are the schemas
// its operations send and return. A document with neither — a components-only
// spec — has no roots, and says nothing about what it owns.
func payloadRoots(doc *v3.Document) []string {
	if doc.Paths == nil || doc.Paths.PathItems == nil {
		return nil
	}

	var roots []string

	for path := doc.Paths.PathItems.First(); path != nil; path = path.Next() {
		for _, operation := range path.Value().GetOperations().FromOldest() {
			roots = append(roots, operationRoots(operation)...)
		}
	}

	return roots
}

// operationRoots names the schemas one operation sends or returns.
func operationRoots(operation *v3.Operation) []string {
	var roots []string

	if operation.RequestBody != nil {
		roots = append(roots, mediaTypeRoots(operation.RequestBody.Content)...)
	}

	if operation.Responses == nil || operation.Responses.Codes == nil {
		return roots
	}

	for code := operation.Responses.Codes.First(); code != nil; code = code.Next() {
		roots = append(roots, mediaTypeRoots(code.Value().Content)...)
	}

	return roots
}

// mediaTypeRoots names the schemas a set of media types carries, following an
// array to the schema of its items.
func mediaTypeRoots(content *orderedmap.Map[string, *v3.MediaType]) []string {
	if content == nil {
		return nil
	}

	var roots []string

	for media := content.First(); media != nil; media = media.Next() {
		proxy := media.Value().Schema
		if proxy == nil {
			continue
		}

		if name, ok := schemaRefName(proxy); ok {
			roots = append(roots, name)

			continue
		}

		// An inline body — an envelope, say — carries its payload in the
		// schemas it points at rather than being one itself.
		roots = append(roots, dbSchema.ReferencedSchemas(proxy.Schema())...)
	}

	return roots
}

// schemaRefName reads the component name out of a local schema reference.
func schemaRefName(proxy *base.SchemaProxy) (string, bool) {
	const prefix = "#/components/schemas/"

	name, ok := strings.CutPrefix(proxy.GetReference(), prefix)

	return name, ok && name != ""
}

// renderDDL renders the create statement for each table, preceded by a drop
// statement for every table it rendered when deleteStatements is set.
//
// A table whose columns have no Postgres equivalent is left out and returned in
// skipped, on the same terms as a schema that never became a table at all.
func renderDDL(
	dialect *dbSchema.Dialect,
	tables []dbSchema.Table,
	deleteStatements bool,
) (sql string, skipped []error, err error) {
	var creates strings.Builder

	rendered := make([]dbSchema.Table, 0, len(tables))

	for i := range tables {
		table := &tables[i]

		statement, err := table.CreateSQLStatement(dialect)
		if err != nil {
			if skippable(err) {
				skipped = append(skipped, fmt.Errorf("table %s: %w", table.Name, err))

				continue
			}

			return "", skipped, fmt.Errorf("cannot build table %s: %w", table.Name, err)
		}

		rendered = append(rendered, *table)

		creates.WriteString("\n\n")
		creates.WriteString(statement)
	}

	var query strings.Builder

	// Add delete statements at the beginning of the output file
	if deleteStatements {
		for i := range rendered {
			query.WriteString(rendered[i].DeleteSQLStatement())
		}
	}

	query.WriteString(creates.String())

	return query.String(), skipped, nil
}

func writeInFolder(stdout io.Writer, sqlStatement string, flags Flags) error {
	// Create folder if not exist
	if err := os.MkdirAll(flags.outputFolderPath, 0o755); err != nil {
		return fmt.Errorf("cannot create output folder %s: %w", flags.outputFolderPath, err)
	}

	schemaFile := filepath.Join(flags.outputFolderPath, "schemas.sql")
	if err := os.WriteFile(schemaFile, []byte(sqlStatement), 0o600); err != nil {
		return fmt.Errorf("cannot write %s: %w", schemaFile, err)
	}
	_, _ = fmt.Fprintf(stdout, "SQL written in folder %s\n", flags.outputFolderPath)

	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGQUIT, syscall.SIGTERM,
	)

	code := exitCode(run(ctx, os.Args, os.Stdout, os.Stderr), os.Stderr)

	// Release the signal goroutine before leaving. A deferred stop would not
	// run, because os.Exit does not unwind.
	stop()

	os.Exit(code)
}

// exitCode reports the status main should exit with, after writing whatever
// the user has not already been told.
func exitCode(err error, stderr io.Writer) int {
	switch {
	// -h is a request, not a failure.
	case err == nil, errors.Is(err, flag.ErrHelp):
		return exitOK

	// The flag set has already written the reason and the usage to stderr.
	case errors.Is(err, errUsage):
		return exitUsage

	default:
		_, _ = fmt.Fprintf(stderr, "oapisqlc: %v\n", err)

		return exitFail
	}
}

// run is intentionally separated from main to improve testability. Please preserve this comment.
//
// It takes the whole argument slice including the program name, and writes
// everything it produces to stdout and everything it reports to stderr, so a
// test can drive it with buffers. It never calls os.Exit: main owns exit codes.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags, specPath, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))

	// load an OpenAPI 3.1 specification from bytes
	openAPISpec, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", specPath, err)
	}

	// Parse the OpenAPI specification
	doc, err := parseOpenAPISpec(ctx, logger, openAPISpec)
	if err != nil {
		return err
	}

	// Generate SQL statement based on the OpenAPI spec
	ddl, skipped, err := fromComponentsToSQL(dbSchema.NewDialect(), doc, flags)
	if err != nil {
		return err
	}

	// A schema that could not become a table costs that schema alone, but the
	// user still has to be told which ones went missing.
	for _, problem := range skipped {
		_, _ = fmt.Fprintf(stderr, "skipped: %v\n", problem)
	}

	if flags.outputFolderPath != "" {
		return writeInFolder(stdout, ddl, flags)
	}

	_, _ = fmt.Fprintln(stdout, "Generated SQL Statement:", ddl)

	return nil
}

// parseArgs reads the command line into Flags and the path of the spec to
// read. It returns flag.ErrHelp unwrapped when the user asked for usage.
func parseArgs(args []string, stderr io.Writer) (Flags, string, error) {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)

	deleteStatements := fs.Bool(
		"deleteStatements", false, "Add delete statements to SQL output")
	outputFolderPath := fs.String(
		"outputFolder", "", "Path to output folder")

	if err := fs.Parse(args[1:]); err != nil {
		return Flags{}, "", fmt.Errorf("%w: %w", errUsage, err)
	}

	if fs.NArg() < 1 {
		_, _ = fmt.Fprintf(stderr,
			"usage: %s [flags] <path to OpenAPI or JSON Schema file>\n", args[0])
		fs.PrintDefaults()

		return Flags{}, "", fmt.Errorf("%w: %w", errUsage, errNoSpecPath)
	}

	flags := Flags{
		deleteStatements: *deleteStatements,
		outputFolderPath: *outputFolderPath,
	}

	return flags, fs.Arg(0), nil
}
