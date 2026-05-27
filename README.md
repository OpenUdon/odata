# OData

Dependency-light Go metadata package for OData service metadata artifacts.

`github.com/OpenUdon/odata` parses OData CSDL XML and JSON metadata into
native metadata for downstream authoring, packaging, validation, and review
tools. The package is intentionally metadata-first: it preserves entity sets,
singletons, entity types, complex types, enum types, properties, navigation
properties, functions, actions, annotations, and query-relevant metadata
without executing OData requests.

## Install

```bash
go get github.com/OpenUdon/odata
```

## Scope

This package is metadata-only. It does not call OData services, resolve tenant
credentials, execute queries, or fetch remote metadata.

## Example

```go
package main

import (
	"fmt"
	"os"

	"github.com/OpenUdon/odata"
)

func main() {
	data, err := os.ReadFile("metadata.xml")
	if err != nil {
		panic(err)
	}

	model, err := odata.Parse(data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%d schemas\n", len(model.Schemas))
	if book, ok := model.EntityTypeByName("Library.Book"); ok {
		fmt.Printf("%s has %d properties\n", book.FullName, len(book.Properties))
	}
}
```

## API

- `Parse` parses CSDL XML or CSDL JSON bytes.
- `ParseXML` parses CSDL XML bytes.
- `ParseJSON` parses CSDL JSON bytes.
- `ParseJSONMap` parses an already-decoded CSDL JSON object.
- `EntityTypeByName`, `ComplexTypeByName`, and `EnumTypeByName` look up parsed
  types by fully-qualified OData name.

Selector APIs and consumer-readiness examples are tracked in later milestones
in `memory-bank/milestone.md`.

## Verification

```bash
GOWORK=off go test ./...
GOWORK=off go vet ./...
git diff --check
```
