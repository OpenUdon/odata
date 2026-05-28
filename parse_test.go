package odata_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/OpenUdon/odata"
)

func TestParseXMLFixture(t *testing.T) {
	model, err := odata.ParseXML(readFixture(t, "testdata/library.xml"))
	if err != nil {
		t.Fatalf("ParseXML failed: %v", err)
	}
	if model.SourceKind != odata.SourceKindXML {
		t.Fatalf("source kind = %q", model.SourceKind)
	}
	if model.Version != "4.0" || model.RawXML == "" {
		t.Fatalf("version/raw XML = %q/%t", model.Version, model.RawXML != "")
	}
	assertLibraryModel(t, model, "4.0")
}

func TestParseJSONFixture(t *testing.T) {
	model, err := odata.ParseJSON(readFixture(t, "testdata/library.json"))
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	if model.SourceKind != odata.SourceKindJSON {
		t.Fatalf("source kind = %q", model.SourceKind)
	}
	if model.RawJSON == nil {
		t.Fatal("RawJSON should preserve decoded CSDL JSON")
	}
	assertLibraryModel(t, model, "4.01")
}

func TestParseAutoDetectsSourceKind(t *testing.T) {
	if model, err := odata.Parse(readFixture(t, "testdata/library.xml")); err != nil || model.SourceKind != odata.SourceKindXML {
		t.Fatalf("XML Parse = %#v, %v", model, err)
	}
	if model, err := odata.Parse(readFixture(t, "testdata/library.json")); err != nil || model.SourceKind != odata.SourceKindJSON {
		t.Fatalf("JSON Parse = %#v, %v", model, err)
	}
}

