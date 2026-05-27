# OData

Dependency-light Go metadata package for OData service metadata artifacts.

`github.com/OpenUdon/odata` will parse OData CSDL XML and JSON metadata into
native metadata for downstream authoring, packaging, validation, and review
tools. The package is intentionally metadata-first: it preserves entity sets,
entity types, properties, navigation properties, functions, actions, and query
semantics without executing OData requests.

## Install

```bash
go get github.com/OpenUdon/odata
```

## Scope

This package is metadata-only. It does not call OData services, resolve tenant
credentials, execute queries, or fetch remote metadata.

The initial module contains the project harness and package boundary. Parser
APIs will be added by later milestones tracked in `memory-bank/milestone.md`.

## Verification

```bash
GOWORK=off go test ./...
GOWORK=off go vet ./...
git diff --check
```
