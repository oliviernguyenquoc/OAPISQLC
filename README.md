![logo](./logo.svg)

# OpenAPI to PostgreSQL Schema (DDL)

This tool transforms [OpenAPI](https://github.com/OAI/OpenAPI-Specification) schemas
into a PostgreSQL schema, tagged for use with the [SQLC](https://sqlc.dev/) library.

It generates `CREATE TABLE` statements from an OpenAPI 3.1 document, or from a bare
JSON Schema.

Tables come from `components/schemas`. When the document has `paths`, the request and
response bodies of its operations are read as well. Not to generate queries, but to
work out which schemas the API actually returns. That distinction decides which
schemas become tables and which become foreign keys; see
[How references become SQL](#how-references-become-sql).

## ⚠️ Warning

🚧 This project is in progress 🚧

## Example

`MyOpenAPISpec.yaml`:

```yaml
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
);
```

`Pet.category` is one category, so it becomes a foreign key column. `Pet.tags` is many
tags, which a column on `pets` could not hold, so it becomes the `pets_tags` table.

## Usage

```sh
go build
./oapisqlc [flags] <path to OpenAPI or JSON Schema file>
```

| Flag                | Effect                                       |
| ------------------- | -------------------------------------------- |
| `-outputFolder DIR` | Write `DIR/schemas.sql` instead of printing  |
| `-deleteStatements` | Emit `DROP TABLE` before each `CREATE TABLE` |

The SQL goes to stdout; diagnostics go to stderr, so `./oapisqlc spec.yaml > schema.sql`
produces a clean file.

Exit codes are `0` on success, `2` for a usage problem, and `1` for anything else.

A schema that cannot be expressed as a table does not stop the run: it is reported on
stderr and the remaining schemas are still generated.

```text
skipped: schema Bad: dbSchema.Builder.Table: dbSchema.Builder.columns: no data type for property mystery
```

The chain between the schema name and the reason reads as a single logical stack trace,
so a report says where the failure happened as well as what it was.

A document with no schemas to build from is an error rather than an empty result.

### Reading a Bare JSON Schema

A JSON Schema is accepted directly, in either JSON or YAML, in the draft-07 or 2020-12
spelling. Its `definitions` (or `$defs`) become the tables, local `$ref`s are rewritten to point
at them, and the schema at the root is read as the payload the definitions are wrapped
in rather than as a table of its own.

A file that declares `openapi: 3.1.0` over what is otherwise a JSON Schema is read as
the JSON Schema it is. The key is not taken as evidence.

### In Go

This is `package main`, so the generator has no importable API. It is available as a
command only.

## 🚀 Feature Highlights

- 📊 **Dynamic data type mapping**: map API properties to PostgreSQL types (see
  [Data type mapping](#data-type-mapping))
- 🔒 **OpenAPI features**: `NOT NULL`, `DEFAULT`, `UNIQUE`, enums, and `CHECK`
  constraints from `minimum`/`maximum`, `minLength`/`maxLength` and `pattern`
- 🔑 **Primary keys**: resolved from the schema rather than assumed (see
  [How identity is resolved](#how-identity-is-resolved))
- ⏱️ **Auto timestamps**: `created_at`, `updated_at` and `deleted_at` become
  `TIMESTAMP NOT NULL DEFAULT NOW()`
- 🔗 **Relationship mapping**: foreign keys typed to match the column they point at,
  junction tables for to-many relationships, and `allOf` for table inheritance
- 📥 **JSON Schema input**: read a bare draft-07 or 2020-12 schema, not just OpenAPI
- 🚫 **Custom ignore tag**: exclude a schema with `x-database-entity: false`
- 🩹 **Partial output**: report a schema it cannot express and carry on with the rest

## How Identity Is Resolved

A table's primary key is the first of these the schema offers:

1. The property named by an `x-primary-key` extension.
2. A property named `id`.
3. A property named `sourcedId`.
4. A property named `<schemaName>Id`, such as `productId` on `Product`.

The key takes the type of the property it came from, so a natural string key stays
`TEXT` and an integer `id` becomes `BIGSERIAL`. Being listed under `required` is not
part of the test, because specifications routinely leave a surrogate `id` optional.

A schema offering none of these has no identity. It is an owned child rather than an
entity: nothing can reference it, and wherever it appears its value is held inline.

## How References Become SQL

What a `$ref` becomes depends on what it points at.

| The `$ref` points at                 | Result                                      |
| ------------------------------------ | ------------------------------------------- |
| A schema with identity, one of them  | Foreign key to the key column it names      |
| A schema with identity, many of them | Junction table keyed by both sides          |
| A reference wrapper                  | Foreign key to the entity the wrapper names |
| A schema with no identity            | `JSON` column holding the value inline      |

A **reference wrapper** is a schema with no data of its own: an identifier, a link,
and a discriminator naming what it points at. OneRoster's
`*GUIDRefDType` schemas are the canonical case:

```yaml
CourseGUIDRefDType:
  required: [href, sourcedId, type]
  properties:
    href: {type: string, format: uri}
    sourcedId: {type: string}
    type: {type: string, enum: [course]}
```

A wrapper is how one table points at another, not something rows are stored in, so it
becomes a foreign key rather than a table. The table that referenced it comes out as:

```sql
CREATE TABLE IF NOT EXISTS class_d_types (
    course_sourcedId TEXT REFERENCES course_d_types(sourcedId),
    metadata JSON,
    sourcedId TEXT NOT NULL PRIMARY KEY,
    terms JSON,
    title TEXT NOT NULL
);
```

A wrapper is recognised by where it sits in the document. A schema with identity that
the payload reaches only by way of another schema with identity points at an entity
rather than being one. Neither the wrapper's name nor its shape enters that test, and
neither does the discriminator, so the rule still applies where a specification leaves
its vocabulary open to extension.

The target entity comes from the discriminator's value, so `course` reaches
`CourseDType`. A wrapper's own name need not match what it points at:
`AcadSessionGUIDRefDType` points at `AcademicSessionDType`.

When the referenced entity is not in the document, which is usual for a single-endpoint
payload, the column keeps the value and drops the constraint. A to-many reference to an
absent entity becomes a `JSON` column, since there is no table to join and no room for
many values in one column.

## Motivation

This library allows you to kickstart your API development by auto-generating DDL
scripts.

![image](./schema.png)

By combining it with [SQLC](https://sqlc.dev/), you can generate the foundation of your
API.

Moreover, you can easily have a full automatic testing software by combining
contract-testing with [Microcks](https://microcks.io) and
[TestContainers](https://golang.testcontainers.org/).

Read more in this blog post (soon).

## Future Possible Features

- Usage of the `x-autoincrement` extension (like in openalchemy)

- [ ] **A Go API**

  - Expose the generator as an importable package rather than a command only.

- [ ] **Query generation**

  - Generate SQLC query files beside the DDL, driven by the tables rather than by the
    operations. See the note under Known limitations.

- [ ] **Metadata Utilization**

  - Use schema descriptions and other metadata to add comments to tables and columns in
    SQL.

- [ ] **Partitioning Support**

  - Implement table partitioning features if specified via OpenAPI extensions or
    conventions.

- [ ] **Custom Extensions Handling**

  - Recognize and process custom `x-` tags for advanced database features like partition
    keys and storage parameters.

- [ ] **Advanced SQL Options**

  - Generate SQL code that includes advanced table options such as tablespaces, storage
    parameters, and index options.

## Known Limitations

- Only OpenAPI 3.1 compatible
- Generates DDL only. No SQLC query files are produced
- A composite primary key can only be declared through `x-primary-key`; it is not
  inferred
- `oneOf` and `anyOf` are supported only where every branch agrees on one type

See note:

While OpenAPI provides powerful schema composition tools such as `anyOf` and `oneOf`,
these constructs do not have straightforward equivalents in SQL schema definitions due
to their inherently flexible and non-deterministic nature. Where every branch of a union declares the same type, that becomes the column type: a
choice between an enumerated string and a patterned string is still a string. Mixing
types leaves no single column type to take, and the tool reports and skips the schema
rather than guessing.

## Data Type Mapping

| Openapi Data Type                     | Openapi Data Format | PostgreSQL Data Types |
| ------------------------------------- | ------------------- | --------------------- |
| `integer`                             |                     | `INTEGER`             |
| `integer`                             | `int32`             | `INTEGER`             |
| `integer`                             | `int64`             | `BIGINT`              |
| `boolean`                             |                     | `BOOLEAN`             |
| `number`                              |                     | `NUMERIC`             |
| `number`                              | `float`             | `REAL`                |
| `number`                              | `double`            | `DOUBLE PRECISION`    |
| `string`                              |                     | `TEXT`                |
| `string`                              | `byte`              | `BYTEA`               |
| `string`                              | `binary`            | `BYTEA`               |
| `file`                                |                     | `BYTEA`               |
| `string`                              | `date`              | `DATE`                |
| `string`                              | `date-time`         | `TIMESTAMP`           |
| `string`                              | `uuid`              | `UUID`                |
| `string`                              | `enum`              | `TEXT`                |
| `array`                               |                     | `JSON`                |
| `object`                              |                     | `JSON`                |
| `\Model\User` (referenced definition) |                     | `TEXT`                |

A format with no entry of its own falls back to the bare type, so a `string` of format
`uri` is `TEXT` rather than an error.

An identifier PostgreSQL reserves gets quoted, so the column `primary` is emitted as
`"primary"`.

## Run Tests

`gotestsum --format testname`

## Contributing

We welcome contributions from the community.

## License

This project is licensed under the [MIT License](./LICENSE). See the LICENSE file for details.

## Contact

If you have any questions or suggestions, please open an issue on GitHub.

Inspired by [openapi-generator](https://github.com/OpenAPITools/openapi-generator) and
[openalchemy](https://openapi-sqlalchemy.readthedocs.io)
