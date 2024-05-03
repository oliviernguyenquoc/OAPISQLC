# Library to transform an OpenAPI specs to a postgreSQL schema (DDL)

The goal is to have CREATE statements than to Data Definition Language (DDL) based on OpenAPI specifications

Feature:

* You can add `x-database-entity: false` [extension](https://swagger.io/docs/specification/openapi-extensions/) in your OpenAPI specs to ignore a specific schema in the SQL schema generation

Limitations:

* Only OpenAPI 3.1 compatible
* Only take schemas under Component/Schemas OpenAPI specs
