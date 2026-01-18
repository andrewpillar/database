package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/andrewpillar/database/query"
)

type Null[T any] struct {
	sql.Null[T]
}

// MarshalJSON returns the JSON representation of the null value. If the value
// is null, then "null" is returned, otherwise the marshalled representation
// of the underlying value is returned.
func (n *Null[T]) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.V)
}

// PrimaryKey represents the primary key of a model. This is typically used to
// query individual models by their primary key. This also supports composite
// keys too.
type PrimaryKey struct {
	// List of columns that make up the primary key of the model.
	Columns []string

	// List of values for the respective columns of the primary key.
	Values []any
}

// Where returns a [query.Option] that can be used for adding a WHERE clause to
// a query that will operate on the value of the primary key itself.
func (pk *PrimaryKey) Where() query.Option {
	opts := make([]query.Option, 0)

	for i, col := range pk.Columns {
		opts = append(opts, query.WhereEq(col, query.Arg(pk.Values[i])))
	}
	return query.Options(opts...)
}

type paramMode uint8

const (
	paramCreate paramMode = iota + 1
	paramUpdate
)

func (m paramMode) has(mask paramMode) bool {
	return (m & mask) == mask
}

// Param is the paramter of a model. This is used to determine what parameters
// in a model can be created, or updated during model operations.
type Param struct {
	mode  paramMode
	value any
}

// MutableParam returns a [Param] that can be both created and updated on a
// model.
func MutableParam(v any) Param {
	return Param{
		mode:  paramCreate | paramUpdate,
		value: v,
	}
}

// CreateOnlyParam returns a [Param] that can only be created on a model.
func CreateOnlyParam(v any) Param {
	return Param{
		mode:  paramCreate,
		value: v,
	}
}

// UpdateOnlyParam returns a [Param] that can only be updated on a model.
func UpdateOnlyParam(v any) Param {
	return Param{
		mode:  paramUpdate,
		value: v,
	}
}

// Params is a map of model parameters where the key is the respective column
// name for that model's parameter in the database table.
type Params map[string]Param

// Model is the interface that represents data in a database table.  It wraps
// three methods.
//
// Table returns the name of the database table where the Model data is stored.
//
// PrimaryKey returns the [PrimaryKey] of the Model itself. It is valid for this
// to return nil, depending on whether or not the Model has a primary key.
//
// Params returns the Model parameters, and whether they can be created or
// updated.
type Model interface {
	Table() string

	PrimaryKey() *PrimaryKey

	Params() Params
}

// List returns a list [query.Expr] of the given column from the given models.
func List[M Model](col string, mm ...M) query.Expr {
	vals := make([]any, 0, len(mm))

	for _, m := range mm {
		params := m.Params()

		if p, ok := params[col]; ok {
			vals = append(vals, p.value)
		}
	}
	return query.List(vals...)
}

// Columns returns the column [query.Expr] for the columns in the given primary
// Model. If any joins are given, then these are included in the expression
// too and aliased. The column names will be prefixed with the model's table
// name, for example,
//
//	database.Columns(&Post{}, &User{})
//
// would result in the following SQL code,
//
//	posts.id, posts.user_id, posts.title, users.id AS users.id, users.email AS users.email
//
// Assuming that both the Post and User model have the above columns names.
func Columns(primary Model, joins ...Model) query.Expr {
	params := primary.Params()
	table := primary.Table()

	cols := make([]string, 0, len(params))

	for fld := range params {
		cols = append(cols, fmt.Sprintf("%s.%s", table, fld))
	}

	if len(joins) == 0 {
		return query.Columns(cols...)
	}

	exprs := []query.Expr{
		query.Columns(cols...),
	}

	for _, m := range joins {
		params := m.Params()
		table := m.Table()

		for fld := range params {
			fullname := fmt.Sprintf("%s.%s", table, fld)

			exprs = append(exprs, query.ColumnAs(fullname, fullname))
		}
	}
	return query.Exprs(exprs...)
}

