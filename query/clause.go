package query

import (
	"fmt"
	"strconv"
	"strings"
)

type clauseKind uint

//go:generate stringer -type clauseKind -linecomment
const (
	_fromClause      clauseKind = iota + 1 // FROM
	_limitClause                           // LIMIT
	_offsetClause                          // OFFSET
	_orderClause                           // ORDER BY
	_unionClause                           // UNION
	_valuesClause                          // VALUES
	_whereClause                           // WHERE
	_returningClause                       // RETURNING
	_setClause                             // SET
	_joinClause                            // JOIN
)

type clause interface {
	Expr

	kind() clauseKind
}

type whereClause struct {
	conj string
	expr Expr
}

func where(conj string, expr Expr) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, &whereClause{
			conj: conj,
			expr: expr,
		})
		q.args = append(q.args, expr.Args()...)

		return q
	}
}

func Where(expr Expr) Option {
	return where("AND", expr)
}

func WhereEq(col string, expr Expr) Option {
	return where("AND", Eq(Ident(col), expr))
}

func WhereNotEq(col string, expr Expr) Option {
	return where("AND", Neq(Ident(col), expr))
}

func WhereGt(col string, expr Expr) Option {
	return where("AND", Gt(Ident(col), expr))
}

func WhereGeq(col string, expr Expr) Option {
	return where("AND", Geq(Ident(col), expr))
}

func WhereLt(col string, expr Expr) Option {
	return where("AND", Lt(Ident(col), expr))
}

func WhereLeq(col string, expr Expr) Option {
	return where("AND", Leq(Ident(col), expr))
}

func WhereIs(col string, expr Expr) Option {
	return where("AND", Is(Ident(col), expr))
}

func WhereLike(col string, expr Expr) Option {
	return where("AND", Like(Ident(col), expr))
}

func WhereIsNot(col string, expr Expr) Option {
	return where("AND", IsNot(Ident(col), expr))
}

func WhereIsNil(col string) Option {
	return WhereIs(col, Lit("NULL"))
}

func WhereIsNotNil(col string) Option {
	return WhereIsNot(col, Lit("NULL"))
}

func WhereIn(col string, expr Expr) Option {
	return where("AND", In(Ident(col), expr))
}

func WhereNotIn(col string, expr Expr) Option {
	return where("AND", NotIn(Ident(col), expr))
}

func OrWhere(expr Expr) Option {
	return where("OR", expr)
}

func OrWhereEq(col string, expr Expr) Option {
	return OrWhere(Eq(Ident(col), expr))
}

func OrWhereNotEq(col string, expr Expr) Option {
	return OrWhere(Neq(Ident(col), expr))
}

func OrWhereGt(col string, expr Expr) Option {
	return OrWhere(Gt(Ident(col), expr))
}

func OrWhereGeq(col string, expr Expr) Option {
	return OrWhere(Geq(Ident(col), expr))
}

func OrWhereLt(col string, expr Expr) Option {
	return OrWhere(Lt(Ident(col), expr))
}

func OrWhereLeq(col string, expr Expr) Option {
	return OrWhere(Leq(Ident(col), expr))
}

func OrWhereIs(col string, expr Expr) Option {
	return OrWhere(Is(Lit(col), expr))
}

func OrWhereIsNot(col string, expr Expr) Option {
	return OrWhere(IsNot(Lit(col), expr))
}

func OrWhereIsNil(col string) Option {
	return OrWhereIs(col, Lit("NULL"))
}

func OrWhereIsNotNil(col string) Option {
	return OrWhereIsNot(col, Lit("NULL"))
}

func OrWhereIn(col string, expr Expr) Option {
	return OrWhere(In(Ident(col), expr))
}

func OrWhereNotIn(col string, expr Expr) Option {
	return OrWhere(NotIn(Ident(col), expr))
}

func (c *whereClause) Args() []any      { return nil }
func (c *whereClause) Build() string    { return c.expr.Build() }
func (c *whereClause) kind() clauseKind { return _whereClause }

type fromClause struct {
	table string
	alias string
}

