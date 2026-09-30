package dbSchema

import (
	"regexp"
	"strings"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// nonIdentifier matches everything that does not survive into a normalised
// name, so that "academicSession" and "AcademicSessionDType" can be compared.
var nonIdentifier = regexp.MustCompile(`[^a-z0-9]`)

// step is a schema still to visit, and whether the path that reached it
// crossed a keyed schema on the way.
type step struct {
	name    string
	crossed bool
}

// Classification separates the schemas a document owns from the ones it only
// points at.
//
// The distinction is a property of the document graph, not of any schema on its
// own: a payload carries its entities directly, and reaches a reference only by
// way of an entity it already carries. That holds however the reference spells
// its discriminator, which a test of the schema's own shape does not.
type Classification struct {
	entities   map[string]*highbase.Schema
	references map[string]*highbase.Schema
	targets    map[string]string
}

// Relation is what a property pointing at another schema becomes.
//
// A zero Table with a declared Key means the reference is real but its target
// is not in this document: the column carries the value without a constraint.
// Embed means the target has no identity at all, so its value belongs inline.
type Relation struct {
	Table string // the table a foreign key points at
	Key   Key    // the column in that table, and the type the key column takes
	Embed bool   // the target has no identity; hold its value inline
}

// Classify partitions schemas into entities and references, starting from the
// schemas named in roots — the payloads the document actually returns.
//
// An entity is a keyed schema reachable from a root without passing through
// another keyed schema. A keyed schema reachable only through one is a
// reference to an entity rather than an entity itself. A schema with no key is
// neither: it is an owned value object, embedded wherever it appears.
//
// With no roots nothing can be reached, so every keyed schema is taken to be an
// entity. That is the right answer for a components-only document, which says
// nothing about what it owns.
func Classify(
	schemas *orderedmap.Map[string, *highbase.SchemaProxy],
	roots []string,
) Classification {
	classification := Classification{
		entities:   map[string]*highbase.Schema{},
		references: map[string]*highbase.Schema{},
		targets:    map[string]string{},
	}

	if schemas == nil {
		return classification
	}

	if len(roots) == 0 {
		for pair := schemas.First(); pair != nil; pair = pair.Next() {
			schema := pair.Value().Schema()
			if KeyOf(pair.Key(), schema).Declared() {
				classification.entities[pair.Key()] = schema
			}
		}

		return classification
	}

	classification.walk(schemas, roots)
	classification.resolveTargets()

	return classification
}

// IsEntity reports whether the document owns rows of this schema.
func (c Classification) IsEntity(schemaName string) bool {
	_, ok := c.entities[schemaName]

	return ok
}

// IsReference reports whether this schema points at an entity rather than
// describing one, and so should become a foreign key instead of a table.
func (c Classification) IsReference(schemaName string) bool {
	_, ok := c.references[schemaName]

	return ok
}

// Target names the entity a reference points at. It reports false when the
// reference names an entity the document does not contain, which is the usual
// case for a single-endpoint payload: the reference is still a reference, but
// there is nothing to point a constraint at.
func (c Classification) Target(schemaName string) (string, bool) {
	target, ok := c.targets[schemaName]

	return target, ok
}

// discriminatorValue returns the single value a reference's discriminator is
// pinned to. A property constrained to exactly one value is not data — it is
// the schema naming what it points at.
func discriminatorValue(schema *highbase.Schema) (string, bool) {
	if schema == nil || schema.Properties == nil {
		return "", false
	}

	for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
		property := pair.Value().Schema()
		if property == nil || len(property.Enum) != 1 {
			continue
		}

		if value := property.Enum[0].Value; value != "" {
			return value, true
		}
	}

	return "", false
}

// ReferencedSchemas lists the component schemas a schema points at, through a
// composition keyword, a property, or an array's items. Callers use it to find
// the schemas a payload carries without repeating the traversal.
func ReferencedSchemas(schema *highbase.Schema) []string {
	if schema == nil {
		return nil
	}

	names := compositionRefs(schema)
	names = append(names, propertyRefs(schema)...)
	names = append(names, itemsRef(schema.Items)...)

	return names
}

// compositionRefs lists the schemas named by allOf, anyOf and oneOf.
func compositionRefs(schema *highbase.Schema) []string {
	var names []string

	for _, group := range [][]*highbase.SchemaProxy{schema.AllOf, schema.AnyOf, schema.OneOf} {
		for _, member := range group {
			names = append(names, refName(member)...)
		}
	}

	return names
}

