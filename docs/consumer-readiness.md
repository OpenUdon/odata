# Consumer Readiness

`odata` is ready for source-aware consumers that need local OData CSDL metadata
without calling OData services.

For UWS 1.4 planning evidence, see
[UWS Source-Profile Evidence](uws-source-profile-evidence.md).

## Stable API Surface

The current consumer-facing surface is:

- `Parse`, `ParseXML`, `ParseJSON`, and `ParseJSONMap` for local CSDL parsing.
- `Model`, `Schema`, `EntityContainer`, `EntitySet`, `Singleton`,
  `StructuredType`, `Property`, `NavigationProperty`, `EnumType`,
  `Operation`, `OperationSummary`, `Parameter`, `ReturnType`, `Annotation`,
  and `SelectorTarget` for native OData metadata.
- `EntityTypeByName`, `ComplexTypeByName`, `EnumTypeByName`,
  `EntityContainerByName`, `OperationByID`, `SelectorAliases`, and
  `ResolveSelector` for lookups and source-aware binding.

Selectors are intentionally local and metadata-only:

- canonical operation IDs such as `entitySet.Books.query`,
  `function.Default.SearchBooks`, and `action.Library.CheckOutBook`;
- operation JSON Pointers: `#/operations/{id}`;
- container, entity set, singleton, type, function, and action JSON Pointers.

JSON Pointer tokens use standard `~0` and `~1` escaping.

## apitools Readiness

`apitools` can classify local CSDL XML and CSDL JSON artifacts as native OData
source metadata by calling this package directly. A thin adapter should:

- parse local bytes with `Parse`, `ParseXML`, or `ParseJSON`;
- report source kind, schema namespaces, entity containers, entity set IDs,
  function/action IDs, type counts, annotation counts, and selector aliases;
- treat parse errors as artifact review diagnostics;
- keep OData artifacts distinct from OpenAPI, Discovery, Smithy, AsyncAPI,
  GraphQL, gRPC/protobuf, and advisory human-doc overlays.

The adapter must not generate a generic REST/OpenAPI overlay merely to make
OData fit a REST-shaped catalog entry. Any REST/OpenAPI lowering must be owned
by a downstream adapter that explicitly defines the conversion semantics.

## OpenUdon Readiness

OpenUdon can use this package after UWS OData source-profile semantics are
scoped. The stable metadata needed for fixture planning is present:

- source operation IDs for entity reads, entity queries, functions, and
  actions;
- local selectors to operation, container, entity set, singleton, type,
  function, and action metadata;
- key, navigation, parameter, return type, and annotation metadata for request
  mapping review;
- no-execution parser behavior suitable for review evidence.

OpenUdon should continue to own workflow artifacts, package review evidence,
approval routing, UWS validation wrappers, and trusted-runner handoff. This
package only parses and indexes local CSDL metadata.

## Boundary

This package does not:

- call OData endpoints or execute queries;
- fetch remote `$metadata` documents or discover tenants;
- resolve endpoints, accounts, auth, tokens, or credentials;
- generate UWS documents, OpenAPI documents, catalog rows, or OpenUdon
  fixtures;
- lower OData semantics into generic REST runtime calls.
