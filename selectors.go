package odata

import (
	"fmt"
	"strings"
)

const (
	// SelectorKindOperation marks a selector target that resolves to operation
	// summary metadata.
	SelectorKindOperation = "operation"
	// SelectorKindEntityContainer marks a selector target that resolves to an
	// entity container.
	SelectorKindEntityContainer = "entityContainer"
	// SelectorKindEntitySet marks a selector target that resolves to an entity
	// set.
	SelectorKindEntitySet = "entitySet"
	// SelectorKindSingleton marks a selector target that resolves to a
	// singleton.
	SelectorKindSingleton = "singleton"
	// SelectorKindFunction marks a selector target that resolves to a function
	// definition.
	SelectorKindFunction = "function"
	// SelectorKindAction marks a selector target that resolves to an action
	// definition.
	SelectorKindAction = "action"
	// SelectorKindEntityType marks a selector target that resolves to an entity
	// type.
	SelectorKindEntityType = "entityType"
	// SelectorKindComplexType marks a selector target that resolves to a complex
	// type.
	SelectorKindComplexType = "complexType"
	// SelectorKindEnumType marks a selector target that resolves to an enum
	// type.
	SelectorKindEnumType = "enumType"
)

// SelectorTarget describes local OData metadata selected by an operation ID or
// JSON Pointer-style fragment.
type SelectorTarget struct {
	Kind            string
	Selector        string
	Operation       *OperationSummary
	EntityContainer *EntityContainer
	EntitySet       *EntitySet
	Singleton       *Singleton
	Function        *Operation
	Action          *Operation
	StructuredType  *StructuredType
	EnumType        *EnumType
}

// SelectorAliases returns canonical operation IDs and local JSON Pointer-style
// selectors in deterministic model order.
func (m *Model) SelectorAliases() []string {
	if m == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(alias string) {
		if alias == "" || seen[alias] {
			return
		}
		seen[alias] = true
		out = append(out, alias)
	}
	for _, op := range m.Operations {
		if op == nil {
			continue
		}
		add(op.ID)
		add("#/operations/" + escapePointerToken(op.ID))
	}
	for _, schema := range m.Schemas {
		for _, container := range schema.EntityContainers {
			if container == nil {
				continue
			}
			containerRef := "#/entityContainers/" + escapePointerToken(container.FullName)
			add(containerRef)
			for _, set := range container.EntitySets {
				if set != nil {
					add(containerRef + "/entitySets/" + escapePointerToken(set.Name))
				}
			}
			for _, singleton := range container.Singletons {
				if singleton != nil {
					add(containerRef + "/singletons/" + escapePointerToken(singleton.Name))
				}
			}
		}
		for _, typ := range schema.EntityTypes {
			if typ != nil {
				add("#/entityTypes/" + escapePointerToken(typ.FullName))
			}
		}
		for _, typ := range schema.ComplexTypes {
			if typ != nil {
				add("#/complexTypes/" + escapePointerToken(typ.FullName))
			}
		}
		for _, enum := range schema.EnumTypes {
			if enum != nil {
				add("#/enumTypes/" + escapePointerToken(enum.FullName))
			}
		}
		for _, function := range schema.Functions {
			if function != nil {
				add("#/functions/" + escapePointerToken(function.FullName))
			}
		}
		for _, action := range schema.Actions {
			if action != nil {
				add("#/actions/" + escapePointerToken(action.FullName))
			}
		}
	}
	return out
}

