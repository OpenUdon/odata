package odata

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	kindEntityType     = "EntityType"
	kindComplexType    = "ComplexType"
	kindEnumType       = "EnumType"
	kindFunction       = "Function"
	kindAction         = "Action"
	kindEntitySet      = "EntitySet"
	kindSingleton      = "Singleton"
	kindFunctionImport = "FunctionImport"
	kindActionImport   = "ActionImport"
)

// Parse parses CSDL XML or CSDL JSON bytes into native OData metadata.
func Parse(data []byte) (*Model, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("OData CSDL document is empty")
	}
	if trimmed[0] == '<' {
		return ParseXML(trimmed)
	}
	if trimmed[0] == '{' {
		return ParseJSON(trimmed)
	}
	return nil, fmt.Errorf("OData CSDL document must be XML or JSON")
}

// ParseXML parses CSDL XML bytes into native OData metadata.
func ParseXML(data []byte) (*Model, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("OData CSDL XML document is empty")
	}
	var root xmlRoot
	dec := xml.NewDecoder(bytes.NewReader(trimmed))
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("parse OData CSDL XML: %w", err)
	}
	if err := rejectTrailingXML(dec); err != nil {
		return nil, err
	}
	if root.XMLName.Local == "" {
		return nil, fmt.Errorf("OData CSDL XML root is missing")
	}
	model := newModel(SourceKindXML)
	model.RawXML = string(trimmed)
	switch root.XMLName.Local {
	case "Edmx":
		model.Version = root.Attr("Version")
		for _, schema := range root.DataServices.Schemas {
			if err := addXMLSchema(model, schema); err != nil {
				return nil, err
			}
		}
	case "Schema":
		var schema xmlSchema
		if err := xml.Unmarshal(trimmed, &schema); err != nil {
			return nil, fmt.Errorf("parse OData CSDL XML schema: %w", err)
		}
		if err := addXMLSchema(model, schema); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported OData CSDL XML root %q", root.XMLName.Local)
	}
	if len(model.Schemas) == 0 {
		return nil, fmt.Errorf("OData CSDL XML contains no schemas")
	}
	if err := buildOperationSummaries(model); err != nil {
		return nil, err
	}
	return model, nil
}

// ParseJSON parses CSDL JSON bytes into native OData metadata.
func ParseJSON(data []byte) (*Model, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("OData CSDL JSON document is empty")
	}
	var raw map[string]any
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse OData CSDL JSON: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return nil, fmt.Errorf("parse OData CSDL JSON: trailing data after root object")
	} else if err != io.EOF {
		return nil, fmt.Errorf("parse OData CSDL JSON: %w", err)
	}
	return ParseJSONMap(raw)
}

// ParseJSONMap parses an already-decoded CSDL JSON object into native OData
// metadata.
func ParseJSONMap(raw map[string]any) (*Model, error) {
	if raw == nil {
		return nil, fmt.Errorf("OData CSDL JSON root must be an object")
	}
	model := newModel(SourceKindJSON)
	model.RawJSON = raw
	model.Version = stringValue(raw["$Version"])
	model.EntityContainer = normalizeName(stringValue(raw["$EntityContainer"]))
	for _, namespace := range sortedMapKeys(raw) {
		if strings.HasPrefix(namespace, "$") {
			continue
		}
		schemaMap, ok := raw[namespace].(map[string]any)
		if !ok {
			continue
		}
		if err := addJSONSchema(model, namespace, schemaMap); err != nil {
			return nil, err
		}
	}
	if len(model.Schemas) == 0 {
		return nil, fmt.Errorf("OData CSDL JSON contains no schemas")
	}
	if err := buildOperationSummaries(model); err != nil {
		return nil, err
	}
	return model, nil
}

func newModel(sourceKind string) *Model {
	return &Model{
		SourceKind:   sourceKind,
		EntityTypes:  map[string]*StructuredType{},
		ComplexTypes: map[string]*StructuredType{},
		EnumTypes:    map[string]*EnumType{},
		Functions:    map[string][]*Operation{},
		Actions:      map[string][]*Operation{},
	}
}

func rejectTrailingXML(dec *xml.Decoder) error {
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("parse OData CSDL XML: %w", err)
		}
		if data, ok := token.(xml.CharData); ok && strings.TrimSpace(string(data)) == "" {
			continue
		}
		return fmt.Errorf("parse OData CSDL XML: trailing data after root element")
	}
}

func buildOperationSummaries(model *Model) error {
	if model == nil {
		return nil
	}
	seen := map[string]bool{}
	add := func(summary *OperationSummary) error {
		if summary == nil {
			return nil
		}
		if strings.TrimSpace(summary.ID) == "" {
			return fmt.Errorf("OData operation summary has empty ID")
		}
		if seen[summary.ID] {
			return fmt.Errorf("duplicate OData operation summary %q", summary.ID)
		}
		seen[summary.ID] = true
		model.Operations = append(model.Operations, summary)
		return nil
	}
	importedFunctions := map[string]bool{}
	importedActions := map[string]bool{}
	functionOverloads := operationOverloadCounts(model.Functions)
	actionOverloads := operationOverloadCounts(model.Actions)
	for _, schema := range model.Schemas {
		for _, container := range schema.EntityContainers {
			for _, set := range container.EntitySets {
				if err := add(entitySetSummary(container, set, "read")); err != nil {
					return err
				}
				if err := add(entitySetSummary(container, set, "query")); err != nil {
					return err
				}
			}
			for _, singleton := range container.Singletons {
				if err := add(singletonSummary(container, singleton)); err != nil {
					return err
				}
			}
			for _, imp := range container.FunctionImports {
				op := firstOperation(model.Functions[imp.Operation])
				if err := add(importSummary("function", container, imp, op)); err != nil {
					return err
				}
				importedFunctions[imp.Operation] = true
			}
			for _, imp := range container.ActionImports {
				op := firstOperation(model.Actions[imp.Operation])
				if err := add(importSummary("action", container, imp, op)); err != nil {
					return err
				}
				importedActions[imp.Operation] = true
			}
		}
		for _, op := range schema.Functions {
			if importedFunctions[op.FullName] && !op.IsBound {
				continue
			}
			if err := add(operationDefinitionSummary("function", op, functionOverloads[op.FullName] > 1)); err != nil {
				return err
			}
		}
		for _, op := range schema.Actions {
			if importedActions[op.FullName] && !op.IsBound {
				continue
			}
			if err := add(operationDefinitionSummary("action", op, actionOverloads[op.FullName] > 1)); err != nil {
				return err
			}
		}
	}
	return nil
}

