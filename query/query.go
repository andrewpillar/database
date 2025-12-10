package query

import (
	"strconv"
	"strings"
)

type statement uint

//go:generate stringer -type statement -linecomment
const (
	deleteStmt           statement = iota + 1 // DELETE
	insertStmt                                // INSERT
	selectStmt                                // SELECT
	updateStmt                                // UPDATE
	selectDistinctStmt                        // SELECT DISTINCT
	selectDistinctOnStmt                      // SELECT DISTINCT ON
)

type Query struct {
	stmt    statement
	table   string
	exprs   []Expr
	clauses []clause
	args    []any
}

type Option func(*Query) *Query

func Delete(table string, opts ...Option) *Query {
	q := &Query{
		stmt:  deleteStmt,
		table: table,
	}

	for _, opt := range opts {
		q = opt(q)
	}
	return q
}

func Insert(table string, expr Expr, opts ...Option) *Query {
	q := &Query{
		stmt:  insertStmt,
		table: table,
		exprs: []Expr{expr},
	}

	for _, opt := range opts {
		q = opt(q)
	}
	return q
}

func Select(expr Expr, opts ...Option) *Query {
	q := &Query{
		stmt:  selectStmt,
		exprs: []Expr{expr},
	}

	for _, opt := range opts {
		q = opt(q)
	}
	return q
}

func SelectDistinct(expr Expr, opts ...Option) *Query {
	q := &Query{
		stmt:  selectDistinctStmt,
		exprs: []Expr{expr},
	}

	for _, opt := range opts {
		q = opt(q)
	}
	return q
}

func SelectDistinctOn(expr1, expr2 Expr, opts ...Option) *Query {
	q := &Query{
		stmt: selectDistinctOnStmt,
		exprs: []Expr{
			expr1,
			expr2,
		},
	}

	for _, opt := range opts {
		q = opt(q)
	}
	return q
}

func Update(table string, opts ...Option) *Query {
	q := &Query{
		stmt:  updateStmt,
		table: table,
	}

	for _, opt := range opts {
		q = opt(q)
	}
	return q
}

func Union(queries ...*Query) *Query {
	var union Query

	for _, q := range queries {
		union.args = append(union.args, q.args...)
		union.clauses = append(union.clauses, &unionClause{
			q: q,
		})
	}
	return &union
}

func Options(opts ...Option) Option {
	return func(q *Query) *Query {
		for _, opt := range opts {
			q = opt(q)
		}
		return q
	}
}

func (q *Query) Args() []any { return q.args }

func (q *Query) conj(cl clause) string {
	if cl == nil {
		return ""
	}

	switch v := cl.(type) {
	case *whereClause:
		return " " + v.conj + " "
	case *unionClause:
		return " " + cl.kind().String() + " "
	case *setClause, *valuesClause, *orderClause:
		return ", "
	default:
		return " "
	}
}

// buildInitial builds up the initial query using ? as the placeholder. This
// will correctly wrap the portions of the query in parentheses depending on the
// clauses in the query, and how these clauses are conjoined.
func (q *Query) buildInitial() string {
	var buf strings.Builder

	if q.stmt > 0 {
		buf.WriteString(q.stmt.String())
	}

	switch q.stmt {
	case insertStmt:
		buf.WriteString(" INTO ")
		buf.WriteString(q.table)
	case updateStmt:
		buf.WriteByte(' ')
		buf.WriteString(q.table)
		buf.WriteByte(' ')
	case deleteStmt:
		buf.WriteString(" FROM ")
		buf.WriteString(q.table)
		buf.WriteByte(' ')
	}

	for i, expr := range q.exprs {
		buf.WriteByte(' ')

		if q.stmt == insertStmt {
			buf.WriteByte('(')
		}

		buf.WriteString(expr.Build())

		if q.stmt == insertStmt {
			buf.WriteByte(')')
		}

		if q.stmt == selectDistinctOnStmt && i == 0 {
			continue
		}
		buf.WriteByte(' ')
	}

	clauses := make(map[clauseKind]struct{})

	for i, cl := range q.clauses {
		var prev, next clause

		if i > 0 {
			prev = q.clauses[i-1]
		}

		if i < len(q.clauses)-1 {
			next = q.clauses[i+1]
		}

		kind := cl.kind()

		if kind != _unionClause {
			// Write the string of the clause kind only once, this avoids multiple
			// clause strings being built into the query.
			if _, ok := clauses[kind]; !ok {
				clauses[kind] = struct{}{}

				buf.WriteString(kind.String())
				buf.WriteByte(' ')

				if kind == _whereClause {
					buf.WriteByte('(')
				}
			}
		}

		buf.WriteString(cl.Build())

		if next != nil {
			conj := q.conj(next)

			// Clauses are wrapped if the next clause is different from the current one,
			// or if the conjoining string is different for the next clause.
			if next.kind() == kind {
				wrap := false

				if prev != nil {
					// Wrap the clause in parentheses if we have a different
					// conjunction string.
					wrap = (prev.kind() == kind) && (conj != q.conj(cl))
				}

				if wrap {
					buf.WriteByte(')')
				}

				buf.WriteString(conj)

				if wrap {
					buf.WriteByte('(')
				}
			} else {
				if kind == _whereClause {
					buf.WriteByte(')')
				}
				buf.WriteByte(' ')
			}
		}

		if i == len(q.clauses)-1 && kind == _whereClause {
			buf.WriteByte(')')
		}
	}
	return buf.String()
}

func (q *Query) Build() string {
	s := q.buildInitial()

	query := make([]byte, 0, len(s))
	param := int64(0)

	for i := strings.Index(s, "?"); i != -1; i = strings.Index(s, "?") {
		param++

		query = append(query, s[:i]...)
		query = append(query, '$')
		query = strconv.AppendInt(query, param, 10)

		s = s[i+1:]
	}
	return string(append(query, []byte(s)...))
}