// ResolveSelector resolves a canonical operation ID or local JSON
// Pointer-style selector to OData metadata.
func (m *Model) ResolveSelector(selector string) (*SelectorTarget, error) {
	if m == nil {
		return nil, fmt.Errorf("OData selector %q cannot resolve against nil model", selector)
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("OData selector is empty")
	}
	if op, ok := m.OperationByID(selector); ok {
		return &SelectorTarget{Kind: SelectorKindOperation, Selector: selector, Operation: op}, nil
	}
	if !strings.HasPrefix(selector, "#/") {
		return nil, fmt.Errorf("OData selector %q not found", selector)
	}
	tokens, err := pointerTokens(selector)
	if err != nil {
		return nil, err
	}
	switch {
	case len(tokens) == 2 && tokens[0] == "operations":
		op, ok := m.OperationByID(tokens[1])
		if !ok {
			return nil, fmt.Errorf("OData operation selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindOperation, Selector: selector, Operation: op}, nil
	case len(tokens) == 2 && tokens[0] == "entityContainers":
		container, ok := m.EntityContainerByName(tokens[1])
		if !ok {
			return nil, fmt.Errorf("OData entity container selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindEntityContainer, Selector: selector, EntityContainer: container}, nil
	case len(tokens) == 4 && tokens[0] == "entityContainers" && tokens[2] == "entitySets":
		container, set := m.findEntitySet(tokens[1], tokens[3])
		if set == nil {
			return nil, fmt.Errorf("OData entity set selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindEntitySet, Selector: selector, EntityContainer: container, EntitySet: set}, nil
	case len(tokens) == 4 && tokens[0] == "entityContainers" && tokens[2] == "singletons":
		container, singleton := m.findSingleton(tokens[1], tokens[3])
		if singleton == nil {
			return nil, fmt.Errorf("OData singleton selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindSingleton, Selector: selector, EntityContainer: container, Singleton: singleton}, nil
	case len(tokens) == 2 && tokens[0] == "entityTypes":
		typ, ok := m.EntityTypeByName(tokens[1])
		if !ok {
			return nil, fmt.Errorf("OData entity type selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindEntityType, Selector: selector, StructuredType: typ}, nil
	case len(tokens) == 2 && tokens[0] == "complexTypes":
		typ, ok := m.ComplexTypeByName(tokens[1])
		if !ok {
			return nil, fmt.Errorf("OData complex type selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindComplexType, Selector: selector, StructuredType: typ}, nil
	case len(tokens) == 2 && tokens[0] == "enumTypes":
		enum, ok := m.EnumTypeByName(tokens[1])
		if !ok {
			return nil, fmt.Errorf("OData enum type selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindEnumType, Selector: selector, EnumType: enum}, nil
	case len(tokens) == 2 && tokens[0] == "functions":
		op := firstOperation(m.Functions[normalizeName(tokens[1])])
		if op == nil {
			return nil, fmt.Errorf("OData function selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindFunction, Selector: selector, Function: op}, nil
	case len(tokens) == 2 && tokens[0] == "actions":
		op := firstOperation(m.Actions[normalizeName(tokens[1])])
		if op == nil {
			return nil, fmt.Errorf("OData action selector %q not found", selector)
		}
		return &SelectorTarget{Kind: SelectorKindAction, Selector: selector, Action: op}, nil
	default:
		return nil, fmt.Errorf("unsupported OData selector %q", selector)
	}
}

func (m *Model) findEntitySet(containerName, setName string) (*EntityContainer, *EntitySet) {
	container, ok := m.EntityContainerByName(containerName)
	if !ok {
		return nil, nil
	}
	for _, set := range container.EntitySets {
		if set != nil && set.Name == setName {
			return container, set
		}
	}
	return container, nil
}

func (m *Model) findSingleton(containerName, singletonName string) (*EntityContainer, *Singleton) {
	container, ok := m.EntityContainerByName(containerName)
	if !ok {
		return nil, nil
	}
	for _, singleton := range container.Singletons {
		if singleton != nil && singleton.Name == singletonName {
			return container, singleton
		}
	}
	return container, nil
}

func pointerTokens(selector string) ([]string, error) {
	raw := strings.Split(strings.TrimPrefix(selector, "#/"), "/")
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		decoded, err := unescapePointerToken(token)
		if err != nil {
			return nil, fmt.Errorf("invalid OData selector %q: %w", selector, err)
		}
		tokens = append(tokens, decoded)
	}
	return tokens, nil
}

func escapePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	token = strings.ReplaceAll(token, "/", "~1")
	return token
}

func unescapePointerToken(token string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(token); i++ {
		if token[i] != '~' {
			out.WriteByte(token[i])
			continue
		}
		if i+1 >= len(token) {
			return "", fmt.Errorf("dangling escape in pointer token")
		}
		switch token[i+1] {
		case '0':
			out.WriteByte('~')
		case '1':
			out.WriteByte('/')
		default:
			return "", fmt.Errorf("unknown escape ~%c in pointer token", token[i+1])
		}
		i++
	}
	return out.String(), nil
}