func entitySetSummary(container *EntityContainer, set *EntitySet, verb string) *OperationSummary {
	return &OperationSummary{
		ID:              "entitySet." + set.Name + "." + verb,
		Kind:            "entitySet." + verb,
		Name:            set.Name,
		Container:       container.FullName,
		EntitySet:       set.Name,
		EntityType:      set.EntityType,
		ReturnType:      &ReturnType{Type: set.EntityType, Collection: verb == "query", Nullable: false},
		Annotations:     set.Annotations,
		QueryRelevant:   verb == "query",
		NavigationPaths: navigationBindingPaths(set.NavigationPropertyBindings),
	}
}

func singletonSummary(container *EntityContainer, singleton *Singleton) *OperationSummary {
	return &OperationSummary{
		ID:              "singleton." + singleton.Name + ".read",
		Kind:            "singleton.read",
		Name:            singleton.Name,
		Container:       container.FullName,
		Singleton:       singleton.Name,
		EntityType:      singleton.Type,
		ReturnType:      &ReturnType{Type: singleton.Type, Nullable: false},
		Annotations:     singleton.Annotations,
		NavigationPaths: navigationBindingPaths(singleton.NavigationPropertyBindings),
	}
}

func importSummary(kind string, container *EntityContainer, imp *OperationImport, op *Operation) *OperationSummary {
	summary := &OperationSummary{
		ID:          kind + "." + container.Name + "." + imp.Name,
		Kind:        kind,
		Name:        imp.Name,
		Container:   container.FullName,
		Operation:   imp.Operation,
		Annotations: append(append([]*Annotation(nil), imp.Annotations...), annotationsFromOperation(op)...),
	}
	if op != nil {
		summary.Bound = op.IsBound
		summary.EntitySetPath = op.EntitySetPath
		summary.Parameters = op.Parameters
		summary.ReturnType = op.ReturnType
	}
	return summary
}

func operationDefinitionSummary(kind string, op *Operation, overloaded bool) *OperationSummary {
	if op == nil {
		return nil
	}
	id := kind + "." + op.FullName
	if overloaded {
		id += operationSignature(op.Parameters)
	}
	return &OperationSummary{
		ID:            id,
		Kind:          kind,
		Name:          op.Name,
		Operation:     op.FullName,
		Bound:         op.IsBound,
		EntitySetPath: op.EntitySetPath,
		Parameters:    op.Parameters,
		ReturnType:    op.ReturnType,
		Annotations:   op.Annotations,
	}
}

func operationOverloadCounts(index map[string][]*Operation) map[string]int {
	out := map[string]int{}
	for fullName, ops := range index {
		for _, op := range ops {
			if op != nil {
				out[fullName]++
			}
		}
	}
	return out
}

func operationSignature(params []*Parameter) string {
	names := make([]string, 0, len(params))
	for _, param := range params {
		if param != nil {
			names = append(names, param.Name)
		}
	}
	return "(" + strings.Join(names, ",") + ")"
}

func annotationsFromOperation(op *Operation) []*Annotation {
	if op == nil {
		return nil
	}
	return op.Annotations
}

func firstOperation(ops []*Operation) *Operation {
	for _, op := range ops {
		if op != nil {
			return op
		}
	}
	return nil
}

func navigationBindingPaths(bindings []*NavigationPropertyBinding) []string {
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding != nil && binding.Path != "" {
			out = append(out, binding.Path)
		}
	}
	return out
}

func addXMLSchema(model *Model, raw xmlSchema) error {
	namespace := strings.TrimSpace(raw.Namespace)
	if namespace == "" {
		return fmt.Errorf("OData CSDL schema namespace is required")
	}
	schema := &Schema{
		Namespace:   namespace,
		Alias:       strings.TrimSpace(raw.Alias),
		Annotations: annotationsFromXML(raw.Annotations),
	}
	for _, rawContainer := range raw.EntityContainers {
		container, err := entityContainerFromXML(namespace, rawContainer)
		if err != nil {
			return err
		}
		schema.EntityContainers = append(schema.EntityContainers, container)
		if model.EntityContainer == "" {
			model.EntityContainer = container.FullName
		}
	}
	for _, rawType := range raw.EntityTypes {
		typ, err := structuredTypeFromXML(namespace, kindEntityType, rawType)
		if err != nil {
			return err
		}
		if err := addStructuredType(model.EntityTypes, typ, "entity type"); err != nil {
			return err
		}
		schema.EntityTypes = append(schema.EntityTypes, typ)
	}
	for _, rawType := range raw.ComplexTypes {
		typ, err := structuredTypeFromXML(namespace, kindComplexType, rawType)
		if err != nil {
			return err
		}
		if err := addStructuredType(model.ComplexTypes, typ, "complex type"); err != nil {
			return err
		}
		schema.ComplexTypes = append(schema.ComplexTypes, typ)
	}
	for _, rawEnum := range raw.EnumTypes {
		enum, err := enumTypeFromXML(namespace, rawEnum)
		if err != nil {
			return err
		}
		if _, exists := model.EnumTypes[enum.FullName]; exists {
			return fmt.Errorf("duplicate OData enum type %q", enum.FullName)
		}
		model.EnumTypes[enum.FullName] = enum
		schema.EnumTypes = append(schema.EnumTypes, enum)
	}
	for _, rawFunction := range raw.Functions {
		op, err := operationFromXML(namespace, kindFunction, rawFunction)
		if err != nil {
			return err
		}
		model.Functions[op.FullName] = append(model.Functions[op.FullName], op)
		schema.Functions = append(schema.Functions, op)
	}
	for _, rawAction := range raw.Actions {
		op, err := operationFromXML(namespace, kindAction, rawAction)
		if err != nil {
			return err
		}
		model.Actions[op.FullName] = append(model.Actions[op.FullName], op)
		schema.Actions = append(schema.Actions, op)
	}
	model.Schemas = append(model.Schemas, schema)
	return nil
}

