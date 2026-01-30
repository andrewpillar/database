package query

import "testing"

func Test_Query(t *testing.T) {
	tests := []struct {
		want  string
		nargs int
		query *Query
	}{
		{
			"SELECT SUM(size) FROM files WHERE (user_id = $1)",
			1,
			Select(Sum(Ident("size")), From("files"), WhereEq("user_id", Arg(1))),
		},
		{
			"SELECT COUNT(*) FROM files",
			0,
			Select(Count("*"), From("files")),
		},
		{
			"SELECT COUNT(id) FROM files",
			0,
			Select(Count("id"), From("files")),
		},
		{
			"SELECT * FROM users WHERE (email = $1 AND deleted_at IS NULL)",
			1,
			Select(
				Columns("*"),
				From("users"),
				WhereEq("email", Arg("email@domain.com")),
				WhereIsNil("deleted_at"),
			),
		},
		{
			"SELECT * FROM users WHERE (email = $1 OR username = $2) AND (deleted_at IS NULL)",
			2,
			Select(
				Columns("*"),
				From("users"),
				WhereEq("email", Arg("email@domain.com")),
				OrWhereEq("username", Arg("username")),
				WhereIsNil("deleted_at"),
			),
		},
		{
			"SELECT * FROM posts WHERE (title LIKE $1) LIMIT 25 OFFSET 2",
			1,
			Select(
				Columns("*"),
				From("posts"),
				WhereLike("title", Arg("%foo%")),
				Limit(int64(25)),
				Offset(int64(2)),
			),
		},
		{
			"SELECT * FROM posts WHERE (user_id = $1 AND id IN (SELECT post_id FROM post_tags WHERE (name LIKE $2)))",
			2,
			Select(
				Columns("*"),
				From("posts"),
				WhereEq("user_id", Arg(1)),
				WhereIn("id", Select(
					Columns("post_id"),
					From("post_tags"),
					WhereLike("name", Arg("%foo%")),
				)),
			),
		},
		{
			"SELECT * FROM posts WHERE (user_id = $1 AND id IN (SELECT post_id FROM post_tags WHERE (name LIKE $2)) AND category_id IN (SELECT id FROM post_categories WHERE (name LIKE $3)))",
			3,
			Select(
				Columns("*"),
				From("posts"),
				WhereEq("user_id", Arg(1)),
				WhereIn("id", Select(
					Columns("post_id"),
					From("post_tags"),
					WhereLike("name", Arg("%foo%")),
				)),
				WhereIn("category_id", Select(
					Columns("id"),
					From("post_categories"),
					WhereLike("name", Arg("%foo%")),
				)),
			),
		},
		{
			"SELECT * FROM posts WHERE (user_id = $1 AND id IN (SELECT post_id FROM post_tags WHERE (name LIKE $2)) AND category_id IN (SELECT id FROM post_categories WHERE (name LIKE $3)))",
			3,
			Select(
				Columns("*"),
				From("posts"),
				Options(
					WhereEq("user_id", Arg(1)),
					WhereIn("id", Select(
						Columns("post_id"),
						From("post_tags"),
						WhereLike("name", Arg("%foo%")),
					)),
					WhereIn("category_id", Select(
						Columns("id"),
						From("post_categories"),
						WhereLike("name", Arg("%foo%")),
					)),
				),
			),
		},
		{
			"SELECT * FROM users WHERE (id IN ($1))",
			1,
			Select(Columns("*"), From("users"), WhereIn("id", List(1))),
		},
		{
			"SELECT * FROM users WHERE (id IN ($1, $2, $3, $4))",
			4,
			Select(Columns("*"), From("users"), WhereIn("id", List(1, 2, 3, 4))),
		},
		{
			"SELECT * FROM variables WHERE (namespace_id IN (SELECT id FROM namespaces WHERE (root_id IN (SELECT namespace_id FROM namespace_collaborators WHERE (user_id = $1) UNION SELECT id FROM namespaces WHERE (user_id = $2)))) OR user_id = $3)",
			3,
			Select(
				Columns("*"),
				From("variables"),
				WhereIn("namespace_id",
					Select(
						Columns("id"),
						From("namespaces"),
						WhereIn("root_id",
							Union(
								Select(
									Columns("namespace_id"),
									From("namespace_collaborators"),
									WhereEq("user_id", Arg(2)),
								),
								Select(
									Columns("id"),
									From("namespaces"),
									WhereEq("user_id", Arg(2)),
								),
							),
						),
					),
				),
				OrWhereEq("user_id", Arg(2)),
			),
		},
		{
			"INSERT INTO users (email, username, password) VALUES ($1, $2, $3)",
			3,
			Insert(
				"users",
				Columns("email", "username", "password"),
				Values("email@domain.com", "user", "secret"),
			),
		},
		{
			"INSERT INTO users (email, username, password) VALUES ($1, $2, $3) RETURNING id, created_at",
			3,
			Insert(
				"users",
				Columns("email", "username", "password"),
				Values("email@domain.com", "user", "secret"),
				Returning("id", "created_at"),
			),
		},
		{
			"INSERT INTO posts (title, body) VALUES ($1, $2), ($3, $4), ($5, $6)",
			6,
			Insert(
				"posts",
				Columns("title", "body"),
				Values("post 1", "post 1"),
				Values("post 2", "post 2"),
				Values("post 3", "post 3"),
			),
		},
		{
			"DELETE FROM users WHERE (id = $1)",
			1,
			Delete("users", WhereEq("id", Arg(10))),
		},
		{
			"DELETE FROM posts WHERE ((id, title) IN (($1, $2), ($3, $4)))",
			2,
			Delete(
				"posts",
				WhereIn(
					"(id, title)",
					List(
						List(1, "foo"),
						List(2, "bar"),
					),
				),
			),
		},
		{
			"SELECT * FROM posts ORDER BY created_at DESC, author ASC",
			0,
			Select(
				Columns("*"),
				From("posts"),
				OrderDesc("created_at"),
				OrderAsc("author"),
			),
		},
		{
			"SELECT DISTINCT name FROM post_tags WHERE (post_id = $1)",
			1,
			SelectDistinct(
				Columns("name"),
				From("post_tags"),
				WhereEq("post_id", Arg(1)),
			),
		},
		{
			"SELECT DISTINCT ON (namespace_id) id, namespace_id FROM builds ORDER BY created_at DESC",
			0,
			SelectDistinctOn(
				List(Ident("namespace_id")),
				Columns("id", "namespace_id"),
				From("builds"),
				OrderDesc("created_at"),
			),
		},
		{
			"UPDATE t SET col = $1, updated_at = NOW() WHERE (id = $2)",
			2,
			Update(
				"t",
				Set("col", Arg("val")),
				Set("updated_at", Lit("NOW()")),
				WhereEq("id", Arg("id")),
			),
		},
		{
			"SELECT * FROM t WHERE (c != $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				WhereNotEq("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c > $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				WhereGt("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c >= $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				WhereGeq("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c < $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				WhereLt("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c <= $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				WhereLeq("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c IS NOT NULL)",
			0,
			Select(
				Columns("*"),
				From("t"),
				WhereIsNotNil("c"),
			),
		},
		{
			"SELECT * FROM t WHERE (c NOT IN ($1, $2, $3))",
			3,
			Select(
				Columns("*"),
				From("t"),
				WhereNotIn("c", List(1, 2, 3)),
			),
		},
		{
			"SELECT * FROM t WHERE (c != $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				OrWhereNotEq("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c > $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				OrWhereGt("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c >= $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				OrWhereGeq("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c < $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				OrWhereLt("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c <= $1)",
			1,
			Select(
				Columns("*"),
				From("t"),
				OrWhereLeq("c", Arg(1)),
			),
		},
		{
			"SELECT * FROM t WHERE (c IS NOT NULL)",
			0,
			Select(
				Columns("*"),
				From("t"),
				OrWhereIsNotNil("c"),
			),
		},
		{
			"SELECT * FROM t WHERE (c NOT IN ($1, $2, $3))",
			3,
			Select(
				Columns("*"),
				From("t"),
				OrWhereNotIn("c", List(1, 2, 3)),
			),
		},
		{
			"SELECT * FROM t WHERE (c IS $1 OR c IS $2)",
			2,
			Select(
				Columns("*"),
				From("t"),
				WhereIs("c", Arg(1)),
				OrWhereIs("c", Arg(2)),
			),
		},
		{
			"SELECT * FROM t WHERE (c IS $1 OR c IS NULL)",
			1,
			Select(
				Columns("*"),
				From("t"),
				WhereIs("c", Arg(1)),
				OrWhereIsNil("c"),
			),
		},
		{
			"SELECT * FROM t WHERE (c IS $1 OR c IN ($2, $3))",
			3,
			Select(
				Columns("*"),
				From("t"),
				WhereIs("c", Arg(1)),
				OrWhereIn("c", List(2, 3)),
			),
		},
		{
			"SELECT * FROM t WHERE (n > (SELECT COUNT(c) FROM t2))",
			0,
			Select(
				Columns("*"),
				From("t"),
				WhereGt("n", Select(
					Count("c"),
					From("t2"),
				)),
			),
		},
		{
			"SELECT id AS \"t.id\", timestamp AS \"t.timestamp\" FROM t",
			0,
			Select(
				Exprs(
					ColumnAs("id", "t.id"),
					ColumnAs("timestamp", "t.timestamp"),
				),
				From("t"),
			),
		},
		{
			"SELECT * FROM table AS t",
			0,
			Select(
				Columns("*"),
				FromAs("table", "t"),
			),
		},
		{
			"SELECT posts.id AS \"id\", posts.title AS \"title\", users.id AS \"user.id\" FROM posts JOIN users ON posts.user_id = users.id",
			0,
			Select(
				Exprs(
					ColumnAs("posts.id", "id"),
					ColumnAs("posts.title", "title"),
					ColumnAs("users.id", "user.id"),
				),
				From("posts"),
				Join("users", Eq(Ident("posts.user_id"), Ident("users.id"))),
			),
		},
		{
			"SELECT * FROM t1 JOIN t2 ON t1.fk_1 = t2.pk_1 AND t1.fk_2 = t2.pk_2",
			0,
			Select(
				Columns("*"),
				From("t1"),
				Join("t2", And(Eq(Ident("t1.fk_1"), Ident("t2.pk_1")), Eq(Ident("t1.fk_2"), Ident("t2.pk_2")))),
			),
		},
		{
			"SELECT * FROM t1 JOIN t2 ON t1.fk_1 = t2.pk_1 AND t1.fk_2 = t2.pk_2 AND t1.fk_3 = t2.pk_3",
			0,
			Select(
				Columns("*"),
				From("t1"),
				Join("t2", And(
					Eq(Ident("t1.fk_1"), Ident("t2.pk_1")),
					Eq(Ident("t1.fk_2"), Ident("t2.pk_2")),
					Eq(Ident("t1.fk_3"), Ident("t2.pk_3")),
				)),
			),
		},
		{
			"SELECT * FROM t WHERE (LOWER(col) = LOWER($1))",
			1,
			Select(
				Columns("*"),
				From("t"),
				Where(Eq(Lower(Ident("col")), Lower(Arg("string")))),
			),
		},
		{
			"SELECT * FROM t WHERE (LOWER(col) IN (LOWER($1), LOWER($2), LOWER($3)))",
			3,
			Select(
				Columns("*"),
				From("t"),
				Where(
					In(
						Lower(Ident("col")),
						List(
							Lower(Arg("val1")),
							Lower(Arg("val2")),
							Lower(Arg("val3")),
						)),
				),
			),
		},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			t.Parallel()

			got := test.query.Build()

			if test.want != got {
				t.Fatalf("test.query.Build() mismatch:\nwant = %q\ngot  = %q\n", test.want, got)
			}

			args := test.query.Args()

			if l := len(args); l != test.nargs {
				t.Fatalf("len(args) = %v, want = %v\n", l, test.nargs)
			}
		})
	}
}
