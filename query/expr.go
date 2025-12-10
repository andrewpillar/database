package query

import (
	"fmt"
	"strings"
)

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

func Exprs(e ...Expr) Expr {
	return exprs(e)
}

type listExpr struct {
	items []string
	wrap  bool
	args  []any
}

func Columns(cols ...string) Expr {
	return &listExpr{
		items: cols,
	}
}

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

func Ident(s string) Expr {
	return identExpr(s)
}

func (e identExpr) Args() []any   { return nil }
func (e identExpr) Build() string { return string(e) }

type argExpr struct {
	val any
}

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

func Sum(col string) Expr {
	return &callExpr{
		name: "SUM",
		args: []Expr{
			Lit(col),
		},
	}
}

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

func (e *callExpr) Args() []any { return nil }

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

func And(conds ...Expr) Expr {
	return &andOrExpr{
		conj:  " AND ",
		conds: conds,
	}
}

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

func Eq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "=",
		right: b,
	}
}

func Neq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "!=",
		right: b,
	}
}

func Gt(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    ">",
		right: b,
	}
}

func Geq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    ">=",
		right: b,
	}
}

func Lt(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "<",
		right: b,
	}
}

func Leq(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "<=",
		right: b,
	}
}

func Like(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "LIKE",
		right: b,
	}
}

func Is(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "IS",
		right: b,
	}
}

func IsNot(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "IS NOT",
		right: b,
	}
}

func In(a, b Expr) Expr {
	return &opExpr{
		left:  a,
		op:    "IN",
		right: b,
	}
}

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

func As(in Expr, out string) Expr {
	return &asClause{
		in:  in,
		out: out,
	}
}

func ColumnAs(in, out string) Expr {
	return As(Ident(in), out)
}

func (c *asClause) Args() []any   { return nil }
func (c *asClause) Build() string { return fmt.Sprintf("%s AS '%s'", c.in.Build(), c.out) }