func addJSONSchema(model *Model, namespace string, raw map[string]any) error {
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("OData CSDL schema namespace is required")
	}
	schema := &Schema{
		Namespace:   namespace,
		Alias:       stringValue(raw["$Alias"]),
		Annotations: annotationsFromJSON(raw),
		Raw:         raw,
	}
	for _, name := range sortedMapKeys(raw) {
		if strings.HasPrefix(name, "$") {
			continue
		}
		value := raw[name]
		switch typed := value.(type) {
		case map[string]any:
			if err := addJSONSchemaMember(model, schema, namespace, name, typed); err != nil {
				return err
			}
		case []any:
			for _, elem := range typed {
				member, ok := elem.(map[string]any)
				if !ok {
					return fmt.Errorf("OData CSDL JSON member %s.%s overload must be an object", namespace, name)
				}
				if err := addJSONSchemaMember(model, schema, namespace, name, member); err != nil {
					return err
				}
			}
		}
	}
	model.Schemas = append(model.Schemas, schema)
	return nil
}

func addJSONSchemaMember(model *Model, schema *Schema, namespace, name string, raw map[string]any) error {
	kind := stringValue(raw["$Kind"])
	switch kind {
	case "EntityContainer":
		container, err := entityContainerFromJSON(namespace, name, raw)
		if err != nil {
			return err
		}
		schema.EntityContainers = append(schema.EntityContainers, container)
		if model.EntityContainer == "" {
			model.EntityContainer = container.FullName
		}
	case "", kindEntityType:
		if kind == "" && !looksLikeStructuredJSON(raw) {
			return nil
		}
		typ, err := structuredTypeFromJSON(namespace, kindEntityType, name, raw)
		if err != nil {
			return err
		}
		if err := addStructuredType(model.EntityTypes, typ, "entity type"); err != nil {
			return err
		}
		schema.EntityTypes = append(schema.EntityTypes, typ)
	case kindComplexType:
		typ, err := structuredTypeFromJSON(namespace, kindComplexType, name, raw)
		if err != nil {
			return err
		}
		if err := addStructuredType(model.ComplexTypes, typ, "complex type"); err != nil {
			return err
		}
		schema.ComplexTypes = append(schema.ComplexTypes, typ)
	case kindEnumType:
		enum, err := enumTypeFromJSON(namespace, name, raw)
		if err != nil {
			return err
		}
		if _, exists := model.EnumTypes[enum.FullName]; exists {
			return fmt.Errorf("duplicate OData enum type %q", enum.FullName)
		}
		model.EnumTypes[enum.FullName] = enum
		schema.EnumTypes = append(schema.EnumTypes, enum)
	case kindFunction:
		op, err := operationFromJSON(namespace, kindFunction, name, raw)
		if err != nil {
			return err
		}
		model.Functions[op.FullName] = append(model.Functions[op.FullName], op)
		schema.Functions = append(schema.Functions, op)
	case kindAction:
		op, err := operationFromJSON(namespace, kindAction, name, raw)
		if err != nil {
			return err
		}
		model.Actions[op.FullName] = append(model.Actions[op.FullName], op)
		schema.Actions = append(schema.Actions, op)
	}
	return nil
}

func entityContainerFromXML(namespace string, raw xmlEntityContainer) (*EntityContainer, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData entity container in %s has empty name", namespace)
	}
	container := &EntityContainer{
		Name:        name,
		FullName:    qualifyName(namespace, name),
		Annotations: annotationsFromXML(raw.Annotations),
	}
	for _, rawSet := range raw.EntitySets {
		set, err := entitySetFromXML(rawSet)
		if err != nil {
			return nil, err
		}
		container.EntitySets = append(container.EntitySets, set)
	}
	for _, rawSingleton := range raw.Singletons {
		singleton, err := singletonFromXML(rawSingleton)
		if err != nil {
			return nil, err
		}
		container.Singletons = append(container.Singletons, singleton)
	}
	for _, rawImport := range raw.FunctionImports {
		imp, err := operationImportFromXML(kindFunctionImport, rawImport.Name, rawImport.Function, rawImport.EntitySet, rawImport.IncludeInServiceDocument, rawImport.Annotations)
		if err != nil {
			return nil, err
		}
		container.FunctionImports = append(container.FunctionImports, imp)
	}
	for _, rawImport := range raw.ActionImports {
		imp, err := operationImportFromXML(kindActionImport, rawImport.Name, rawImport.Action, rawImport.EntitySet, "", rawImport.Annotations)
		if err != nil {
			return nil, err
		}
		container.ActionImports = append(container.ActionImports, imp)
	}
	return container, nil
}