// Join returns a JOIN clause on the given [Model], using the given fields. The
// given fields are expected to be foreign keys in the table onto which the data
// is being joined. For example, assume a query is being written that wants to
// join the user of a post to each post that is queried, then the following code
// would be written,
//
//	q := query.Select(
//	    database.Columns(&Post{}, &User{}),
//	    database.Join(&User{}, "user_id"),
//	)
//
// This would then result in the following SQL code when built,
//
//	SELECT posts.id,
//	    posts.user_id,
//	    posts.title,
//	    users.id AS users.id,
//	    users.email AS users.email
//	 JOIN users ON posts.user_id = users.id;
//
// For joining on composite keys, the fields must line up with the composite
// keys in the [PrimaryKey] of the [Model]. That is, if a model has a primary
// key of,
//
//	&database.PrimaryKey{
//	    Columns: []string{"field_1", "field_2"},
//	    Values:  []any{m.Field1, m.Field2},
//	}
//
// then the fields must be passed like so,
//
//	database.Join(&Table2{}, "t2_field_1", "t2_field_2")
func Join(m Model, fields ...string) query.Option {
	pk := m.PrimaryKey()
	table := m.Table()

	exprs := make([]query.Expr, 0, len(pk.Columns))

	for i, col := range pk.Columns {
		foreign := fields[i]
		primary := fmt.Sprintf("%s.%s", table, col)

		exprs = append(exprs, query.Eq(query.Ident(foreign), query.Ident(primary)))
	}
	return query.Join(table, query.And(exprs...))
}

// Store handles the create, read, update, and delete operations of the [Model].
type Store[M Model] struct {
	*sql.DB

	table string
	new   func() M
}

// NewStore returns a new store for the given [Model]. This takes a database
// connection and a callback function. The callback function is used for
// instantiating new models whenever a model is queried from the database.
func NewStore[M Model](db *sql.DB, new func() M) *Store[M] {
	m := new()

	return &Store[M]{
		DB:    db,
		table: m.Table(),
		new:   new,
	}
}

type execFunc func(context.Context, string, ...any) (sql.Result, error)

func (s *Store[M]) doCreate(ctx context.Context, execFn execFunc, mm ...M) error {
	if len(mm) == 0 {
		return nil
	}

	m := mm[0]

	params := m.Params()
	cols := make([]string, 0, len(params))

	for name, param := range params {
		if param.mode.has(paramCreate) {
			cols = append(cols, name)
		}
	}

	opts := make([]query.Option, 0, len(mm))
	vals := make([]any, 0)

	for _, m := range mm {
		params := m.Params()

		for _, col := range cols {
			vals = append(vals, params[col].value)
		}

		opts = append(opts, query.Values(vals...))
		vals = vals[0:0]
	}

	q := query.Insert(s.table, query.Columns(cols...), opts...)

	_, err := execFn(ctx, q.Build(), q.Args()...)

	return err
}

// Create the given models.
func (s *Store[M]) Create(ctx context.Context, mm ...M) error {
	return s.doCreate(ctx, s.ExecContext, mm...)
}

// CreateTx creates the given models using the given transaction.
func (s *Store[M]) CreateTx(ctx context.Context, tx *sql.Tx, mm ...M) error {
	return s.doCreate(ctx, tx.ExecContext, mm...)
}

type queryFunc func(context.Context, string, ...any) (*sql.Rows, error)

