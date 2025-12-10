# database

database is a simple library that builds on top of [database/sql][] from the Go
standard library to provide modelling and query building. It aims to stay out of
your way as much as possible, and makes as few assumptions about the data you
are working with.

* [Quickstart](#quickstart)
* [Conventions](#conventions)
* [Models](#models)
  * [Parameters](#parameters)
  * [Field aliases](#field-aliases)
* [Stores](#stores)
  * [Creating models](#creating-models)
  * [Getting models](#getting-models)
  * [Updating models](#updating-models)
  * [Deleting models](#deleting-models)
* [Query building](#query-building)
  * [Options](#options)
  * [Expressions](#expressions)
* [Examples](#examples)
  * [Custom model scanning](#custom-model-scanning)
  * [Model relations](#model-relations)

[database/sql]: https://pkg.go.dev/database/sql

## Quickstart

To start using the library just import it alongside your pre-existing code.
Below is an example that defines a simple model, creates it, and retrieves it,

```go
package main

import (
    "context"
    "database/sql"
    "log"
    "time"

    "github.com/andrewpillar/database"

    _ "modernc.org/sqlite"
)

const schema = `CREATE TABLE IF NOT EXISTS posts (
    id         INTEGER NOT NULL,
    title      VARCHAR NOT NULL,
    content    TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (id)
);`

type Post struct {
    ID        int64
    Title     string
    Content   string
    CreatedAt time.Time `db:"created_at"`
}

func (p *Post) Table() string { return "posts" }

func (p *Post) PrimaryKey() *database.PrimaryKey {
    return &database.PrimaryKey{
        Columns: []string{"id"},
        Values:  []any{p.ID},
    }
}

func (p *Post) Params() database.Params {
    return database.Params{
        "id":         database.CreateOnlyParam(p.ID),
        "title":      database.CreateOnlyParam(p.Title),
        "content":    database.MutableParam(p.Content),
        "created_at": database.CreateOnlyParam(p.CreatedAt),
    }
}

func main() {
    db, err := sql.Open("sqlite", "db.sqlite")

    if err != nil {
        log.Fatalln(err)
    }

    defer db.Close()

    if _, err := db.Exec(schema); err != nil {
		log.Fatalln(err)
	}

    p := &Post{
        ID:        10,
        Title:     "My first post",
        Content:   "This is a demonstration.",
        CreatedAt: time.Now().UTC(),
    }

    store := database.NewStore(db, func() *Post {
        return &Post{}
    })

    ctx := context.Background()

    if err := store.Create(ctx, p); err != nil {
        log.Fatalln(err)
    }

    p, ok, err := store.Get(ctx)

    if err != nil {
        log.Fatalln(err)
    }

    if !ok {
        log.Fatalln("could not find post", p.ID)
    }
    log.Println(p)
}
```

## Conventions

This library aims to impose minimal conventions upon the user and tries to make
as few assumptions as possible about the data being worked with. It seeks to
actively eschew anything that resembles an ORM, and instead opting for a query
builder to allow the user to define their own queries. Because of this, it is
entirely possible to only use a subset of the library that is necessary. For
example, if you require only query building, then use the query builder. If you
only need to use [Models](#models) but no [Stores](#stores), then use only
models.

This library strives to stay out of you way as much as possible, whilst still
enabling you the ability to seamlessly work with your data.

## Models

Models allow for the mapping of Go structs to database tables. This is done by
implementing the [database.Model][] interface. This interface wraps three
methods,

* `Table` - The table that contains the Model's data.
* `PrimaryKey` - The primary key for the Model.
* `Params` - The parameters of the Model.

[database.Model]: https://pkg.go.dev/github.com/andrewpillar/database#Model

A model for a blogging application might look like this,

```go
type Post struct {
    ID        int64
    Title     string
    Content   string
    CreatedAt time.Time `db:"created_at"`
}

func (p *Post) Table() string { return "posts" }

func (p *Post) PrimaryKey() *database.PrimaryKey {
    return &database.PrimaryKey{
        Columns: []string{"id"},
        Values:  []any{p.ID},
    }
}

func (p *Post) Params() database.Params {
    return database.Params{
        "id":         database.CreateOnlyParam(p.ID),
        "title":      database.CreateOnlyParam(p.Title),
        "content":    database.MutableParam(p.Content),
        "created_at": database.CreateOnlyParam(p.CreatedAt),
    }
}
```

With the above implementation, the `Post` model defines its table as being
`posts`, its primary key as being the `id` column with a value of `p.ID`, and
its parameters.

### Parameters

Model parameters define which fields on a model can be created, updated, or
mutated. This is done via the `Params` method which returns a set of
[database.Params][].

[database.Params]: https://pkg.go.dev/github.com/andrewpillar/database#Params

Each parameter is defined by one of three functions,

* [database.MutableParam][]
* [database.CreateOnlyParam][]
* [database.UpdateOnlyParam][]

[database.MutableParam]: https://pkg.go.dev/github.com/andrewpillar/database#MutableParam
[database.CreateOnlyParam]: https://pkg.go.dev/github.com/andrewpillar/database#CreateOnlyParam
[database.UpdateOnlyParam]: https://pkg.go.dev/github.com/andrewpillar/database#UpdateOnlyParam

Mutable parameters can be set during creation, and modified during updates.
Whereas a create only param can only be set during creation, and update only can
only be set during model updates.

The Post model defines the following parameters,

```go
func (p *Post) Params() database.Params {
    return database.Params{
        "id":         database.CreateOnlyParam(p.ID),
        "title":      database.CreateOnlyParam(p.Title),
        "content":    database.MutableParam(p.Content),
        "created_at": database.CreateOnlyParam(p.CreatedAt),
    }
}
```

`p.ID`, `p.Title`, and `p.CreatedAt`, are all create only, so these can only be
set during model creation. Whereas `p.Content` is defined as mutable, so this
can be set during creation, and modified afterwards.

### Field aliases

By default, the columns being scanned from a table will be compared against the
struct field. If the two match, then the column value will be scanned into it.
For example, the column `id` would map to the field `ID`, and the column
`fullname` would map to the field `FullName`.

Field aliases can be defined via the `db` struct tag. For example, to map a
snake case field to a Pascal Case struct field, then a struct tag should be
defined,

```go
type Post struct {
    CreatedAt time.Time `db:"created_at"`
}
```

The struct tag can also be used to map column names to nested fields within a
model too. Assume there is a Post model that embeds a User model, and you want
to map the `user_id` column to the `User.ID` field, then this can be achieved
like so,

```go
type User struct {
    ID int64
}

type Post struct {
    ID   int64
    User *User `db:"user_id:id"`
}
```

The format of `<column>:<field>` tells the scanner to map the column to the
field on the underlying struct. This will only work if the field is a struct,
has the necessary exported field, and is a pointer.

This can be taken a step further to scan data into embedded structs, via pattern
matching,

```go
type User struct {
    ID int64
}

type Moderator struct {
    *User `db:"*:*"`
}
```

The syntax of `*:*` tells the underlying scanner to match _all_ columns it has
to all of the fields it can in the underlying struct.

> **Note:** The pattern matching only supports the `*` wildcard, this was added
> to make working with embedded structs in models easier. There is no support
> for finegrained pattern matching of columns and their mapping.

Finally, multiple columns can be mapped to a single struct field. This is useful
when working with queries that can return different column names depending on
the query being run. Consider the following,

```go
type User struct {
    ID int64
}

type Post struct {
    ID   int64
    User *User `db:"user_id:id,users.*"`
}
```

in the above example a comma separated list of alias values has been configured
for the `Post.User` field. This tells the scanner to map the `user_id` column to
the `id` field of the `User` model. But, it also tells the scanner to map any
columns with the `users.*` prefix, to the entire `User` model, should the query
that is performed haved any columns with said prefix. Configuring such aliases
comes in handy when working with joins and you want to load in related model
data. In this case, this would allow for the loading in of the User who made a
Post.

## Stores

Stores are the mechanism that operate on models. They handle creating, updating,
mutating, and querying of models. Each store makes use of [generic][] type
parameters to determine which model is being worked with.

To create a store invoke the [database.NewStore][] function passing the database
connection and a callback for model instantiation,

[generic]: https://go.dev/doc/tutorial/generics
[database.NewStore]: https://pkg.go.dev/github.com/andrewpillar/database#NewStore

```go
posts := database.NewStore(db, func() *Post {
    return &Post{}
})
```

> **Note:** The type parameter is optional when creating a new store. They can
> be given to provide more explicitness in code, such as `NewStore[*Post]`.

In the above exmaple a new [database.Store][] is created for working with Post
models. With this in place, Post models can now be created, retrieved, updated,
and deleted.

[database.Store]: https://pkg.go.dev/github.com/andrewpillar/database#Store

### Creating models

Models can be created via the `Create` and `CreateTx` methods.

```go
p := &Post{
    ID:        10,
    Title:     "Example post",
    Content:   "This is an example post",
    CreatedAt: time.Now().UTC(),
}

if err := posts.Create(ctx, p); err != nil {
    // Handle error.
}
```

This will populate the table's columns with the model parameters that have been
defined as being create only or mutable.

The `CreateTx` method operates the same, the only difference being that it
operates on a transaction. This means the transaction needs committing in order
for the data to persist in the database.

```go
tx, err := db.BeginTx(ctx, nil)

if err != nil {
    // Handle error.
}

defer tx.Rollback()

p := &Post{
    ID:        10,
    Title:     "Example post",
    Content:   "This is an example post",
    CreatedAt: time.Now().UTC(),
}

if err := posts.CreateTx(ctx, tx, p); err != nil {
    // Handle error.
}

if err := tx.Commit(); err != nil {
    // Handle error.
}
```

### Getting models

Models can be retrieved via either the `Get` or `Select` methods.

The `Get` method returns the first model that matches the given
[query.Options][], along with whether or not any model was found.

[query.Options]: https://pkg.go.dev/github.com/andrewpillar/database/query#Option

```go
p, ok, err := posts.Get(ctx, query.WhereEq("id", query.Arg(10)))

if err != nil {
    // Handle error.
}

if !ok {
    // Handle not found.
}
```

The `Select` method returns multiple models that match the given query options.
This takes a [query.Expr][] that defines the columns to get for the model,

[query.Expr]: https://pkg.go.dev/github.com/andrewpillar/database/query#Expr

```go
pp, err := posts.Select(ctx, query.Columns("*"), query.OrderDesc("created_at"))

if err != nil {
    // Handle error.
}
```

### Updating models

Models can be updated via the `Update`, `UpdateTx`, `UpdateMany`, and
`UpdateManyTx` methods.

```go
p, ok, err := posts.Get(ctx, query.WhereEq("id", query.Arg(10)))

if err != nil {
    // Handle error.
}

if !ok {
    // Handle not found.
}

p.Content = "New post content"

if _, err := posts.Update(ctx, p); err != nil {
    // Handle error.
}
```

The `UpdateTx` method operates the same, the only difference being that it
operates on a transaction.

The `UpdateMany` method takes a map for the fields of the model that should be
updated and a list of query options that is used to restrict which models are
updated.

```go
fields := map[string]any{
    "id":           10,
    "content":      "New post content",
    "non_existent": "value",
}

if _, err := posts.UpdateMany(ctx, fields, query.WhereGt(1), query.WhereLt(10)); err != nil {
    // Handle error.
}
```

The above code example will only update the `content` column in the table. This
is because the `content` column on the Post model is defined as mutable, whereas
the `id` column is defined as create only, therefore, it will not be updated.
The `non_existent` field will be ignored as it does not exist on the Post model.

The `UpdateManyTx` method operates the same, the only difference being that it
operates on a transaction.

### Deleting models

Models can be deleted via the `Delete` and `DeleteTx` methods. These take the
lsit of models to delete. If an empty list is given then the methods do nothing,
and no data is deleted.

```go
pp, err := posts.Select(ctx, query.Columns("*"))

if err != nil {
    // Handle error.
}

if err := posts.Delete(ctx, pp...); err != nil {
    // Handle error.
}
```

The `DeleteTx`method operates the same, the only difference being that it
operates on a transaction.

## Query building

Queries can be built via the `github.com/andrewpillar/database/query` package.
This makes use of first class functions for queires to be built up. This aims to
support the most common features of SQL that would be needed for CRUD
operations, but, the package can be extended upon via the implementation of
custom query expressions.

There are 6 main functions that are used for defining a query,

* [query.Select][]
* [query.SelectDistinct][]
* [query.SelectDistinctOn][]
* [query.Insert][]
* [query.Update][]
* [query.Delete][]

[query.Select]: https://pkg.go.dev/github.com/andrewpillar/database/query#Select
[query.SelectDistinct]: https://pkg.go.dev/github.com/andrewpillar/database/query#SelectDistinct
[query.SelectDistinctOn]: https://pkg.go.dev/github.com/andrewpillar/database/query#SelectDistinctOn
[query.Insert]: https://pkg.go.dev/github.com/andrewpillar/database/query#Insert
[query.Update]: https://pkg.go.dev/github.com/andrewpillar/database/query#Update
[query.Delete]: https://pkg.go.dev/github.com/andrewpillar/database/query#Delete

Each of these functions operate in a similar way, in that they each take a
variadic list of [query.Options][] to build the query, and each of them return a
[query.Query][] that can be built and passed off to the database connection to
be run.

[query.Query]: https://pkg.go.dev/github.com/andrewpillar/database/query#Query

### Options

Options are the primary building blocks of the query builder. These are a first
class function which take a query, modify it, and return it,

```go
type Option func(*Query) *Query
```

these are passed to the query functions to define how the query ought be built.
Custom options can be defined by implementing a function that matches the Option
definition. For example, let's consider a blogging application where you might
want to implement a search functionality on posts by a tag. A custom option for
this could be written like so,

```go
func Search(tag name) query.Option {
    return func(q *query.Query) *query.Query {
        return query.WhereIn("id", query.Select(
            query.Columns("post_id"),
            query.From("post_tags"),
            query.WhereLike("name", query.Arg("%" + tag "%")),
        ))(q)
    }
}
```

this custom option could then be used like so,

```go
pp, err := posts.Select(ctx, Search("programming"))
```

### Expressions

SQL expressions are represented via the [query.Expr][] interface that wraps the
`Args` and `Build` methods.

The `Args` method returns the list of arguments for the given expression, if
any, and the `Build` method returns the SQL code for the expression.

For example, [query.Arg][] returns an argument expression. This would be used
for passing arguments through to the underlying query being built. Calling
`Build` on this expression directly would result in the `?` placeholder value
being generated, what with the `Args` method return the actual argument that is
given. For example,

[query.Arg]: https://pkg.go.dev/github.com/andrewpillar/database/query#Arg

```go
q := query.Select(
    query.Columns("*"),
    query.From("users"),
    query.WhereEq("email", query.Arg("user@example.com")),
)
```

the `"user@example.com"` string is passed to the query being built as an
argument, via the [query.WhereEq][] function.

Queries returned from the query functions can also be used as expressions, since
these also implement the `Args` and `Build` methods. This allows for powerful
queries to be built,

```go
q := query.Select(
    query.Columns("*"),
    query.From("posts"),
    query.WhereEq("user_id", query.Arg(1)),
    query.WhereIn("id", query.Select(
        query.Columns("id"),
        query.From("post_tags"),
        query.WhereLike("name", Arg("%programming%")),
    )),
)
```

the above example would result in the following query being built,

```sql
SELECT *
FROM posts
WHERE (
    user_id = $1
    AND id IN (
        SELECT post_id
        FROM post_tags
        WHERE (name LIKE $2)
    )
)
```

## Examples

Below are some examples which will demonstrate how this library can be used in
various scenarios. These exist to show that different parts of the library can
be used independent of one another, and can be used alongside the standard
library itself.

Whilst this library does offer some nice abstractions of the scanning of data
from the database into Go structs, it does not tell you how your data should be
structured. For example, with primary keys, it does not say that your primary
key should be a single field, or that it should be auto-incrementing.

In essence, this library was designed with rows of arbitrary data in mind, since
that is what data from the database is returned as. Either a row, or rows, that
have some column names, and respective values. This library just provides some
simple helpers to aid in the scanning of said values into the data you may have
in your Go code.

### Custom model scanning

By default, the library makes use of [reflect][] to attempt to deduce how the
columns should be mapped to the struct it is scanning data into. However, custom
scanning can be implemented on a per-model basis via the [database.RowScanner][]
interface.

[reflect]: https://pkg.go.dev/reflect
[database.RowScanner]: https://pkg.go.dev/github.com/andrewpillar/database#RowScanner

For example,

```go
type Notification struct {
    ID   int64
    Data map[string]any
}

func (n *Notification) Scan(r *database.Row) error {
    var data string

    dest := map[string]any{
        "id":   &n.ID,
        "data": &data,
    }

    if err := r.Scan(dest); err != nil {
        return err
    }

    if err := json.Unmarshal([]byte(data), &n.Data); err != nil {
        return err
    }
    return nil
}
```

with the above implementation, the user defines exactly how the row is scanned
into the model. This is achieved by passing a map of pointer values to the
[Row.Scan][] method. This will scan in only the columns that exist in the row
and are defined in the given map.

[Row.Scan]: https://pkg.go.dev/github.com/andrewpillar/database#Row.Scan

Under the hood, a new [Scanner][] is created which is given the database rows
that have been selected. This means that it is entirely possible to not used
[Stores](#stores) when working with models. For example, the following code
could be written to retrieve a model,

[Scanner]: https://pkg.go.dev/github.com/andrewpillar/database#Scanner

```go
rows, err := db.Query("SELECT * FROM notifications")

if err != nil {
    // Handle error.
}

defer rows.Close()

sc, err := database.NewScanner(rows)

if err != nil {
    // Handle error.
}

nn := make([]*Notification, 0)

for rows.Next() {
    n := &Notification{}

    if err := sc.Scan(n); err != nil {
        // Handle error.
    }
}
```

Of course, even without the custom `Scan` method, and just through reflection,
the same above code would still work.

### Model relations

Unlike in an ORM, there is no way of formally defining relations between models
with this library. Instead, it is recommended that the necessary queries are
built that are used to query the related data, and return them in rows.

For example, assume a blogging application is being built that has User and Post
models as defined below,

```go
type User struct {
    ID        int64
    Email     string
    Username  string
    CreatedAt time.Time `db:"created_at"`
}

type Post struct {
    ID        int64
    User      *User `db:"user_id:id,users.*"`
    Title     string
    Content   string
    CreatedAt time.Time `db:"created_at"`
}
```

If you wanted to load in all of the posts with their respective user then you
would write the following,

```go
posts := database.NewStore(db, func() *Post {
    // Make sure the User model is instantiated for scanning, otherwise the
    // program will panic trying to deference a nil pointer.
    return &Post{
        User: &User{},
    }
})

// Again, make sure this is fully instantiated because the database.Columns
// function calls Table on each model it is given to determine the columns of
// the model.
p := &Post{
    User: &User{},
}

pp, err := posts.Select(ctx, database.Columns(p, p.User), database.Join(p.User, "user_id"))

if err != nil {
    // Handle error.
}

for _, p := range pp {
    fmt.Printf("Post %s by %s\n", p.Title, p.User.Username)
}
```

The above code makes use of the `database.Columns` and `database.Join` functions
which simply makes building these queries easier. The same code using the query
builder itself would look something like,

```
posts.Select(
    ctx,
    query.Exprs(
        query.Ident("posts.id"),
        query.Ident("posts.user_id"),
        query.Ident("posts.title"),
        query.Ident("posts.content"),
        query.Ident("posts.created_at"),
        query.ColumnAs("users.id", "users.id"),
        query.ColumnAs("users.email", "users.email"),
        query.ColumnAs("users.username", "users.username"),
        query.ColumnAs("users.created_at", "users.created_at"),
    ),
    query.From("posts"),
    query.Join("users", Eq(Ident("posts.user_id"), Ident("users.id"))),
)
```

It is entirely possible to write these queries by hand, and make use of the
[database.Scanner][] to achieve the same result,

```go
q := `
SELECT posts.id,
    posts.user_id,
    posts.title,
    posts.content,
    posts.created_at,
    users.id AS 'users.id',
    users.email AS 'users.email',
    users.username AS 'users.username',
    users.created_at AS 'users.created_at'
FROM posts
JOIN users ON posts.user_id = users.id
`

rows, err := db.Query(q)

if err != nil {
    // Handle error.
}

defer rows.Close()

sc, err := database.NewScanner(rows)

if err != nil {
    // Handle error.
}

pp := make([]*Post, 0)

for rows.Next() {
    p := &Post{
        User: &User{},
    }

    if err := sc.Scan(p); err != nil {
        // Handle error.
    }
}
```