func entityContainerFromJSON(namespace, name string, raw map[string]any) (*EntityContainer, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("OData entity container in %s has empty name", namespace)
	}
	container := &EntityContainer{
		Name:        name,
		FullName:    qualifyName(namespace, name),
		Annotations: annotationsFromJSON(raw),
		Raw:         raw,
	}
	for _, memberName := range sortedMapKeys(raw) {
		if strings.HasPrefix(memberName, "$") {
			continue
		}
		member, ok := raw[memberName].(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(member["$Kind"]) {
		case "", kindEntitySet:
			if stringValue(member["$Type"]) == "" {
				continue
			}
			set, err := entitySetFromJSON(memberName, member)
			if err != nil {
				return nil, err
			}
			container.EntitySets = append(container.EntitySets, set)
		case kindSingleton:
			singleton, err := singletonFromJSON(memberName, member)
			if err != nil {
				return nil, err
			}
			container.Singletons = append(container.Singletons, singleton)
		case kindFunctionImport:
			imp, err := operationImportFromJSON(kindFunctionImport, memberName, member)
			if err != nil {
				return nil, err
			}
			container.FunctionImports = append(container.FunctionImports, imp)
		case kindActionImport:
			imp, err := operationImportFromJSON(kindActionImport, memberName, member)
			if err != nil {
				return nil, err
			}
			container.ActionImports = append(container.ActionImports, imp)
		}
	}
	return container, nil
}

func entitySetFromXML(raw xmlEntitySet) (*EntitySet, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData entity set has empty name")
	}
	entityType := normalizeName(raw.EntityType)
	if entityType == "" {
		return nil, fmt.Errorf("OData entity set %s has empty entity type", name)
	}
	return &EntitySet{
		Name:                       name,
		EntityType:                 entityType,
		IncludeInServiceDocument:   parseBoolDefault(raw.IncludeInServiceDocument, true),
		NavigationPropertyBindings: navigationBindingsFromXML(raw.NavigationPropertyBindings),
		Annotations:                annotationsFromXML(raw.Annotations),
	}, nil
}

func entitySetFromJSON(name string, raw map[string]any) (*EntitySet, error) {
	entityType := normalizeName(stringValue(raw["$Type"]))
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("OData entity set has empty name")
	}
	if entityType == "" {
		return nil, fmt.Errorf("OData entity set %s has empty entity type", name)
	}
	return &EntitySet{
		Name:                       name,
		EntityType:                 entityType,
		IncludeInServiceDocument:   boolValueDefault(raw["$IncludeInServiceDocument"], true),
		NavigationPropertyBindings: navigationBindingsFromJSON(raw["$NavigationPropertyBinding"]),
		Annotations:                annotationsFromJSON(raw),
		Raw:                        raw,
	}, nil
}

func singletonFromXML(raw xmlSingleton) (*Singleton, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData singleton has empty name")
	}
	typ := normalizeName(raw.Type)
	if typ == "" {
		return nil, fmt.Errorf("OData singleton %s has empty type", name)
	}
	return &Singleton{
		Name:                       name,
		Type:                       typ,
		NavigationPropertyBindings: navigationBindingsFromXML(raw.NavigationPropertyBindings),
		Annotations:                annotationsFromXML(raw.Annotations),
	}, nil
}

func singletonFromJSON(name string, raw map[string]any) (*Singleton, error) {
	typ := normalizeName(stringValue(raw["$Type"]))
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("OData singleton has empty name")
	}
	if typ == "" {
		return nil, fmt.Errorf("OData singleton %s has empty type", name)
	}
	return &Singleton{
		Name:                       name,
		Type:                       typ,
		NavigationPropertyBindings: navigationBindingsFromJSON(raw["$NavigationPropertyBinding"]),
		Annotations:                annotationsFromJSON(raw),
		Raw:                        raw,
	}, nil
}

func operationImportFromXML(kind, name, operation, entitySet, include string, annotations []xmlAnnotation) (*OperationImport, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("OData %s has empty name", kind)
	}
	operation = normalizeName(operation)
	if operation == "" {
		return nil, fmt.Errorf("OData %s %s has empty operation", kind, name)
	}
	return &OperationImport{
		Kind:                     kind,
		Name:                     name,
		Operation:                operation,
		EntitySet:                entitySet,
		IncludeInServiceDocument: parseBoolDefault(include, false),
		Annotations:              annotationsFromXML(annotations),
	}, nil
}

func operationImportFromJSON(kind, name string, raw map[string]any) (*OperationImport, error) {
	operation := normalizeName(stringValue(raw["$Function"]))
	if kind == kindActionImport {
		operation = normalizeName(stringValue(raw["$Action"]))
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("OData %s has empty name", kind)
	}
	if operation == "" {
		return nil, fmt.Errorf("OData %s %s has empty operation", kind, name)
	}
	return &OperationImport{
		Kind:                     kind,
		Name:                     name,
		Operation:                operation,
		EntitySet:                stringValue(raw["$EntitySet"]),
		IncludeInServiceDocument: boolValueDefault(raw["$IncludeInServiceDocument"], false),
		Annotations:              annotationsFromJSON(raw),
		Raw:                      raw,
	}, nil
}

