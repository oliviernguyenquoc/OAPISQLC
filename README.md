# Library to transform an OpenAPI specs to a postgreSQL schema (DDL)

The goal is to have CREATE statements than to Data Definition Language (DDL) based on OpenAPI specifications

Feature:

* You can add `x-database-entity: false` [extension](https://swagger.io/docs/specification/openapi-extensions/) in your OpenAPI specs to ignore a specific schema in the SQL schema generation

Future possible features:

* Usage of `x-primary-key` and `x-autoincrement` extension (like in openalchemy)

Known limitations:

* Only OpenAPI 3.1 compatible
* Only take schemas under Component/Schemas OpenAPI specs

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


Inspired by [openapi-generator](https://github.com/OpenAPITools/openapi-generator) and [openalchemy](https://openapi-sqlalchemy.readthedocs.io)