func From(table string) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, &fromClause{
			table: table,
		})
		return q
	}
}

func FromAs(table, alias string) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, &fromClause{
			table: table,
			alias: alias,
		})
		return q
	}
}

func (c *fromClause) Args() []any { return nil }

func (c *fromClause) Build() string {
	if c.alias == "" {
		return c.table
	}
	return c.table + " AS " + c.alias
}

func (c *fromClause) kind() clauseKind { return _fromClause }

type limitClause struct {
	n int64
}

func Limit(n int64) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, limitClause{
			n: n,
		})
		return q
	}
}

func (c limitClause) Args() []any      { return nil }
func (c limitClause) Build() string    { return strconv.FormatInt(c.n, 10) }
func (c limitClause) kind() clauseKind { return _limitClause }

type offsetClause struct {
	n int64
}

func Offset(n int64) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, offsetClause{
			n: n,
		})
		return q
	}
}

func (c offsetClause) Args() []any      { return nil }
func (c offsetClause) Build() string    { return strconv.FormatInt(c.n, 10) }
func (c offsetClause) kind() clauseKind { return _offsetClause }

type orderClause struct {
	cols []string
	dir  string
}

func OrderAsc(cols ...string) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, &orderClause{
			cols: cols,
			dir:  "ASC",
		})
		return q
	}
}

func OrderDesc(cols ...string) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, &orderClause{
			cols: cols,
			dir:  "DESC",
		})
		return q
	}
}

func (c *orderClause) Args() []any      { return nil }
func (c *orderClause) Build() string    { return strings.Join(c.cols, ", ") + " " + c.dir }
func (c *orderClause) kind() clauseKind { return _orderClause }

type unionClause struct {
	q *Query
}

func (c *unionClause) Args() []any      { return nil }
func (c *unionClause) Build() string    { return c.q.buildInitial() }
func (c *unionClause) kind() clauseKind { return _unionClause }

type returningClause struct {
	cols []string
}

func Returning(cols ...string) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, &returningClause{
			cols: cols,
		})
		return q
	}
}

func (c *returningClause) Args() []any      { return nil }
func (c *returningClause) Build() string    { return strings.Join(c.cols, ", ") }
func (c *returningClause) kind() clauseKind { return _returningClause }

type setClause struct {
	col  string
	expr Expr
}

func Set(col string, expr Expr) Option {
	return func(q *Query) *Query {
		if q.stmt == updateStmt {
			q.clauses = append(q.clauses, &setClause{
				col:  col,
				expr: Lit(expr.Build()),
			})
			q.args = append(q.args, expr.Args()...)
		}
		return q
	}
}

func (c *setClause) Args() []any      { return nil }
func (c *setClause) Build() string    { return c.col + " = " + c.expr.Build() }
func (c *setClause) kind() clauseKind { return _setClause }

type valuesClause struct {
	items []string
	args  []any
}

func Values(vals ...any) Option {
	tmp := make([]any, 0, len(vals))

	for _, val := range vals {
		tmp = append(tmp, val)
	}

	return func(q *Query) *Query {
		items := make([]string, 0, len(tmp))

		for range tmp {
			items = append(items, "?")
		}

		q.clauses = append(q.clauses, &valuesClause{
			items: items,
			args:  tmp,
		})
		q.args = append(q.args, tmp...)
		return q
	}
}

func (c *valuesClause) Args() []any      { return c.args }
func (c *valuesClause) Build() string    { return "(" + strings.Join(c.items, ", ") + ")" }
func (c *valuesClause) kind() clauseKind { return _valuesClause }

type joinClause struct {
	table string
	expr  Expr
}

func Join(table string, expr Expr) Option {
	return func(q *Query) *Query {
		q.clauses = append(q.clauses, &joinClause{
			table: table,
			expr:  expr,
		})
		return q
	}
}

func (c *joinClause) Args() []any      { return nil }
func (c *joinClause) Build() string    { return fmt.Sprintf("%s ON %s", c.table, c.expr.Build()) }
func (c *joinClause) kind() clauseKind { return _joinClause }