func structuredTypeFromXML(namespace, kind string, raw xmlStructuredType) (*StructuredType, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData %s in %s has empty name", kind, namespace)
	}
	typ := &StructuredType{
		Kind:        kind,
		Name:        name,
		FullName:    qualifyName(namespace, name),
		BaseType:    normalizeName(raw.BaseType),
		Abstract:    parseBoolDefault(raw.Abstract, false),
		OpenType:    parseBoolDefault(raw.OpenType, false),
		HasStream:   parseBoolDefault(raw.HasStream, false),
		Key:         keyFromXML(raw.Key),
		Annotations: annotationsFromXML(raw.Annotations),
	}
	for _, rawProperty := range raw.Properties {
		property, err := propertyFromXML(rawProperty)
		if err != nil {
			return nil, err
		}
		typ.Properties = append(typ.Properties, property)
	}
	for _, rawNavigation := range raw.NavigationProperties {
		nav, err := navigationPropertyFromXML(rawNavigation)
		if err != nil {
			return nil, err
		}
		typ.NavigationProperties = append(typ.NavigationProperties, nav)
	}
	return typ, nil
}

func structuredTypeFromJSON(namespace, kind, name string, raw map[string]any) (*StructuredType, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("OData %s in %s has empty name", kind, namespace)
	}
	typ := &StructuredType{
		Kind:        kind,
		Name:        name,
		FullName:    qualifyName(namespace, name),
		BaseType:    normalizeName(stringValue(raw["$BaseType"])),
		Abstract:    boolValueDefault(raw["$Abstract"], false),
		OpenType:    boolValueDefault(raw["$OpenType"], false),
		HasStream:   boolValueDefault(raw["$HasStream"], false),
		Key:         stringSlice(raw["$Key"]),
		Annotations: annotationsFromJSON(raw),
		Raw:         raw,
	}
	for _, memberName := range sortedMapKeys(raw) {
		if strings.HasPrefix(memberName, "$") || strings.HasPrefix(memberName, "@") {
			continue
		}
		member, ok := raw[memberName].(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(stringValue(member["$Kind"]), "NavigationProperty") || boolValueDefault(member["$NavigationProperty"], false) {
			nav, err := navigationPropertyFromJSON(memberName, member)
			if err != nil {
				return nil, err
			}
			typ.NavigationProperties = append(typ.NavigationProperties, nav)
			continue
		}
		property, err := propertyFromJSON(memberName, member)
		if err != nil {
			return nil, err
		}
		typ.Properties = append(typ.Properties, property)
	}
	return typ, nil
}

func addStructuredType(index map[string]*StructuredType, typ *StructuredType, label string) error {
	if _, exists := index[typ.FullName]; exists {
		return fmt.Errorf("duplicate OData %s %q", label, typ.FullName)
	}
	index[typ.FullName] = typ
	return nil
}

func propertyFromXML(raw xmlProperty) (*Property, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData property has empty name")
	}
	typ, collection := normalizeTypeRef(raw.Type)
	if typ == "" {
		return nil, fmt.Errorf("OData property %s has empty type", name)
	}
	return &Property{
		Name:         name,
		Type:         typ,
		Collection:   collection,
		Nullable:     parseBoolDefault(raw.Nullable, true),
		MaxLength:    raw.MaxLength,
		Precision:    raw.Precision,
		Scale:        raw.Scale,
		SRID:         raw.SRID,
		DefaultValue: raw.DefaultValue,
		Annotations:  annotationsFromXML(raw.Annotations),
	}, nil
}

func propertyFromJSON(name string, raw map[string]any) (*Property, error) {
	typ, collection := normalizeTypeRef(stringValue(raw["$Type"]))
	if typ == "" {
		return nil, fmt.Errorf("OData property %s has empty type", name)
	}
	if collectionValue, ok := raw["$Collection"]; ok {
		collection = boolValueDefault(collectionValue, collection)
	}
	return &Property{
		Name:         name,
		Type:         typ,
		Collection:   collection,
		Nullable:     boolValueDefault(raw["$Nullable"], true),
		MaxLength:    stringValue(raw["$MaxLength"]),
		Precision:    stringValue(raw["$Precision"]),
		Scale:        stringValue(raw["$Scale"]),
		SRID:         stringValue(raw["$SRID"]),
		DefaultValue: stringValue(raw["$DefaultValue"]),
		Annotations:  annotationsFromJSON(raw),
		Raw:          raw,
	}, nil
}

func navigationPropertyFromXML(raw xmlNavigationProperty) (*NavigationProperty, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData navigation property has empty name")
	}
	typ, collection := normalizeTypeRef(raw.Type)
	if typ == "" {
		return nil, fmt.Errorf("OData navigation property %s has empty type", name)
	}
	return &NavigationProperty{
		Name:                   name,
		Type:                   typ,
		Collection:             collection,
		Nullable:               parseBoolDefault(raw.Nullable, true),
		Partner:                raw.Partner,
		ContainsTarget:         parseBoolDefault(raw.ContainsTarget, false),
		ReferentialConstraints: referentialConstraintsFromXML(raw.ReferentialConstraints),
		Annotations:            annotationsFromXML(raw.Annotations),
	}, nil
}

func navigationPropertyFromJSON(name string, raw map[string]any) (*NavigationProperty, error) {
	typ, collection := normalizeTypeRef(stringValue(raw["$Type"]))
	if typ == "" {
		return nil, fmt.Errorf("OData navigation property %s has empty type", name)
	}
	if collectionValue, ok := raw["$Collection"]; ok {
		collection = boolValueDefault(collectionValue, collection)
	}
	return &NavigationProperty{
		Name:                   name,
		Type:                   typ,
		Collection:             collection,
		Nullable:               boolValueDefault(raw["$Nullable"], true),
		Partner:                stringValue(raw["$Partner"]),
		ContainsTarget:         boolValueDefault(raw["$ContainsTarget"], false),
		ReferentialConstraints: referentialConstraintsFromJSON(raw["$ReferentialConstraint"]),
		Annotations:            annotationsFromJSON(raw),
		Raw:                    raw,
	}, nil
}

