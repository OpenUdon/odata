package odata_test

import (
	"fmt"

	"github.com/OpenUdon/odata"
)

func ExampleParseXML() {
	model, err := odata.ParseXML([]byte(`<Schema Namespace="Example" xmlns="http://docs.oasis-open.org/odata/ns/edm">
  <EntityContainer Name="Default">
    <EntitySet Name="Books" EntityType="Example.Book" />
  </EntityContainer>
  <EntityType Name="Book">
    <Key><PropertyRef Name="ID" /></Key>
    <Property Name="ID" Type="Edm.String" Nullable="false" />
  </EntityType>
</Schema>`))
	if err != nil {
		panic(err)
	}
	fmt.Println(model.Schemas[0].Namespace)
	fmt.Println(model.Operations[0].ID)
	// Output:
	// Example
	// entitySet.Books.read
}

func ExampleParseJSON() {
	model, err := odata.ParseJSON([]byte(`{
  "$Version": "4.01",
  "Example": {
    "Default": {
      "$Kind": "EntityContainer",
      "Books": {"$Type": "Example.Book"}
    },
    "Book": {
      "$Kind": "EntityType",
      "$Key": ["ID"],
      "ID": {"$Type": "Edm.String", "$Nullable": false}
    }
  }
}`))
	if err != nil {
		panic(err)
	}
	book, _ := model.EntityTypeByName("Example.Book")
	fmt.Println(model.Version)
	fmt.Println(len(book.Properties))
	// Output:
	// 4.01
	// 1
}

func ExampleModel_OperationByID() {
	model, err := odata.ParseXML([]byte(`<Schema Namespace="Example" xmlns="http://docs.oasis-open.org/odata/ns/edm">
  <EntityContainer Name="Default">
    <EntitySet Name="Books" EntityType="Example.Book" />
  </EntityContainer>
  <EntityType Name="Book" />
</Schema>`))
	if err != nil {
		panic(err)
	}
	op, ok := model.OperationByID("entitySet.Books.query")
	fmt.Println(ok)
	fmt.Println(op.ReturnType.Collection)
	// Output:
	// true
	// true
}

func ExampleModel_ResolveSelector() {
	model, err := odata.ParseXML([]byte(`<Schema Namespace="Example" xmlns="http://docs.oasis-open.org/odata/ns/edm">
  <EntityContainer Name="Default">
    <EntitySet Name="Books" EntityType="Example.Book" />
  </EntityContainer>
  <EntityType Name="Book" />
</Schema>`))
	if err != nil {
		panic(err)
	}
	target, err := model.ResolveSelector("#/entityContainers/Example.Default/entitySets/Books")
	if err != nil {
		panic(err)
	}
	fmt.Println(target.Kind)
	fmt.Println(target.EntitySet.EntityType)
	// Output:
	// entitySet
	// Example.Book
}
