package query

import (
	"fmt"
	"strings"
)

// Expr represents an SQL expression.
//
// Args returns the list of arguments the SQL expression has. This is used to
// correctly handle the parameter binding of dynamic arguments into a query.
// It is valid for this to return nil.
//
// Build returns the SQL code that represents the expression.
type Expr interface {
	Args() []any

	Build() string
}

type exprs []Expr

func (e exprs) Args() []any {
	args := make([]any, 0, len(e))

	for _, expr := range e {
		args = append(args, expr.Args()...)
	}
	return args
}

func (e exprs) Build() string {
	var buf strings.Builder

	for i, expr := range e {
		buf.WriteString(expr.Build())

		if i != len(e)-1 {
			buf.WriteString(", ")
		}
	}
	return buf.String()
}

// Exprs turns the list of expressions into a single expression. When built the
// expressions will be separated by the string ", ".
func Exprs(e ...Expr) Expr {
	return exprs(e)
}

type listExpr struct {
	items []string
	wrap  bool
	args  []any
}

// Columns turns the given strings into a list expression of column names. This
// would be used to control which columns should be retrieved as part of a
// SELECT query.
//
// Calling this function is no different than calling [query.List] and passing
// multiple [query.Ident] expressions, for example,
//
//	query.List(query.Ident("id"), query.Ident("email"))
//
// The main difference is that the above will return an expression wrapped in a
// pair of parentheses. This is still valid SQL code, this function simply exists
// for ease of use.
func Columns(cols ...string) Expr {
	return &listExpr{
		items: cols,
	}
}

// List turns the given values into a list expression. If the given values are
// lists then they will be wrapped appropriately. If the given values are
// literal expressions, then they will end up in the built SQL code verbatim and
// not as placeholders.
func List(vals ...any) Expr {
	items := make([]string, 0, len(vals))
	args := make([]any, 0, len(vals))

	for _, val := range vals {
		switch expr := val.(type) {
		case *listExpr:
			items = append(items, expr.Build())
			args = append(args, val)
		case litExpr:
			items = append(items, expr.Build())
		case identExpr:
			items = append(items, expr.Build())
		default:
			items = append(items, "?")
			args = append(args, val)
		}
	}

	return &listExpr{
		items: items,
		wrap:  true,
		args:  args,
	}
}

func (e *listExpr) Args() []any { return e.args }

func (e *listExpr) Build() string {
	items := strings.Join(e.items, ", ")

	if e.wrap {
		return "(" + items + ")"
	}
	return items
}

type identExpr string

// Ident turns the given string into an identifier expression. This would be
// used for referring directly to a column name in SQL.
//
// For example,
//
//	query.Join("users", query.Eq(query.Ident("posts.user_id"), query.Ident("users.id")))
//
// becomes,
//
//	JOIN users ON posts.user_id = users.id
func Ident(s string) Expr {
	return identExpr(s)
}

func (e identExpr) Args() []any   { return nil }
func (e identExpr) Build() string { return string(e) }

type argExpr struct {
	val any
}

// Arg turns the given value into an argument expression. This would be used for
// passing user provided values into a query that should be bound to parameters.
func Arg(val any) Expr {
	return argExpr{
		val: val,
	}
}

func (e argExpr) Args() []any   { return []any{e.val} }
func (e argExpr) Build() string { return "?" }

type litExpr struct {
	val any
}

// Lit turns the given value in a literal expression. This is passed directly
// through into the query, and does not use a parameter binding for it.
func Lit(val any) Expr {
	return litExpr{
		val: val,
	}
}

func (e litExpr) Args() []any   { return nil }
func (e litExpr) Build() string { return fmt.Sprintf("%v", e.val) }

type callExpr struct {
	name string
	args []Expr
}

// Sum returns the SUM aggregate call expression on the given column.
func Sum(expr Expr) Expr {
	return &callExpr{
		name: "SUM",
		args: []Expr{
			expr,
		},
	}
}

func Lower(expr Expr) Expr {
	return &callExpr{
		name: "LOWER",
		args: []Expr{
			expr,
		},
	}
}

// Count returns the COUNT aggregate call expression on the given columns.
func Count(cols ...string) Expr {
	args := make([]Expr, 0, len(cols))

	for _, col := range cols {
		args = append(args, Lit(col))
	}

	return &callExpr{
		name: "COUNT",
		args: args,
	}
}

func (e *callExpr) Args() []any {
	args := make([]any, 0)

	for _, expr := range e.args {
		args = append(args, expr.Args()...)
	}
	return args
}

func (e *callExpr) Build() string {
	args := make([]string, 0, len(e.args))

	for _, arg := range e.args {
		args = append(args, arg.Build())
	}
	return e.name + "(" + strings.Join(args, ", ") + ")"
}

type andOrExpr struct {
	conj  string
	conds []Expr
}

// And takes the given expressions and joins them together using the AND clause
// when built.
func And(conds ...Expr) Expr {
	return &andOrExpr{
		conj:  " AND ",
		conds: conds,
	}
}

// Or takes the given expressions and joins them together using the OR clause
// when built.
func Or(conds ...Expr) Expr {
	return &andOrExpr{
		conj:  " OR ",
		conds: conds,
	}
}

func (e *andOrExpr) Args() []any {
	args := make([]any, 0)

	for _, expr := range e.conds {
		args = append(args, expr.Args()...)
	}
	return args
}

func (e *andOrExpr) Build() string {
	conds := make([]string, 0, len(e.conds))

	for _, expr := range e.conds {
		conds = append(conds, expr.Build())
	}
	return strings.Join(conds, e.conj)
}

type opExpr struct {
	left  Expr
	op    string
	right Expr
}

// Eq a = b
func Eq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "=",
		right: b,
	}
}

// Neq a != b
func Neq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "!=",
		right: b,
	}
}

// Gt a > b
func Gt(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    ">",
		right: b,
	}
}

// Get a >= b
func Geq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    ">=",
		right: b,
	}
}

// Lt a < b
func Lt(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "<",
		right: b,
	}
}

// Leq a <= b
func Leq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "<=",
		right: b,
	}
}

// Like a LIKE b
func Like(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "LIKE",
		right: b,
	}
}

// Is a IS b
func Is(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "IS",
		right: b,
	}
}

// IsNot a IS NOT b
func IsNot(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "IS NOT",
		right: b,
	}
}

// In a IN b
func In(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "IN",
		right: b,
	}
}

// NotIn a NOT IN b
func NotIn(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "NOT IN",
		right: b,
	}
}

func (e *opExpr) Args() []any {
	return append(
		e.left.Args(),
		e.right.Args()...,
	)
}

func (e *opExpr) Build() string {
	var left, right string

	if q, ok := e.left.(*Query); ok {
		left = fmt.Sprintf("(%s)", q.buildInitial())
	} else {
		left = e.left.Build()
	}

	if q, ok := e.right.(*Query); ok {
		right = fmt.Sprintf("(%s)", q.buildInitial())
	} else {
		right = e.right.Build()
	}
	return fmt.Sprintf("%s %s %s", left, e.op, right)
}

type asClause struct {
	in  Expr
	out string
}

// As specifies an AS expression on the given expression. For example,
//
//	query.As(query.Count("id"), "id_count")
func As(in Expr, out string) Expr {
	return &asClause{
		in:  in,
		out: out,
	}
}

// ColumnAs specifies an AS expression on the two columns.
func ColumnAs(in, out string) Expr {
	return As(Ident(in), out)
}

func (c *asClause) Args() []any   { return nil }
func (c *asClause) Build() string { return fmt.Sprintf("%s AS '%s'", c.in.Build(), c.out) }