func enumTypeFromXML(namespace string, raw xmlEnumType) (*EnumType, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData enum type in %s has empty name", namespace)
	}
	enum := &EnumType{
		Name:           name,
		FullName:       qualifyName(namespace, name),
		UnderlyingType: normalizeName(raw.UnderlyingType),
		IsFlags:        parseBoolDefault(raw.IsFlags, false),
		Annotations:    annotationsFromXML(raw.Annotations),
	}
	for _, rawMember := range raw.Members {
		member, err := enumMemberFromXML(rawMember)
		if err != nil {
			return nil, err
		}
		enum.Members = append(enum.Members, member)
	}
	return enum, nil
}

func enumTypeFromJSON(namespace, name string, raw map[string]any) (*EnumType, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("OData enum type in %s has empty name", namespace)
	}
	enum := &EnumType{
		Name:           name,
		FullName:       qualifyName(namespace, name),
		UnderlyingType: normalizeName(stringValue(raw["$UnderlyingType"])),
		IsFlags:        boolValueDefault(raw["$IsFlags"], false),
		Annotations:    annotationsFromJSON(raw),
		Raw:            raw,
	}
	for _, memberName := range sortedMapKeys(raw) {
		if strings.HasPrefix(memberName, "$") || strings.HasPrefix(memberName, "@") {
			continue
		}
		member := &EnumMember{Name: memberName}
		if memberMap, ok := raw[memberName].(map[string]any); ok {
			member.Value = stringValue(memberMap["$Value"])
			member.Annotations = annotationsFromJSON(memberMap)
			member.Raw = memberMap
		} else {
			member.Value = stringValue(raw[memberName])
		}
		enum.Members = append(enum.Members, member)
	}
	return enum, nil
}

func enumMemberFromXML(raw xmlEnumMember) (*EnumMember, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData enum member has empty name")
	}
	return &EnumMember{
		Name:        name,
		Value:       raw.Value,
		Annotations: annotationsFromXML(raw.Annotations),
	}, nil
}

func operationFromXML(namespace, kind string, raw xmlOperation) (*Operation, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData %s in %s has empty name", kind, namespace)
	}
	op := &Operation{
		Kind:          kind,
		Name:          name,
		FullName:      qualifyName(namespace, name),
		Namespace:     namespace,
		IsBound:       parseBoolDefault(raw.IsBound, false),
		EntitySetPath: raw.EntitySetPath,
		Annotations:   annotationsFromXML(raw.Annotations),
	}
	for _, rawParam := range raw.Parameters {
		param, err := parameterFromXML(rawParam)
		if err != nil {
			return nil, err
		}
		op.Parameters = append(op.Parameters, param)
	}
	if raw.ReturnType != nil {
		ret, err := returnTypeFromXML(*raw.ReturnType)
		if err != nil {
			return nil, err
		}
		op.ReturnType = ret
	}
	return op, nil
}

func operationFromJSON(namespace, kind, name string, raw map[string]any) (*Operation, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("OData %s in %s has empty name", kind, namespace)
	}
	op := &Operation{
		Kind:          kind,
		Name:          name,
		FullName:      qualifyName(namespace, name),
		Namespace:     namespace,
		IsBound:       boolValueDefault(raw["$IsBound"], false),
		EntitySetPath: stringValue(raw["$EntitySetPath"]),
		Annotations:   annotationsFromJSON(raw),
		Raw:           raw,
	}
	if params, ok := raw["$Parameter"].([]any); ok {
		for _, elem := range params {
			paramMap, ok := elem.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("OData %s %s parameter must be an object", kind, op.FullName)
			}
			param, err := parameterFromJSON(paramMap)
			if err != nil {
				return nil, err
			}
			op.Parameters = append(op.Parameters, param)
		}
	}
	if returnMap, ok := raw["$ReturnType"].(map[string]any); ok {
		ret, err := returnTypeFromJSON(returnMap)
		if err != nil {
			return nil, err
		}
		op.ReturnType = ret
	}
	return op, nil
}

func parameterFromXML(raw xmlParameter) (*Parameter, error) {
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, fmt.Errorf("OData parameter has empty name")
	}
	typ, collection := normalizeTypeRef(raw.Type)
	if typ == "" {
		return nil, fmt.Errorf("OData parameter %s has empty type", name)
	}
	return &Parameter{
		Name:        name,
		Type:        typ,
		Collection:  collection,
		Nullable:    parseBoolDefault(raw.Nullable, true),
		Annotations: annotationsFromXML(raw.Annotations),
	}, nil
}

func parameterFromJSON(raw map[string]any) (*Parameter, error) {
	name := stringValue(raw["$Name"])
	if name == "" {
		name = stringValue(raw["Name"])
	}
	if name == "" {
		return nil, fmt.Errorf("OData parameter has empty name")
	}
	typ, collection := normalizeTypeRef(stringValue(raw["$Type"]))
	if typ == "" {
		return nil, fmt.Errorf("OData parameter %s has empty type", name)
	}
	if collectionValue, ok := raw["$Collection"]; ok {
		collection = boolValueDefault(collectionValue, collection)
	}
	return &Parameter{
		Name:        name,
		Type:        typ,
		Collection:  collection,
		Nullable:    boolValueDefault(raw["$Nullable"], true),
		Annotations: annotationsFromJSON(raw),
		Raw:         raw,
	}, nil
}

func returnTypeFromXML(raw xmlReturnType) (*ReturnType, error) {
	typ, collection := normalizeTypeRef(raw.Type)
	if typ == "" {
		return nil, fmt.Errorf("OData return type has empty type")
	}
	return &ReturnType{
		Type:        typ,
		Collection:  collection,
		Nullable:    parseBoolDefault(raw.Nullable, true),
		Annotations: annotationsFromXML(raw.Annotations),
	}, nil
}

