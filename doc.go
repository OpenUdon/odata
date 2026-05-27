// Package odata provides metadata-only parsing support for OData service
// metadata artifacts.
//
// Parse, ParseXML, ParseJSON, and ParseJSONMap convert local CSDL XML or JSON
// metadata into native entity container, entity set, singleton, type,
// function, action, parameter, return type, property, navigation, and
// annotation metadata. OperationByID, SelectorAliases, and ResolveSelector
// expose selectable entity, function, and action operations without executing
// them. The package must not call OData services, execute queries, fetch tenant
// metadata, or resolve credentials.
package odata
