# UWS Source-Profile Evidence

This note records non-normative OData evidence for later UWS 1.4 and OpenUdon
planning. It does not define UWS schema, validation rules, OpenUdon fixtures,
catalog rows, endpoint behavior, tenant discovery, credentials, or runtime
execution.

## Candidate Source Concepts

| Candidate Concept | `odata` Evidence |
|---|---|
| Source artifact type | Local CSDL XML or CSDL JSON parsed by `Parse`, `ParseXML`, `ParseJSON`, or `ParseJSONMap`. |
| Source operation ID | Canonical operation summary IDs stored on `OperationSummary.ID`, such as `entitySet.Books.query` and `function.Default.SearchBooks`. |
| Source selector | Canonical operation IDs plus local JSON Pointer-style selectors returned by `SelectorAliases` and resolved by `ResolveSelector`. |
| Entity metadata | `EntityContainer`, `EntitySet`, `Singleton`, and `StructuredType` preserve container, entity, key, property, navigation, annotation, and return-type evidence. |
| Function/action metadata | `Operation`, `OperationImport`, `Parameter`, `ReturnType`, and `OperationSummary` preserve bound/unbound shape and request mapping metadata. |
| Query mapping evidence | Entity-set query summaries preserve return collection shape, navigation binding paths, and annotations; entity types preserve keys and properties. |

## Selector Shapes

Given entity container `Library.Default`, entity set `Books`, function import
`SearchBooks`, bound function `Library.RelatedBooks`, bound action
`Library.CheckOutBook`, and action import `ResetLibrary`, all examples below
are local metadata selectors only. They do not imply endpoint selection, query
execution, tenant discovery, credential lookup, or OData service calls.

| OData Shape | Canonical Operation ID | JSON Pointer Alias Or Metadata Selector |
|---|---|---|
| Entity read | `entitySet.Books.read` | `#/operations/entitySet.Books.read` |
| Collection query | `entitySet.Books.query` | `#/operations/entitySet.Books.query` |
| Navigation evidence | `entitySet.Books.query` with navigation path `Author` | `#/entityContainers/Library.Default/entitySets/Books` |
| Singleton read | `singleton.Me.read` | `#/operations/singleton.Me.read`, `#/entityContainers/Library.Default/singletons/Me` |
| Unbound function import | `function.Default.SearchBooks` | `#/operations/function.Default.SearchBooks`, `#/functions/Library.SearchBooks` |
| Bound function | `function.Library.RelatedBooks` | `#/operations/function.Library.RelatedBooks`, `#/functions/Library.RelatedBooks` |
| Bound action | `action.Library.CheckOutBook` | `#/operations/action.Library.CheckOutBook`, `#/actions/Library.CheckOutBook` |
| Unbound action import | `action.Default.ResetLibrary` | `#/operations/action.Default.ResetLibrary`, `#/actions/Library.ResetLibrary` |

Type metadata can also be selected with `#/entityTypes/Library.Book`,
`#/complexTypes/Library.Address`, and `#/enumTypes/Library.BookStatus`.

## Boundary Matrix

| Project | Owns | Does Not Own Here |
|---|---|---|
| `odata` | Local CSDL XML/JSON parsing, native OData metadata, operation IDs, selector aliases, request mapping evidence, parser examples, and evidence notes. | UWS schema changes, OpenUdon workflow fixtures, catalog rows, endpoint calls, tenant discovery, credential handling, or OData execution. |
| `apitools` | Catalog classification, artifact discovery/import/cache, provider metadata, ranking, summaries, and adapters that call `odata`. | Native CSDL parsing logic that belongs in `odata`, trusted execution, or UWS workflow semantics. |
| UWS | Normative workflow source description types, source-profile schema, validation rules, and public workflow semantics if OData graduates into UWS. | Parser implementation details, catalog storage, OpenUdon package review behavior, or runtime execution. |
| OpenUdon | Workflow/package fixtures, review evidence, approval templates, validation wrappers, source-aware synthesis, and trusted-runner handoff. | Low-level CSDL parsing, catalog discovery/cache, UWS normative schema, or private runtime execution. |
| Trusted runtimes | Concrete OData execution, endpoint/account/tenant selection, auth setup, retries, query/action invocation, and side effects after approval. | Planning-only metadata parsing, catalog classification, UWS schema definition, or OpenUdon review artifacts. |

## Compatibility Expectations

Before downstream UWS 1.4 source-profile work relies on this package, treat
the M4 public surface as the compatibility baseline:

- parser entrypoints: `Parse`, `ParseXML`, `ParseJSON`, and `ParseJSONMap`;
- metadata types: `Model`, `Schema`, `EntityContainer`, `EntitySet`,
  `Singleton`, `StructuredType`, `Property`, `NavigationProperty`, `EnumType`,
  `Operation`, `OperationImport`, `OperationSummary`, `Parameter`,
  `ReturnType`, `Annotation`, and `SelectorTarget`;
- lookup and selector methods: `EntityTypeByName`, `ComplexTypeByName`,
  `EnumTypeByName`, `EntityContainerByName`, `OperationByID`,
  `SelectorAliases`, and `ResolveSelector`;
- selector shapes listed above;
- no-execution guarantees: no endpoint calls, query execution, tenant
  discovery, credential resolution, remote `$metadata` fetching, UWS
  generation, REST/OpenAPI lowering, catalog-row creation, OpenUdon fixture
  creation, or OData runtime behavior.

Breaking changes to this surface should be intentional, documented in the
memory bank, and reviewed against `apitools`, OpenUdon, and UWS planning needs.