func returnTypeFromJSON(raw map[string]any) (*ReturnType, error) {
	typ, collection := normalizeTypeRef(stringValue(raw["$Type"]))
	if typ == "" {
		return nil, fmt.Errorf("OData return type has empty type")
	}
	if collectionValue, ok := raw["$Collection"]; ok {
		collection = boolValueDefault(collectionValue, collection)
	}
	return &ReturnType{
		Type:        typ,
		Collection:  collection,
		Nullable:    boolValueDefault(raw["$Nullable"], true),
		Annotations: annotationsFromJSON(raw),
		Raw:         raw,
	}, nil
}

func keyFromXML(raw xmlKey) []string {
	var out []string
	for _, ref := range raw.PropertyRefs {
		name := strings.TrimSpace(ref.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func navigationBindingsFromXML(raw []xmlNavigationPropertyBinding) []*NavigationPropertyBinding {
	var out []*NavigationPropertyBinding
	for _, binding := range raw {
		if strings.TrimSpace(binding.Path) == "" || strings.TrimSpace(binding.Target) == "" {
			continue
		}
		out = append(out, &NavigationPropertyBinding{Path: binding.Path, Target: binding.Target})
	}
	return out
}

func navigationBindingsFromJSON(raw any) []*NavigationPropertyBinding {
	bindings, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make([]*NavigationPropertyBinding, 0, len(bindings))
	for _, path := range sortedMapKeys(bindings) {
		target := stringValue(bindings[path])
		if target != "" {
			out = append(out, &NavigationPropertyBinding{Path: path, Target: target})
		}
	}
	return out
}

func referentialConstraintsFromXML(raw []xmlReferentialConstraint) []*ReferentialConstraint {
	var out []*ReferentialConstraint
	for _, constraint := range raw {
		if strings.TrimSpace(constraint.Property) == "" || strings.TrimSpace(constraint.ReferencedProperty) == "" {
			continue
		}
		out = append(out, &ReferentialConstraint{
			Property:           constraint.Property,
			ReferencedProperty: constraint.ReferencedProperty,
			Annotations:        annotationsFromXML(constraint.Annotations),
		})
	}
	return out
}

func referentialConstraintsFromJSON(raw any) []*ReferentialConstraint {
	constraints, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make([]*ReferentialConstraint, 0, len(constraints))
	for _, property := range sortedMapKeys(constraints) {
		referencedProperty := stringValue(constraints[property])
		if referencedProperty != "" {
			out = append(out, &ReferentialConstraint{Property: property, ReferencedProperty: referencedProperty})
		}
	}
	return out
}

func annotationsFromXML(raw []xmlAnnotation) []*Annotation {
	var out []*Annotation
	for _, ann := range raw {
		term := normalizeName(ann.Term)
		if term == "" {
			continue
		}
		out = append(out, &Annotation{
			Term:      term,
			Qualifier: ann.Qualifier,
			Value:     firstNonEmpty(ann.String, ann.Bool, ann.Int, ann.Decimal),
			Path:      ann.Path,
		})
	}
	return out
}

func annotationsFromJSON(raw map[string]any) []*Annotation {
	if raw == nil {
		return nil
	}
	var out []*Annotation
	for _, key := range sortedMapKeys(raw) {
		if !strings.HasPrefix(key, "@") {
			continue
		}
		term := strings.TrimPrefix(key, "@")
		if term == "" {
			continue
		}
		out = append(out, &Annotation{
			Term:  normalizeName(term),
			Value: stringValue(raw[key]),
			Raw:   map[string]any{key: raw[key]},
		})
	}
	return out
}

func normalizeTypeRef(typ string) (string, bool) {
	typ = strings.TrimSpace(typ)
	if strings.HasPrefix(typ, "Collection(") && strings.HasSuffix(typ, ")") {
		return normalizeName(strings.TrimSuffix(strings.TrimPrefix(typ, "Collection("), ")")), true
	}
	return normalizeName(typ), false
}

func qualifyName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" {
		return name
	}
	return namespace + "." + name
}

func looksLikeStructuredJSON(raw map[string]any) bool {
	if _, ok := raw["$Key"]; ok {
		return true
	}
	for _, name := range raw {
		if member, ok := name.(map[string]any); ok && stringValue(member["$Type"]) != "" {
			return true
		}
	}
	return false
}

func parseBoolDefault(value string, def bool) bool {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return strings.EqualFold(value, "true")
}

func boolValueDefault(value any, def bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return parseBoolDefault(typed, def)
	default:
		return def
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(typed)
	}
}