func (s *Store[M]) doSelect(ctx context.Context, queryFn queryFunc, expr query.Expr, opts ...query.Option) ([]M, error) {
	opts = append([]query.Option{
		query.From(s.table),
	}, opts...)

	q := query.Select(expr, opts...)

	rows, err := queryFn(ctx, q.Build(), q.Args()...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	sc, err := NewScanner(rows)

	if err != nil {
		return nil, err
	}

	mm := make([]M, 0)

	for rows.Next() {
		m := s.new()

		if err := sc.Scan(m); err != nil {
			return nil, err
		}
		mm = append(mm, m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mm, nil
}

// Select returns the models that match the given query options. The given
// [query.Expr] should be the columns to select for the models.
func (s *Store[M]) Select(ctx context.Context, expr query.Expr, opts ...query.Option) ([]M, error) {
	return s.doSelect(ctx, s.QueryContext, expr, opts...)
}

func (s *Store[M]) doGet(ctx context.Context, queryFn queryFunc, opts ...query.Option) (M, bool, error) {
	var zero M

	opts = append(opts, query.Limit(1))

	mm, err := s.doSelect(ctx, queryFn, query.Columns("*"), opts...)

	if err != nil {
		return zero, false, err
	}

	if len(mm) == 0 {
		return zero, false, nil
	}
	return mm[0], true, nil
}

// Get returns the first model that can be found that matches the given query
// options, and whether or not it was found via the bool return value.
func (s *Store[M]) Get(ctx context.Context, opts ...query.Option) (M, bool, error) {
	return s.doGet(ctx, s.QueryContext, opts...)
}

func (s *Store[M]) doUpdate(ctx context.Context, execFn execFunc, m M) (sql.Result, error) {
	opts := make([]query.Option, 0)

	params := m.Params()

	for name, param := range params {
		if param.mode.has(paramUpdate) {
			opts = append(opts, query.Set(name, query.Arg(param.value)))
		}
	}

	opts = append(opts, m.PrimaryKey().Where())

	q := query.Update(s.table, opts...)

	return execFn(ctx, q.Build(), q.Args()...)
}

// Update the given model on the model's [PrimaryKey] to determine which one
// should be updated.
func (s *Store[M]) Update(ctx context.Context, m M) (sql.Result, error) {
	return s.doUpdate(ctx, s.ExecContext, m)
}

// UpdateTx updates the given model using the given transation, on the model's
// [PrimaryKey] to determine which one should be updated.
func (s *Store[M]) UpdateTx(ctx context.Context, tx *sql.Tx, m M) (sql.Result, error) {
	return s.doUpdate(ctx, tx.ExecContext, m)
}

func (s *Store[M]) doUpdateMany(ctx context.Context, execFn execFunc, fields map[string]any, opts ...query.Option) (sql.Result, error) {
	setopts := make([]query.Option, 0)

	m := s.new()
	params := m.Params()

	for fld, val := range fields {
		if param, ok := params[fld]; ok {
			if param.mode.has(paramUpdate) {
				setopts = append(setopts, query.Set(fld, query.Arg(val)))
			}
		}
	}

	q := query.Update(s.table, append(setopts, opts...)...)

	return execFn(ctx, q.Build(), q.Args()...)
}

// UpdateMany updates all models in the database that match the given query
// options using the given map of fields. Only the fields that exist in the
// model and can be updated will be changed.
func (s *Store[M]) UpdateMany(ctx context.Context, fields map[string]any, opts ...query.Option) (sql.Result, error) {
	return s.doUpdateMany(ctx, s.ExecContext, fields, opts...)
}

// UpdateManyTx updates all models in the database that match the given query
// options using the given map of fields using the given transaction. Only the
// fields that exist in the model and can be updated will be changed.
func (s *Store[M]) UpdateManyTx(ctx context.Context, tx *sql.Tx, fields map[string]any, opts ...query.Option) (sql.Result, error) {
	return s.doUpdateMany(ctx, tx.ExecContext, fields, opts...)
}

type noResult struct{}

func (r noResult) LastInsertId() (int64, error) { return 0, nil }
func (r noResult) RowsAffected() (int64, error) { return 0, nil }

func (s *Store[M]) doDelete(ctx context.Context, execFn execFunc, mm ...M) (sql.Result, error) {
	if len(mm) == 0 {
		return noResult{}, nil
	}

	m := mm[0]
	pk := m.PrimaryKey()

	col := "(" + strings.Join(pk.Columns, ", ") + ")"

	vals := make([]any, 0)

	for _, m := range mm {
		var val any

		pk := m.PrimaryKey()
		val = pk.Values[0]

		if len(pk.Values) > 1 {
			val = query.List(pk.Values...)
		}
		vals = append(vals, val)
	}

	q := query.Delete(s.table, query.WhereIn(col, query.List(vals...)))

	return execFn(ctx, q.Build(), q.Args()...)
}

// Delete the given models. If no models are given, this is a no-op.
func (s *Store[M]) Delete(ctx context.Context, mm ...M) (sql.Result, error) {
	return s.doDelete(ctx, s.ExecContext, mm...)
}

// DeleteTx deletes the given models using the given transaction. If no models
// are given, then this is a no-op.
func (s *Store[M]) DeleteTx(ctx context.Context, tx *sql.Tx, mm ...M) (sql.Result, error) {
	return s.doDelete(ctx, tx.ExecContext, mm...)
}