func TestParseJSONMap(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal(readFixture(t, "testdata/library.json"), &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	model, err := odata.ParseJSONMap(raw)
	if err != nil {
		t.Fatalf("ParseJSONMap failed: %v", err)
	}
	if model.SourceKind != odata.SourceKindJSON || model.EntityContainer != "Library.Default" {
		t.Fatalf("model = %#v", model)
	}
}

func TestParseJSONDefaultsPropertyTypeToString(t *testing.T) {
	model, err := odata.ParseJSON([]byte(`{
		"$Version": "4.0",
		"Example": {
			"Book": {
				"$Kind": "EntityType",
				"$Key": ["ID"],
				"ID": {"$Type": "Edm.Int32"},
				"Title": {}
			}
		}
	}`))
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	book, ok := model.EntityTypeByName("Example.Book")
	if !ok {
		t.Fatal("Example.Book missing")
	}
	if len(book.Properties) != 2 || book.Properties[1].Name != "Title" || book.Properties[1].Type != "Edm.String" {
		t.Fatalf("book properties = %#v", book.Properties)
	}
}

func TestTypeLookupMisses(t *testing.T) {
	model, err := odata.ParseXML(readFixture(t, "testdata/library.xml"))
	if err != nil {
		t.Fatalf("ParseXML failed: %v", err)
	}
	if _, ok := model.EntityTypeByName(""); ok {
		t.Fatal("empty entity type unexpectedly resolved")
	}
	if _, ok := model.EntityTypeByName("Missing.Type"); ok {
		t.Fatal("missing entity type unexpectedly resolved")
	}
	if _, ok := model.ComplexTypeByName("Missing.Type"); ok {
		t.Fatal("missing complex type unexpectedly resolved")
	}
	if _, ok := model.EnumTypeByName("Missing.Type"); ok {
		t.Fatal("missing enum type unexpectedly resolved")
	}
	var nilModel *odata.Model
	if _, ok := nilModel.EntityTypeByName("Library.Book"); ok {
		t.Fatal("nil model unexpectedly resolved entity type")
	}
}

func TestOperationSummariesAndSelectors(t *testing.T) {
	model, err := odata.ParseXML(readFixture(t, "testdata/library.xml"))
	if err != nil {
		t.Fatalf("ParseXML failed: %v", err)
	}
	wantIDs := []string{
		"entitySet.Books.read",
		"entitySet.Books.query",
		"singleton.Me.read",
		"function.Default.SearchBooks",
		"action.Default.ResetLibrary",
		"function.Library.RelatedBooks",
		"action.Library.CheckOutBook",
	}
	if len(model.Operations) != len(wantIDs) {
		t.Fatalf("operation count = %d, want %d: %#v", len(model.Operations), len(wantIDs), model.Operations)
	}
	for i, want := range wantIDs {
		if model.Operations[i].ID != want {
			t.Fatalf("operation[%d] = %q, want %q", i, model.Operations[i].ID, want)
		}
	}
	booksQuery, ok := model.OperationByID("entitySet.Books.query")
	if !ok {
		t.Fatal("Books query operation missing")
	}
	if !booksQuery.QueryRelevant || booksQuery.ReturnType == nil || booksQuery.ReturnType.Type != "Library.Book" || !booksQuery.ReturnType.Collection {
		t.Fatalf("Books query = %#v", booksQuery)
	}
	if len(booksQuery.NavigationPaths) != 1 || booksQuery.NavigationPaths[0] != "Author" {
		t.Fatalf("Books query navigation paths = %#v", booksQuery.NavigationPaths)
	}
	search, ok := model.OperationByID("function.Default.SearchBooks")
	if !ok || search.Operation != "Library.SearchBooks" || len(search.Parameters) != 1 || search.Parameters[0].Name != "term" {
		t.Fatalf("SearchBooks summary = %#v, %v", search, ok)
	}
	related, ok := model.OperationByID("function.Library.RelatedBooks")
	if !ok || !related.Bound || related.EntitySetPath != "bindingParameter" {
		t.Fatalf("RelatedBooks summary = %#v, %v", related, ok)
	}
	checkout, ok := model.OperationByID("action.Library.CheckOutBook")
	if !ok || !checkout.Bound || len(checkout.Parameters) != 2 {
		t.Fatalf("CheckOutBook summary = %#v, %v", checkout, ok)
	}
	container, ok := model.EntityContainerByName("Library.Default")
	if !ok || container.Name != "Default" {
		t.Fatalf("container lookup = %#v, %v", container, ok)
	}

	aliases := model.SelectorAliases()
	for _, want := range []string{
		"entitySet.Books.query",
		"#/operations/entitySet.Books.query",
		"#/entityContainers/Library.Default",
		"#/entityContainers/Library.Default/entitySets/Books",
		"#/entityContainers/Library.Default/singletons/Me",
		"#/entityTypes/Library.Book",
		"#/complexTypes/Library.Address",
		"#/enumTypes/Library.BookStatus",
		"#/functions/Library.SearchBooks",
		"#/actions/Library.CheckOutBook",
	} {
		if !containsString(aliases, want) {
			t.Fatalf("aliases missing %q: %#v", want, aliases)
		}
	}
	assertSelector(t, model, "entitySet.Books.query", odata.SelectorKindOperation)
	assertSelector(t, model, "#/operations/function.Default.SearchBooks", odata.SelectorKindOperation)
	assertSelector(t, model, "#/entityContainers/Library.Default", odata.SelectorKindEntityContainer)
	assertSelector(t, model, "#/entityContainers/Library.Default/entitySets/Books", odata.SelectorKindEntitySet)
	assertSelector(t, model, "#/entityContainers/Library.Default/singletons/Me", odata.SelectorKindSingleton)
	assertSelector(t, model, "#/entityTypes/Library.Book", odata.SelectorKindEntityType)
	assertSelector(t, model, "#/complexTypes/Library.Address", odata.SelectorKindComplexType)
	assertSelector(t, model, "#/enumTypes/Library.BookStatus", odata.SelectorKindEnumType)
	assertSelector(t, model, "#/functions/Library.SearchBooks", odata.SelectorKindFunction)
	assertSelector(t, model, "#/actions/Library.CheckOutBook", odata.SelectorKindAction)
}

func TestSelectorMisses(t *testing.T) {
	model, err := odata.ParseJSON(readFixture(t, "testdata/library.json"))
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	cases := []string{
		"",
		"entitySet.Books.delete",
		"#/operations/entitySet.Books.delete",
		"#/entityContainers/Library.Missing",
		"#/entityContainers/Library.Default/entitySets/Missing",
		"#/entityContainers/Library.Default/singletons/Missing",
		"#/entityTypes/Library.Missing",
		"#/complexTypes/Library.Missing",
		"#/enumTypes/Library.Missing",
		"#/functions/Library.Missing",
		"#/actions/Library.Missing",
		"#/operations/entitySet.Books~2query",
	}
	for _, selector := range cases {
		if target, err := model.ResolveSelector(selector); err == nil {
			t.Fatalf("selector %q unexpectedly resolved to %#v", selector, target)
		}
	}
	var nilModel *odata.Model
	if aliases := nilModel.SelectorAliases(); aliases != nil {
		t.Fatalf("nil aliases = %#v", aliases)
	}
	if _, ok := nilModel.OperationByID("entitySet.Books.query"); ok {
		t.Fatal("nil model unexpectedly resolved operation")
	}
	if _, ok := nilModel.EntityContainerByName("Library.Default"); ok {
		t.Fatal("nil model unexpectedly resolved container")
	}
	if _, err := nilModel.ResolveSelector("entitySet.Books.query"); err == nil {
		t.Fatal("nil model unexpectedly resolved selector")
	}
}

func TestOperationOverloadIDs(t *testing.T) {
	model, err := odata.ParseXML([]byte(`<Schema Namespace="Okay" xmlns="http://docs.oasis-open.org/odata/ns/edm"><Function Name="Search"><Parameter Name="term" Type="Edm.String" /></Function><Function Name="Search"><Parameter Name="term" Type="Edm.String" /><Parameter Name="limit" Type="Edm.Int32" /></Function></Schema>`))
	if err != nil {
		t.Fatalf("ParseXML overloads failed: %v", err)
	}
	if _, ok := model.OperationByID("function.Okay.Search(term)"); !ok {
		t.Fatal("single-parameter overload missing")
	}
	if _, ok := model.OperationByID("function.Okay.Search(term,limit)"); !ok {
		t.Fatal("two-parameter overload missing")
	}
	aliases := model.SelectorAliases()
	if !containsString(aliases, "#/functions/Okay.Search") {
		t.Fatalf("function selector alias missing: %#v", aliases)
	}
	if countString(aliases, "#/functions/Okay.Search") != 1 {
		t.Fatalf("function selector alias duplicated: %#v", aliases)
	}
}

func TestMalformedInputs(t *testing.T) {
	cases := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "empty parse",
			run: func() error {
				_, err := odata.Parse(nil)
				return err
			},
			want: "empty",
		},
		{
			name: "unsupported root kind",
			run: func() error {
				_, err := odata.Parse([]byte(`not metadata`))
				return err
			},
			want: "must be XML or JSON",
		},
		{
			name: "invalid XML",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<edmx:Edmx>`))
				return err
			},
			want: "parse OData CSDL XML",
		},
		{
			name: "unsupported XML root",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<Metadata />`))
				return err
			},
			want: "unsupported OData CSDL XML root",
		},
		{
			name: "XML no schemas",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<edmx:Edmx Version="4.0" xmlns:edmx="http://docs.oasis-open.org/odata/ns/edmx"><edmx:DataServices /></edmx:Edmx>`))
				return err
			},
			want: "contains no schemas",
		},
		{
			name: "trailing XML",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<Schema Namespace="Broken" xmlns="http://docs.oasis-open.org/odata/ns/edm" /> <Schema Namespace="Other" xmlns="http://docs.oasis-open.org/odata/ns/edm" />`))
				return err
			},
			want: "trailing data",
		},
		{
			name: "XML missing namespace",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<Schema xmlns="http://docs.oasis-open.org/odata/ns/edm"><EntityType Name="Book" /></Schema>`))
				return err
			},
			want: "namespace is required",
		},
		{
			name: "XML missing property type",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<Schema Namespace="Broken" xmlns="http://docs.oasis-open.org/odata/ns/edm"><EntityType Name="Book"><Property Name="ID" /></EntityType></Schema>`))
				return err
			},
			want: "empty type",
		},
		{
			name: "invalid JSON",
			run: func() error {
				_, err := odata.ParseJSON([]byte(`{`))
				return err
			},
			want: "parse OData CSDL JSON",
		},
		{
			name: "trailing JSON",
			run: func() error {
				_, err := odata.ParseJSON([]byte(`{"$Version":"4.01"} {"$Version":"4.01"}`))
				return err
			},
			want: "trailing data",
		},
		{
			name: "nil JSON map",
			run: func() error {
				_, err := odata.ParseJSONMap(nil)
				return err
			},
			want: "root must be an object",
		},
		{
			name: "JSON no schemas",
			run: func() error {
				_, err := odata.ParseJSON([]byte(`{"$Version":"4.01"}`))
				return err
			},
			want: "contains no schemas",
		},
		{
			name: "JSON bad overload",
			run: func() error {
				_, err := odata.ParseJSON([]byte(`{"Broken":{"Search":[1]}}`))
				return err
			},
			want: "overload must be an object",
		},
		{
			name: "duplicate entity type",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<Schema Namespace="Broken" xmlns="http://docs.oasis-open.org/odata/ns/edm"><EntityType Name="Book" /><EntityType Name="Book" /></Schema>`))
				return err
			},
			want: "duplicate OData entity type",
		},
		{
			name: "duplicate operation summary",
			run: func() error {
				_, err := odata.ParseXML([]byte(`<Schema Namespace="Broken" xmlns="http://docs.oasis-open.org/odata/ns/edm"><EntityContainer Name="Default"><EntitySet Name="Books" EntityType="Broken.Book" /><EntitySet Name="Books" EntityType="Broken.Book" /></EntityContainer><EntityType Name="Book" /></Schema>`))
				return err
			},
			want: "duplicate OData operation summary",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func assertLibraryModel(t *testing.T, model *odata.Model, version string) {
	t.Helper()
	if model.Version != version {
		t.Fatalf("version = %q", model.Version)
	}
	if len(model.Schemas) != 1 {
		t.Fatalf("schemas = %d", len(model.Schemas))
	}
	schema := model.Schemas[0]
	if schema.Namespace != "Library" || schema.Alias != "Lib" {
		t.Fatalf("schema = %#v", schema)
	}
	if len(schema.EntityContainers) != 1 {
		t.Fatalf("containers = %d", len(schema.EntityContainers))
	}
	container := schema.EntityContainers[0]
	if container.FullName != "Library.Default" || len(container.EntitySets) != 1 || len(container.Singletons) != 1 {
		t.Fatalf("container = %#v", container)
	}
	books := container.EntitySets[0]
	if books.Name != "Books" || books.EntityType != "Library.Book" || !books.IncludeInServiceDocument {
		t.Fatalf("Books = %#v", books)
	}
	if len(books.NavigationPropertyBindings) != 1 || books.NavigationPropertyBindings[0].Path != "Author" || books.NavigationPropertyBindings[0].Target != "Authors" {
		t.Fatalf("nav bindings = %#v", books.NavigationPropertyBindings)
	}
	if len(books.Annotations) != 1 || books.Annotations[0].Term != "Org.OData.Capabilities.V1.SearchRestrictions" {
		t.Fatalf("book annotations = %#v", books.Annotations)
	}
	if got := container.Singletons[0]; got.Name != "Me" || got.Type != "Library.Author" {
		t.Fatalf("singleton = %#v", got)
	}
	if len(container.FunctionImports) != 1 || container.FunctionImports[0].Operation != "Library.SearchBooks" {
		t.Fatalf("function imports = %#v", container.FunctionImports)
	}
	if len(container.ActionImports) != 1 || container.ActionImports[0].Operation != "Library.ResetLibrary" {
		t.Fatalf("action imports = %#v", container.ActionImports)
	}

	book, ok := model.EntityTypeByName(".Library.Book")
	if !ok {
		t.Fatal("Library.Book missing")
	}
	if !book.HasStream || len(book.Key) != 1 || book.Key[0] != "ID" {
		t.Fatalf("book type = %#v", book)
	}
	id := propertyByName(book.Properties, "ID")
	if id == nil || id.Type != "Edm.String" || id.Nullable {
		t.Fatalf("ID property = %#v", id)
	}
	tags := propertyByName(book.Properties, "Tags")
	if tags == nil || tags.Type != "Edm.String" || !tags.Collection {
		t.Fatalf("Tags property = %#v", tags)
	}
	authorNav := navigationByName(book.NavigationProperties, "Author")
	if authorNav == nil || authorNav.Type != "Library.Author" || authorNav.Nullable || authorNav.Partner != "Books" {
		t.Fatalf("Author nav = %#v", authorNav)
	}
	if len(authorNav.ReferentialConstraints) != 1 || authorNav.ReferentialConstraints[0].Property != "AuthorID" {
		t.Fatalf("constraints = %#v", authorNav.ReferentialConstraints)
	}
	if _, ok := model.ComplexTypeByName("Library.Address"); !ok {
		t.Fatal("Library.Address missing")
	}
	enum, ok := model.EnumTypeByName("Library.BookStatus")
	if !ok || len(enum.Members) != 3 || enumMemberByName(enum.Members, "CheckedOut") == nil {
		t.Fatalf("enum = %#v", enum)
	}
	if len(model.Functions["Library.SearchBooks"]) != 1 {
		t.Fatalf("functions = %#v", model.Functions)
	}
	search := model.Functions["Library.SearchBooks"][0]
	if search.IsBound || search.Parameters[0].Name != "term" || search.ReturnType == nil || search.ReturnType.Type != "Library.Book" || !search.ReturnType.Collection {
		t.Fatalf("SearchBooks = %#v", search)
	}
	related := model.Functions["Library.RelatedBooks"][0]
	if !related.IsBound || related.EntitySetPath != "bindingParameter" {
		t.Fatalf("RelatedBooks = %#v", related)
	}
	checkout := model.Actions["Library.CheckOutBook"][0]
	if !checkout.IsBound || checkout.ReturnType == nil || checkout.ReturnType.Type != "Library.Book" {
		t.Fatalf("CheckOutBook = %#v", checkout)
	}
	if len(model.Actions["Library.ResetLibrary"]) != 1 {
		t.Fatalf("ResetLibrary missing: %#v", model.Actions)
	}
}

func propertyByName(properties []*odata.Property, name string) *odata.Property {
	for _, property := range properties {
		if property != nil && property.Name == name {
			return property
		}
	}
	return nil
}

func navigationByName(properties []*odata.NavigationProperty, name string) *odata.NavigationProperty {
	for _, property := range properties {
		if property != nil && property.Name == name {
			return property
		}
	}
	return nil
}

func assertSelector(t *testing.T, model *odata.Model, selector, kind string) {
	t.Helper()
	target, err := model.ResolveSelector(selector)
	if err != nil {
		t.Fatalf("ResolveSelector(%q) failed: %v", selector, err)
	}
	if target.Kind != kind {
		t.Fatalf("ResolveSelector(%q) kind = %q, want %q; target=%#v", selector, target.Kind, kind, target)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func enumMemberByName(members []*odata.EnumMember, name string) *odata.EnumMember {
	for _, member := range members {
		if member != nil && member.Name == name {
			return member
		}
	}
	return nil
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	return data
}