func stringSlice(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s := stringValue(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type xmlRoot struct {
	XMLName      xml.Name
	Version      string    `xml:"Version,attr"`
	DataServices xmlData   `xml:"DataServices"`
	Schema       xmlSchema `xml:"Schema"`
}

func (x xmlRoot) Attr(name string) string {
	if name == "Version" {
		return x.Version
	}
	return ""
}

type xmlData struct {
	Schemas []xmlSchema `xml:"Schema"`
}

type xmlSchema struct {
	Namespace        string               `xml:"Namespace,attr"`
	Alias            string               `xml:"Alias,attr"`
	EntityContainers []xmlEntityContainer `xml:"EntityContainer"`
	EntityTypes      []xmlStructuredType  `xml:"EntityType"`
	ComplexTypes     []xmlStructuredType  `xml:"ComplexType"`
	EnumTypes        []xmlEnumType        `xml:"EnumType"`
	Functions        []xmlOperation       `xml:"Function"`
	Actions          []xmlOperation       `xml:"Action"`
	Annotations      []xmlAnnotation      `xml:"Annotation"`
}

type xmlEntityContainer struct {
	Name            string              `xml:"Name,attr"`
	EntitySets      []xmlEntitySet      `xml:"EntitySet"`
	Singletons      []xmlSingleton      `xml:"Singleton"`
	FunctionImports []xmlFunctionImport `xml:"FunctionImport"`
	ActionImports   []xmlActionImport   `xml:"ActionImport"`
	Annotations     []xmlAnnotation     `xml:"Annotation"`
}

type xmlEntitySet struct {
	Name                       string                         `xml:"Name,attr"`
	EntityType                 string                         `xml:"EntityType,attr"`
	IncludeInServiceDocument   string                         `xml:"IncludeInServiceDocument,attr"`
	NavigationPropertyBindings []xmlNavigationPropertyBinding `xml:"NavigationPropertyBinding"`
	Annotations                []xmlAnnotation                `xml:"Annotation"`
}

type xmlSingleton struct {
	Name                       string                         `xml:"Name,attr"`
	Type                       string                         `xml:"Type,attr"`
	NavigationPropertyBindings []xmlNavigationPropertyBinding `xml:"NavigationPropertyBinding"`
	Annotations                []xmlAnnotation                `xml:"Annotation"`
}

type xmlNavigationPropertyBinding struct {
	Path   string `xml:"Path,attr"`
	Target string `xml:"Target,attr"`
}

type xmlFunctionImport struct {
	Name                     string          `xml:"Name,attr"`
	Function                 string          `xml:"Function,attr"`
	EntitySet                string          `xml:"EntitySet,attr"`
	IncludeInServiceDocument string          `xml:"IncludeInServiceDocument,attr"`
	Annotations              []xmlAnnotation `xml:"Annotation"`
}

type xmlActionImport struct {
	Name        string          `xml:"Name,attr"`
	Action      string          `xml:"Action,attr"`
	EntitySet   string          `xml:"EntitySet,attr"`
	Annotations []xmlAnnotation `xml:"Annotation"`
}

type xmlStructuredType struct {
	Name                 string                  `xml:"Name,attr"`
	BaseType             string                  `xml:"BaseType,attr"`
	Abstract             string                  `xml:"Abstract,attr"`
	OpenType             string                  `xml:"OpenType,attr"`
	HasStream            string                  `xml:"HasStream,attr"`
	Key                  xmlKey                  `xml:"Key"`
	Properties           []xmlProperty           `xml:"Property"`
	NavigationProperties []xmlNavigationProperty `xml:"NavigationProperty"`
	Annotations          []xmlAnnotation         `xml:"Annotation"`
}

type xmlKey struct {
	PropertyRefs []xmlPropertyRef `xml:"PropertyRef"`
}

type xmlPropertyRef struct {
	Name string `xml:"Name,attr"`
}

type xmlProperty struct {
	Name         string          `xml:"Name,attr"`
	Type         string          `xml:"Type,attr"`
	Nullable     string          `xml:"Nullable,attr"`
	MaxLength    string          `xml:"MaxLength,attr"`
	Precision    string          `xml:"Precision,attr"`
	Scale        string          `xml:"Scale,attr"`
	SRID         string          `xml:"SRID,attr"`
	DefaultValue string          `xml:"DefaultValue,attr"`
	Annotations  []xmlAnnotation `xml:"Annotation"`
}

type xmlNavigationProperty struct {
	Name                   string                     `xml:"Name,attr"`
	Type                   string                     `xml:"Type,attr"`
	Nullable               string                     `xml:"Nullable,attr"`
	Partner                string                     `xml:"Partner,attr"`
	ContainsTarget         string                     `xml:"ContainsTarget,attr"`
	ReferentialConstraints []xmlReferentialConstraint `xml:"ReferentialConstraint"`
	Annotations            []xmlAnnotation            `xml:"Annotation"`
}

type xmlReferentialConstraint struct {
	Property           string          `xml:"Property,attr"`
	ReferencedProperty string          `xml:"ReferencedProperty,attr"`
	Annotations        []xmlAnnotation `xml:"Annotation"`
}

type xmlEnumType struct {
	Name           string          `xml:"Name,attr"`
	UnderlyingType string          `xml:"UnderlyingType,attr"`
	IsFlags        string          `xml:"IsFlags,attr"`
	Members        []xmlEnumMember `xml:"Member"`
	Annotations    []xmlAnnotation `xml:"Annotation"`
}

type xmlEnumMember struct {
	Name        string          `xml:"Name,attr"`
	Value       string          `xml:"Value,attr"`
	Annotations []xmlAnnotation `xml:"Annotation"`
}

type xmlOperation struct {
	Name          string          `xml:"Name,attr"`
	IsBound       string          `xml:"IsBound,attr"`
	EntitySetPath string          `xml:"EntitySetPath,attr"`
	Parameters    []xmlParameter  `xml:"Parameter"`
	ReturnType    *xmlReturnType  `xml:"ReturnType"`
	Annotations   []xmlAnnotation `xml:"Annotation"`
}

type xmlParameter struct {
	Name        string          `xml:"Name,attr"`
	Type        string          `xml:"Type,attr"`
	Nullable    string          `xml:"Nullable,attr"`
	Annotations []xmlAnnotation `xml:"Annotation"`
}

type xmlReturnType struct {
	Type        string          `xml:"Type,attr"`
	Nullable    string          `xml:"Nullable,attr"`
	Annotations []xmlAnnotation `xml:"Annotation"`
}

type xmlAnnotation struct {
	Term      string `xml:"Term,attr"`
	Qualifier string `xml:"Qualifier,attr"`
	String    string `xml:"String,attr"`
	Bool      string `xml:"Bool,attr"`
	Int       string `xml:"Int,attr"`
	Decimal   string `xml:"Decimal,attr"`
	Path      string `xml:"Path,attr"`
}
