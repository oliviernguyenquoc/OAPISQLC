# OpenAPI to PostgreSQL Schema (DDL) Library

This library transforms [OpenAPI](https://github.com/OAI/OpenAPI-Specification) schemas into a PostgreSQL schema. The goal is to generate CREATE statements based on OpenAPI specifications. 

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
);
```

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

## 🚀 Feature Highlights

- 📊 Dynamic Data Type Mapping - Accurately map API properties to PostgreSQL data types (See details in [Openapi Data Type to MySQL Data Type mapping](#openapi-data-type-to-mysql-data-type-mapping) section)
- 🔒 Handle multiple OpenAPI features:
  - Enforce NOT NULL and support DEFAULT values directly from OpenAPI
  - Unique values
  - Enums
- 🔑 Auto Primary Key Setup - Set primary keys on `id` columns.
- ⏱️ Auto Timestamps - Set created_at and updated_at fields as DATES with automatic updates.
- 🔗 Robust Relationship Mapping - Establish and link foreign key relationships as defined in API specs (with `allOf` constructs for sophisticated table inheritance)
- 🚫 Custom Ignore Tag - Optionally exclude schemas with "x-database-entity" tag from database creation.

## Motivation

This library allows you to kickstart your API development by auto-generating DDL scripts. 

By combining it with [SQLC](https://sqlc.dev/), you can generate the foundation of your API.

Moreover, you can easily have a full automatic testing software by combining contract-testing with [Microcks](https://microcks.io) and [TestContainers](https://golang.testcontainers.org/).

Read more in this [blog post]().

## Future possible features:

- Usage of `x-primary-key` and `x-autoincrement` extension (like in openalchemy)

- [ ] **Metadata Utilization**
  - Use schema descriptions and other metadata to add comments to tables and columns in SQL.

- [ ] **Partitioning Support**
  - Implement table partitioning features if specified via OpenAPI extensions or conventions.

- [ ] **Custom Extensions Handling**
  - Recognize and process custom `x-` tags for advanced database features like partition keys and storage parameters.

- [ ] **Advanced SQL Options**
  - Generate SQL code that includes advanced table options such as tablespaces, storage parameters, and index options.

## Known limitations:

* Only OpenAPI 3.1 compatible
* Only compatible with YAML input
* Only take schemas under Component/Schemas OpenAPI specs
* Does not support foreign keys other than "id" columns
* `anyOf` and `oneOf` is not supported (see note)

See note:

While OpenAPI provides powerful schema composition tools such as `anyOf` and `oneOf`, these constructs do not have straightforward equivalents in SQL schema definitions due to their inherently flexible and non-deterministic nature. To maintain clarity and ensure the integrity of database structures, this tool does not support the direct transformation of these constructs into SQL. This decision helps avoid ambiguity in table definitions and keeps the transformation process simpler and more predictable.


## Openapi Data Type to MySQL Data Type mapping

| Openapi Data Type | Openapi Data Format | PostgreSQL Data Types |
| ----------------- | ------------------- | --------------------- |
| `integer`         |                     | `INTEGER`             |
| `integer`         | `int32`             | `INTEGER`             |
| `integer`         | `int64`             | `BIGINT`              |
| `boolean`         |                     | `BOOLEAN`             |
| `number`          |                     | `NUMERIC`             |
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

## Run tests

`gotestsum --format testname`

## Contributing

We welcome contributions from the community.

## License

This project is licensed under the [MIT License](). See the LICENSE file for details.

## Contact

If you have any questions or suggestions, please open an issue on GitHub.


Inspired by [openapi-generator](https://github.com/OpenAPITools/openapi-generator) and [openalchemy](https://openapi-sqlalchemy.readthedocs.io)
