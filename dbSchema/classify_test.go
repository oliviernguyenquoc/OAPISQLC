package dbSchema_test

import (
	"testing"

	"github.com/oliviernguyenquoc/oapisqlc/dbSchema"
	"github.com/pb33f/libopenapi"
	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// oneRosterShaped mirrors the structure of a OneRoster payload: an unkeyed
// envelope owning entities, which reach other entities only through reference
// wrappers pinned to a single discriminator value.
const oneRosterShaped = `
openapi: 3.1.0
info: {title: t, version: "1"}
components:
  schemas:
    RosterDType:
      properties:
        classes: {type: array, items: {$ref: '#/components/schemas/ClassDType'}}
        courses: {type: array, items: {$ref: '#/components/schemas/CourseDType'}}
    ClassDType:
      required: [sourcedId]
      properties:
        sourcedId: {type: string}
        title: {type: string}
        course: {$ref: '#/components/schemas/CourseGUIDRefDType'}
        terms:
          type: array
          items: {$ref: '#/components/schemas/AcadSessionGUIDRefDType'}
        metadata: {$ref: '#/components/schemas/MetadataDType'}
    CourseDType:
      required: [sourcedId]
      properties:
        sourcedId: {type: string}
        title: {type: string}
    CourseGUIDRefDType:
      required: [href, sourcedId, type]
      properties:
        href: {type: string, format: uri}
        sourcedId: {type: string}
        type: {type: string, enum: [course]}
    AcadSessionGUIDRefDType:
      required: [href, sourcedId, type]
      properties:
        href: {type: string, format: uri}
        sourcedId: {type: string}
        type: {type: string, enum: [academicSession]}
    MetadataDType:
      properties:
        note: {type: string}
`

// componentsOf builds the schema map of a components-only OpenAPI document so
// that a test can state its input as the spec a user would write.
func componentsOf(t *testing.T, spec string) *orderedmap.Map[string, *highbase.SchemaProxy] {
	t.Helper()

	document, err := libopenapi.NewDocument([]byte(spec))
	if err != nil {
		t.Fatalf("cannot create document: %v", err)
	}

	model, err := document.BuildV3Model()
	if err != nil {
		t.Fatalf("cannot build model: %v", err)
	}

	return model.Model.Components.Schemas
}

func TestClassifyOwnedSchemasAreEntities(t *testing.T) {
	t.Parallel()

	got := dbSchema.Classify(componentsOf(t, oneRosterShaped), []string{"RosterDType"})

	for _, name := range []string{"ClassDType", "CourseDType"} {
		if !got.IsEntity(name) || got.IsReference(name) {
			t.Errorf("%s is owned by the envelope, so it is an entity", name)
		}
	}

	// The envelope carries the entities but has no identity of its own.
	if got.IsEntity("RosterDType") || got.IsReference("RosterDType") {
		t.Error("RosterDType is the envelope, neither an entity nor a reference")
	}
}

func TestClassifyWrappersAreReferences(t *testing.T) {
	t.Parallel()

	got := dbSchema.Classify(componentsOf(t, oneRosterShaped), []string{"RosterDType"})

	// Reached through a single property and through an array respectively;
	// cardinality does not change what they are.
	for _, name := range []string{"CourseGUIDRefDType", "AcadSessionGUIDRefDType"} {
		if !got.IsReference(name) || got.IsEntity(name) {
			t.Errorf("%s points at an entity, so it is a reference", name)
		}
	}
}

func TestClassifyKeylessSchemasAreNeither(t *testing.T) {
	t.Parallel()

	got := dbSchema.Classify(componentsOf(t, oneRosterShaped), []string{"RosterDType"})

	// Nothing can reference it and it owns nothing, so it stays embedded.
	if got.IsEntity("MetadataDType") || got.IsReference("MetadataDType") {
		t.Error("MetadataDType has no key, so it is neither")
	}
}

func TestClassifyResolvesTargets(t *testing.T) {
	t.Parallel()

	got := dbSchema.Classify(componentsOf(t, oneRosterShaped), []string{"RosterDType"})

	// "course" reaches CourseDType without this package knowing that some
	// specifications suffix their schema names.
	if target, ok := got.Target("CourseGUIDRefDType"); !ok || target != "CourseDType" {
		t.Errorf("CourseGUIDRefDType target = %q (%v), want CourseDType", target, ok)
	}

	// The document carries no academic session, so there is nothing to point
	// a constraint at — the usual case for a single-endpoint payload.
	if target, ok := got.Target("AcadSessionGUIDRefDType"); ok {
		t.Errorf("AcadSessionGUIDRefDType should be unresolved, got %q", target)
	}
}

func TestClassifyWithoutRoots(t *testing.T) {
	t.Parallel()

	// A components-only document says nothing about what it owns, so every
	// keyed schema is an entity and nothing is a reference.
	got := dbSchema.Classify(componentsOf(t, oneRosterShaped), nil)

	for _, name := range []string{"ClassDType", "CourseDType", "CourseGUIDRefDType"} {
		if !got.IsEntity(name) {
			t.Errorf("%s should be an entity with no roots given", name)
		}

		if got.IsReference(name) {
			t.Errorf("%s should not be a reference with no roots given", name)
		}
	}
}

func TestClassifyOwnedEntityIsNeverAReference(t *testing.T) {
	t.Parallel()

	// Tag is owned by the envelope and also pointed at from Pet. Being owned
	// anywhere settles it.
	const spec = `
openapi: 3.1.0
info: {title: t, version: "1"}
components:
  schemas:
    Envelope:
      properties:
        pets: {type: array, items: {$ref: '#/components/schemas/Pet'}}
        tags: {type: array, items: {$ref: '#/components/schemas/Tag'}}
    Pet:
      properties:
        id: {type: integer}
        tag: {$ref: '#/components/schemas/Tag'}
    Tag:
      properties:
        id: {type: integer}
        name: {type: string}
`

	got := dbSchema.Classify(componentsOf(t, spec), []string{"Envelope"})

	if !got.IsEntity("Tag") || got.IsReference("Tag") {
		t.Error("Tag is owned by the envelope, so it is an entity")
	}
}