// propertyRefs lists the schemas named by properties, and by the items of any
// property that is an array.
func propertyRefs(schema *highbase.Schema) []string {
	if schema.Properties == nil {
		return nil
	}

	var names []string

	for pair := schema.Properties.First(); pair != nil; pair = pair.Next() {
		names = append(names, refName(pair.Value())...)

		if property := pair.Value().Schema(); property != nil {
			names = append(names, itemsRef(property.Items)...)
		}
	}

	return names
}

// itemsRef lists the schema an array's items name, when the items are a single
// schema rather than a tuple.
func itemsRef(items *highbase.DynamicValue[*highbase.SchemaProxy, bool]) []string {
	if items == nil || !items.IsA() {
		return nil
	}

	return refName(items.A)
}

// refName lists the component name a proxy refers to, or nothing when it is an
// inline schema.
func refName(proxy *highbase.SchemaProxy) []string {
	if proxy == nil {
		return nil
	}

	if name, ok := schemaNameFromRef(proxy.GetReference()); ok {
		return []string{name}
	}

	return nil
}

// schemaNameFromRef reads the component name out of a local reference.
func schemaNameFromRef(ref string) (string, bool) {
	const prefix = "#/components/schemas/"

	if name, ok := strings.CutPrefix(ref, prefix); ok && name != "" {
		return name, true
	}

	return "", false
}

// normaliseName reduces a name to its comparable core: lowercase, letters and
// digits only.
func normaliseName(name string) string {
	return nonIdentifier.ReplaceAllString(strings.ToLower(name), "")
}

// RelationTo reports what a reference to schemaName should become.
//
// An entity yields a foreign key to its own key. A reference yields a foreign
// key to the entity it names, or — when the document does not carry that
// entity — an unconstrained column of the reference's own key type. Anything
// else has no identity to point at and is embedded.
func (c Classification) RelationTo(schemaName string) Relation {
	if schema, ok := c.entities[schemaName]; ok {
		return Relation{Table: TableName(schemaName), Key: KeyOf(schemaName, schema)}
	}

	reference, ok := c.references[schemaName]
	if !ok {
		return Relation{Embed: true}
	}

	target, ok := c.targets[schemaName]
	if !ok {
		return Relation{Key: KeyOf(schemaName, reference)}
	}

	return Relation{Table: TableName(target), Key: KeyOf(target, c.entities[target])}
}

// walk visits every schema reachable from roots, filing each keyed one
// according to how it was reached.
func (c *Classification) walk(
	schemas *orderedmap.Map[string, *highbase.SchemaProxy],
	roots []string,
) {
	queue := make([]step, 0, len(roots))
	for _, root := range roots {
		queue = append(queue, step{name: root})
	}

	seen := map[step]bool{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		proxy, ok := schemas.Get(current.name)
		if !ok || seen[current] {
			continue
		}

		seen[current] = true

		schema := proxy.Schema()
		keyed := c.record(current, schema)

		for _, next := range ReferencedSchemas(schema) {
			queue = append(queue, step{name: next, crossed: current.crossed || keyed})
		}
	}

	c.preferOwnership()
}

// record files a keyed schema as an entity or a reference, and reports whether
// it was keyed at all.
func (c *Classification) record(at step, schema *highbase.Schema) bool {
	keyed := KeyOf(at.name, schema).Declared()

	switch {
	case keyed && at.crossed:
		c.references[at.name] = schema
	case keyed:
		c.entities[at.name] = schema
	}

	return keyed
}

// preferOwnership settles a schema reached both ways: one the document owns
// somewhere is an entity, whatever other path also points at it.
func (c *Classification) preferOwnership() {
	for name := range c.entities {
		delete(c.references, name)
	}
}

// resolveTargets matches each reference's discriminator against the entity
// names in the document.
func (c *Classification) resolveTargets() {
	for name, schema := range c.references {
		discriminator, ok := discriminatorValue(schema)
		if !ok {
			continue
		}

		if target, ok := c.matchEntity(discriminator); ok {
			c.targets[name] = target
		}
	}
}

// matchEntity finds the one entity a discriminator names. The comparison is on
// normalised names so that a value like "academicSession" reaches a schema
// called "AcademicSessionDType" without this package knowing that "DType" is a
// suffix some specifications happen to use. An ambiguous match resolves to
// nothing, because guessing would be worse than reporting.
func (c Classification) matchEntity(discriminator string) (string, bool) {
	wanted := normaliseName(discriminator)

	var found string

	for name := range c.entities {
		if !strings.HasPrefix(normaliseName(name), wanted) {
			continue
		}

		if found != "" {
			return "", false
		}

		found = name
	}

	return found, found != ""
}
