package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/andrewpillar/database"
	"github.com/andrewpillar/database/query"
)

const PostSchema = `CREATE TABLE IF NOT EXISTS posts (
	id         INTEGER NOT NULL,
	user_id    INTEGER NOT NULL,
	title      VARCHAR NOT NULL,
	content    TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NULL,
	PRIMARY KEY (id),
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS post_tags (
	post_id VARCHAR NOT NULL,
	name    VARCHAR NOT NULL,
	PRIMARY KEY (post_id, name)
);`

type Post struct {
	ID        int64
	User      *User `db:"user_id:id,users.*:*"`
	Title     string
	Content   string
	CreatedAt time.Time           `db:"created_at"`
	UpdatedAt sql.Null[time.Time] `db:"updated_at"`
	Tags      []string            `db:"-"`
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
		"user_id":    database.CreateOnlyParam(p.User.ID),
		"title":      database.CreateOnlyParam(p.Title),
		"content":    database.MutableParam(p.Content),
		"created_at": database.CreateOnlyParam(p.CreatedAt),
		"updated_at": database.UpdateOnlyParam(p.UpdatedAt),
	}
}

func LoadTags(ctx context.Context, db *sql.DB, pp []*Post) error {
	// Table to look up the post's position in the given slice. The key is the
	// post's ID.
	tab := make(map[int64]int)

	for i, p := range pp {
		tab[p.ID] = i
	}

	q := query.Select(
		query.Columns("post_id", "name"),
		query.From("post_tags"),
		query.WhereIn("post_id", database.List("id", pp...)),
	)

	rows, err := db.QueryContext(ctx, q.Build(), q.Args()...)

	if err != nil {
		return err
	}

	defer rows.Close()

	var (
		postId int64
		tag    string
	)

	for rows.Next() {
		if err := rows.Scan(&postId, &tag); err != nil {
			return err
		}

		if pos, ok := tab[postId]; ok {
			pp[pos].Tags = append(pp[pos].Tags, tag)
		}
	}
	return nil
}

func WhereTag(tag string) query.Option {
	return func(q *query.Query) *query.Query {
		// We specify the column as "posts.id" because of its use in a JOIN
		// column names will end up being ambiguous.
		return query.WhereIn("posts.id", query.Select(
			query.Columns("post_id"),
			query.From("post_tags"),
			query.WhereEq("name", query.Arg(tag)),
		))(q)
	}
}
