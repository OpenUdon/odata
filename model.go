package odata

import "strings"

const (
	// SourceKindXML marks models parsed from CSDL XML.
	SourceKindXML = "csdl_xml"
	// SourceKindJSON marks models parsed from CSDL JSON.
	SourceKindJSON = "csdl_json"
)

// Model is a parsed OData CSDL metadata artifact.
type Model struct {
	SourceKind      string
	Version         string
	EntityContainer string
	Schemas         []*Schema
	Operations      []*OperationSummary
	EntityTypes     map[string]*StructuredType
	ComplexTypes    map[string]*StructuredType
	EnumTypes       map[string]*EnumType
	Functions       map[string][]*Operation
	Actions         map[string][]*Operation
	RawJSON         map[string]any
	RawXML          string
}

// OperationSummary describes a selectable OData entity, function, or action
// operation.
type OperationSummary struct {
	ID              string
	Kind            string
	Name            string
	Container       string
	EntitySet       string
	Singleton       string
	EntityType      string
	Operation       string
	Bound           bool
	EntitySetPath   string
	Parameters      []*Parameter
	ReturnType      *ReturnType
	Annotations     []*Annotation
	QueryRelevant   bool
	NavigationPaths []string
}

// Schema describes an OData schema namespace.
type Schema struct {
	Namespace        string
	Alias            string
	EntityContainers []*EntityContainer
	EntityTypes      []*StructuredType
	ComplexTypes     []*StructuredType
	EnumTypes        []*EnumType
	Functions        []*Operation
	Actions          []*Operation
	Annotations      []*Annotation
	Raw              map[string]any
}

// EntityContainer describes an OData entity container.
type EntityContainer struct {
	Name            string
	FullName        string
	EntitySets      []*EntitySet
	Singletons      []*Singleton
	FunctionImports []*OperationImport
	ActionImports   []*OperationImport
	Annotations     []*Annotation
	Raw             map[string]any
}

// EntitySet describes an OData entity set.
type EntitySet struct {
	Name                       string
	EntityType                 string
	IncludeInServiceDocument   bool
	NavigationPropertyBindings []*NavigationPropertyBinding
	Annotations                []*Annotation
	Raw                        map[string]any
}

// Singleton describes an OData singleton.
type Singleton struct {
	Name                       string
	Type                       string
	NavigationPropertyBindings []*NavigationPropertyBinding
	Annotations                []*Annotation
	Raw                        map[string]any
}

// NavigationPropertyBinding describes an OData navigation-property binding.
type NavigationPropertyBinding struct {
	Path   string
	Target string
}

// OperationImport describes an OData function import or action import.
type OperationImport struct {
	Kind                     string
	Name                     string
	Operation                string
	EntitySet                string
	IncludeInServiceDocument bool
	Annotations              []*Annotation
	Raw                      map[string]any
}

// StructuredType describes an OData entity or complex type.
type StructuredType struct {
	Kind                 string
	Name                 string
	FullName             string
	BaseType             string
	Abstract             bool
	OpenType             bool
	HasStream            bool
	Key                  []string
	Properties           []*Property
	NavigationProperties []*NavigationProperty
	Annotations          []*Annotation
	Raw                  map[string]any
}

// Property describes an OData structural property.
type Property struct {
	Name         string
	Type         string
	Collection   bool
	Nullable     bool
	MaxLength    string
	Precision    string
	Scale        string
	SRID         string
	DefaultValue string
	Annotations  []*Annotation
	Raw          map[string]any
}

// NavigationProperty describes an OData navigation property.
type NavigationProperty struct {
	Name                   string
	Type                   string
	Collection             bool
	Nullable               bool
	Partner                string
	ContainsTarget         bool
	ReferentialConstraints []*ReferentialConstraint
	Annotations            []*Annotation
	Raw                    map[string]any
}

// ReferentialConstraint describes an OData referential constraint.
type ReferentialConstraint struct {
	Property           string
	ReferencedProperty string
	Annotations        []*Annotation
}

// EnumType describes an OData enum type.
type EnumType struct {
	Name           string
	FullName       string
	UnderlyingType string
	IsFlags        bool
	Members        []*EnumMember
	Annotations    []*Annotation
	Raw            map[string]any
}

// EnumMember describes an OData enum member.
type EnumMember struct {
	Name        string
	Value       string
	Annotations []*Annotation
	Raw         map[string]any
}

// Operation describes an OData function or action overload.
type Operation struct {
	Kind          string
	Name          string
	FullName      string
	Namespace     string
	IsBound       bool
	EntitySetPath string
	Parameters    []*Parameter
	ReturnType    *ReturnType
	Annotations   []*Annotation
	Raw           map[string]any
}

// Parameter describes an OData operation parameter.
type Parameter struct {
	Name        string
	Type        string
	Collection  bool
	Nullable    bool
	Annotations []*Annotation
	Raw         map[string]any
}

// ReturnType describes an OData operation return type.
type ReturnType struct {
	Type        string
	Collection  bool
	Nullable    bool
	Annotations []*Annotation
	Raw         map[string]any
}

// Annotation preserves a CSDL annotation value in a small native form.
type Annotation struct {
	Term      string
	Qualifier string
	Value     string
	Path      string
	Raw       map[string]any
}

// EntityTypeByName returns an entity type by fully-qualified name.
func (m *Model) EntityTypeByName(name string) (*StructuredType, bool) {
	if m == nil {
		return nil, false
	}
	name = normalizeName(name)
	if name == "" {
		return nil, false
	}
	typ, ok := m.EntityTypes[name]
	return typ, ok
}

// EntityContainerByName returns an entity container by fully-qualified name.
func (m *Model) EntityContainerByName(name string) (*EntityContainer, bool) {
	if m == nil {
		return nil, false
	}
	name = normalizeName(name)
	if name == "" {
		return nil, false
	}
	for _, schema := range m.Schemas {
		for _, container := range schema.EntityContainers {
			if container != nil && container.FullName == name {
				return container, true
			}
		}
	}
	return nil, false
}

// OperationByID returns a selectable OData operation summary by canonical ID.
func (m *Model) OperationByID(id string) (*OperationSummary, bool) {
	if m == nil {
		return nil, false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, false
	}
	for _, op := range m.Operations {
		if op != nil && op.ID == id {
			return op, true
		}
	}
	return nil, false
}

// ComplexTypeByName returns a complex type by fully-qualified name.
func (m *Model) ComplexTypeByName(name string) (*StructuredType, bool) {
	if m == nil {
		return nil, false
	}
	name = normalizeName(name)
	if name == "" {
		return nil, false
	}
	typ, ok := m.ComplexTypes[name]
	return typ, ok
}

// EnumTypeByName returns an enum type by fully-qualified name.
func (m *Model) EnumTypeByName(name string) (*EnumType, bool) {
	if m == nil {
		return nil, false
	}
	name = normalizeName(name)
	if name == "" {
		return nil, false
	}
	enum, ok := m.EnumTypes[name]
	return enum, ok
}

func normalizeName(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(name), ".")
}
