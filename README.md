# OpenAPI to PostgreSQL Schema (DDL) Library

This library transforms [OpenAPI](https://github.com/OAI/OpenAPI-Specification) specifications into a PostgreSQL schema (Data Definition Language - DDL). The goal is to generate CREATE statements based on OpenAPI specifications. 

It **only takes Components/Schemas** section of OpenAPI Spec.

## Example

MyOpenAPISpec.YAML:
```YAML
openapi: 3.1.0
info:
  title: Complex Properties Schema Test
  version: 1.0.0
components:
  schemas:
    Pet:
      type: object
      required:
        - name
        - photoUrls
      properties:
        id:
          type: integer
          format: int64
        category:
          $ref: '#/components/schemas/Category'
        name:
          type: string
        photoUrls:
          type: array
          items:
            type: string
        tags:
          type: array
          items:
            $ref: '#/components/schemas/Tag'
    Category:
      type: object
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
    Tag:
      type: object
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
```

It returns:
```sql

```

## Features

- Transform Components/Schemas into PostgreSQL create statements
- Add `x-database-entity: false` [extension](https://swagger.io/docs/specification/openapi-extensions/) in your OpenAPI specs to ignore a specific schema in the SQL schema generation.
- AllOf decleration are handled
- Special cases handling for Id, updated_at, created_at: Detect these fields
    - Associate "BIGSERIAL NOT NULL PRIMARY KEY" for 'id' field and 
    - Associate "TIMESTAMP NOT NULL DEFAULT NOW()" for created_at and updated_at

## Usage

You can use the library either in CLI or in Go.

### In CLI

Run: `go build`

Then: `./oas2pgschema YOUR_OPENAPI.yaml`

### In Go:

```go
import (
    "os"
    "https://github.com/oliviernguyenquoc/oas2pgschema"
)

openAPISpec, err := os.ReadFile(filePath)
if err != nil {
    fmt.Printf("Failed to read OpenAPI spec: %v\n", err)
    os.Exit(1)
}

sqlStatement, err := oas2pgschema.OpenAPISpecToSQL(openAPISpec)
if err != nil {
    fmt.Printf("Failed to transform OpenAPI spec to SQL: %v\n", err)
    os.Exit(1)
}
```

## Motivation

This library allows you to kickstart your API development by auto-generating DDL scripts. 

By combining it with [SQLC](https://sqlc.dev/), you can generate the foundation of your API.

Moreover, you can easily have a full automatic testing software by combining contract-testing with [Microcks](https://microcks.io) and [TestContainers](https://golang.testcontainers.org/).

Read more in this [blog post]().

## Future possible features:

* Usage of `x-primary-key` and `x-autoincrement` extension (like in openalchemy)

## Known limitations:

* Only OpenAPI 3.1 compatible
* Only compatible with YAML input
* Only take schemas under Component/Schemas OpenAPI specs
* Does not support foreign keys other than "id" columns

## Need to be done


## Openapi Data Type to MySQL Data Type mapping

| Openapi Data Type | Openapi Data Format | PostgreSQL Data Types |
| ----------------- | ------------------- | --------------------- |
| `integer`         | `int32`             | `INTEGER`             |
| `integer`         | `int64`             | `BIGINT`              |
| `boolean`         |                     | `BOOLEAN`             |
| `number`          | `float`             | `REAL`                |
| `number`          | `double`            | `DOUBLE PRECISION`    |
| `string`          |                     | `TEXT`                |
| `string`          | `byte`              | `BYTEA`               |
| `string`          | `binary`            | `BYTEA`               |
| `file`            |                     | `BYTEA`               |
| `string`          | `date`              | `DATE`                |
| `string`          | `date-time`         | `TIMESTAMP`           |
| `string`          | `enum`              | `TEXT`                |
| `array`           |                     | `JSON`                |
| `object`          |                     | `JSON`                |
| `\Model\User` (referenced definition) | | `TEXT`                |

## Postgresql specifications supported

Postgres DDL specifications: https://www.postgresql.org/docs/current/sql-createtable.html

```sql
CREATE TABLE [ IF NOT EXISTS ] table_name ( [
  { column_name data_type [ STORAGE { PLAIN | EXTERNAL | EXTENDED | MAIN | DEFAULT } ] [ COMPRESSION compression_method ] [ COLLATE collation ] [ column_constraint [ ... ] ]
    | table_constraint
    | LIKE source_table [ like_option ... ] }
    [, ... ]
] )
[ INHERITS ( parent_table [, ... ] ) ]
[ PARTITION BY { RANGE | LIST | HASH } ( { column_name | ( expression ) } [ COLLATE collation ] [ opclass ] [, ... ] ) ]
[ USING method ]
[ WITH ( storage_parameter [= value] [, ... ] ) | WITHOUT OIDS ]
[ ON COMMIT { PRESERVE ROWS | DELETE ROWS | DROP } ]
[ TABLESPACE tablespace_name ]
```

where column_constraint is:

```sql
[ CONSTRAINT constraint_name ]
{ NOT NULL |
  NULL |
  CHECK ( expression ) [ NO INHERIT ] |
  DEFAULT default_expr |
  GENERATED ALWAYS AS ( generation_expr ) STORED |
  GENERATED { ALWAYS | BY DEFAULT } AS IDENTITY [ ( sequence_options ) ] |
  UNIQUE [ NULLS [ NOT ] DISTINCT ] index_parameters |
  PRIMARY KEY index_parameters |
  REFERENCES reftable [ ( refcolumn ) ] [ MATCH FULL | MATCH PARTIAL | MATCH SIMPLE ]
    [ ON DELETE referential_action ] [ ON UPDATE referential_action ] }
[ DEFERRABLE | NOT DEFERRABLE ] [ INITIALLY DEFERRED | INITIALLY IMMEDIATE ]
```

and table_constraint is:

```sql
[ CONSTRAINT constraint_name ]
{ CHECK ( expression ) [ NO INHERIT ] |
  UNIQUE [ NULLS [ NOT ] DISTINCT ] ( column_name [, ... ] ) index_parameters |
  PRIMARY KEY ( column_name [, ... ] ) index_parameters |
  EXCLUDE [ USING index_method ] ( exclude_element WITH operator [, ... ] ) index_parameters [ WHERE ( predicate ) ] |
  FOREIGN KEY ( column_name [, ... ] ) REFERENCES reftable [ ( refcolumn [, ... ] ) ]
    [ MATCH FULL | MATCH PARTIAL | MATCH SIMPLE ] [ ON DELETE referential_action ] [ ON UPDATE referential_action ] }
[ DEFERRABLE | NOT DEFERRABLE ] [ INITIALLY DEFERRED | INITIALLY IMMEDIATE ]
```

and like_option is:

```sql
{ INCLUDING | EXCLUDING } { COMMENTS | COMPRESSION | CONSTRAINTS | DEFAULTS | GENERATED | IDENTITY | INDEXES | STATISTICS | STORAGE | ALL }
```

index_parameters in UNIQUE, PRIMARY KEY, and EXCLUDE constraints are:

```sql
[ INCLUDE ( column_name [, ... ] ) ]
[ WITH ( storage_parameter [= value] [, ... ] ) ]
[ USING INDEX TABLESPACE tablespace_name ]
```

exclude_element in an EXCLUDE constraint is:

```sql
{ column_name | ( expression ) } [ COLLATE collation ] [ opclass [ ( opclass_parameter = value [, ... ] ) ] ] [ ASC | DESC ] [ NULLS { FIRST | LAST } ]
```

referential_action in a FOREIGN KEY/REFERENCES constraint is:

```sql
{ NO ACTION | RESTRICT | CASCADE | SET NULL [ ( column_name [, ... ] ) ] | SET DEFAULT [ ( column_name [, ... ] ) ] }
```


`CREATE TABLE table_name OF type_name`, `CREATE TABLE table_name PARTITION OF parent_table` OR `CREATE TABLE table_name AS` are not supported

## Contributing

We welcome contributions from the community.

## License

This project is licensed under the [MIT License](). See the LICENSE file for details.

## Contact

If you have any questions or suggestions, please open an issue on GitHub.


Inspired by [openapi-generator](https://github.com/OpenAPITools/openapi-generator) and [openalchemy](https://openapi-sqlalchemy.readthedocs.io)
