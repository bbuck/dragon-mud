# Pre-alpha!

This project is in a pre-alpha phase. It's not ready for use or consumption. 
At such a time the basic features are implmented I'll create a v0 tag for
gopkg to use and remove this banner.

# Goals

I'm trying to simplify dynamic Neo4j querying by creating constructs to build
Cypher queries in Code in steps or all at once and allow working with Go 
structs. For example, a common usage may be aligning stucts to node labels in
Neo4j and so at it's simplest:

```go
type Person struct {
        ID int `talon:"id"`
        Firstname string `talon:"firstname"`
        Lastname string `talon:"lastname"`
}
p := Person{ID: 1}
db.Find(&p)
```

Which will query the database `MATCH (_ref1:Person {id: 1}) RETURN _ref1` and
once the command has executed you're free to use the properties freely.

The other thing is building Cypher queries in code:

```go
db.Match(
        Node()
                .Named("a")
                .Labeled("Person")
                .WithProperties(talon.Properties{
                        "id": 1,
                })
).Retrun("a").String()
```

Produces a similar query as fetching the raw struct: `MATCH (a:Person {id: 1}) RETURN a`.

From the point of having a query you can easily load single rows into structs: `query.LoadResult(&p)
or work with many rows:

```go
var people []Person
db.Match(Node().Named("a").Labeled("Person")).Return("a").LoadResults(&people)
```

