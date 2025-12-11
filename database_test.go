package database

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/andrewpillar/database/query"

	_ "modernc.org/sqlite"
)

var sqlitePragmas = [...]string{
	"busy_timeout=5000",
	"cache_size=1000000000",
	"foreign_keys=true",
	"journal_mode=WAL",
	"synchronous=NORMAL",
	"temp_store=memory",
}

func NewDB(t *testing.T) *sql.DB {
	t.Helper()

	name := fmt.Sprintf("%s.sqlite", t.Name())

	url, err := url.Parse(name)

	if err != nil {
		t.Fatalf("url.Parse(%q): %v\n", name, err)
	}

	q := url.Query()

	for _, pragma := range sqlitePragmas {
		q.Add("_pragma", pragma)
	}

	url.RawQuery = q.Encode()

	db, err := sql.Open("sqlite", url.String())

	if err != nil {
		t.Fatalf("sql.Open(%q, %q): %v\n", "sqlite", t.Name(), err)
	}

	t.Cleanup(func() {
		db.Close()

		if !t.Failed() {
			os.Remove(name)
			return
		}

		t.Helper()
		t.Log("database at:", name)
	})

	return db
}

const modelSchema = `CREATE TABLE IF NOT EXISTS models (
	id        INTEGER UNIQUE NOT NULL,
	str       VARCHAR NOT NULL,
	bigstr    TEXT NOT NULL,
	int       INTEGER NOT NULL,
	bigint    INTEGER NOT NULL,
	bool      BOOLEAN NOT NULL,
	blob      BLOB NOT NULL,
	time      TIMESTAMP NOT NULL,
	null_time TIMESTAMP NULL,
	PRIMARY KEY (id)
);`

type M struct {
	ID       int64
	Str      string
	BigStr   string
	Int      int
	BigInt   int64
	Bool     bool
	Blob     []byte
	Time     time.Time
	NullTime sql.Null[time.Time] `db:"null_time"`
}

func (m *M) Table() string { return "models" }

func (m *M) PrimaryKey() *PrimaryKey {
	return &PrimaryKey{
		Columns: []string{"id"},
		Values:  []any{m.ID},
	}
}

func (m *M) Params() Params {
	return Params{
		"id":        CreateOnlyParam(m.ID),
		"str":       MutableParam(m.Str),
		"bigstr":    MutableParam(m.BigStr),
		"int":       MutableParam(m.Int),
		"bigint":    MutableParam(m.BigInt),
		"bool":      MutableParam(m.Bool),
		"blob":      MutableParam(m.Blob),
		"time":      CreateOnlyParam(m.Time),
		"null_time": UpdateOnlyParam(m.NullTime),
	}
}

func TestStore(t *testing.T) {
	ctx := t.Context()
	db := NewDB(t)

	if _, err := db.ExecContext(ctx, modelSchema); err != nil {
		t.Fatalf("db.ExecContext(ctx, %q): %v\n", modelSchema, err)
	}

	n := 10
	mm := make([]*M, 0, n)

	for i := 0; i < cap(mm); i++ {
		blob := make([]byte, 16)

		if _, err := rand.Read(blob); err != nil {
			t.Fatalf("rand.Read(blob): %v\n", err)
		}

		mm = append(mm, &M{
			ID:     int64(i),
			Str:    "string",
			BigStr: "bigstring",
			Int:    1 * i,
			BigInt: 32 << i,
			Bool:   true,
			Blob:   blob,
			Time:   time.Now(),
		})
	}

	store := NewStore[*M](db, func() *M {
		return &M{}
	})

	if err := store.Create(ctx); err != nil {
		t.Fatalf("store.Create(ctx): %v\n", err)
	}

	if err := store.Create(ctx, mm...); err != nil {
		t.Fatalf("store.Create(ctx, mm...): %v\n", err)
	}

	q := query.Select(
		query.Count("id"),
		query.From("models"),
	)

	rows, err := store.QueryContext(ctx, q.Build(), q.Args()...)

	if err != nil {
		t.Fatalf("store.QueryContext(ctx, %q, q.Args()...): %v\n", q.Build(), err)
	}

	var count int64

	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			t.Fatalf("rows.Scan(&count): %v\n", err)
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(): %v\n", err)
	}

	rows.Close()

	if n := int64(cap(mm)); count != n {
		t.Fatalf("count = %v, want = %v\n", count, n)
	}

	m := mm[0]
	originalTime := m.Time

	m.Time = time.Unix(0, 0)

	if _, err := store.Update(ctx, m); err != nil {
		t.Fatalf("store.Update(ctx, m): %v\n", err)
	}

	m, ok, err := store.Get(ctx, m.PrimaryKey().Where())

	if err != nil {
		t.Fatalf("store.Get(ctx, m.PrimaryKey().Where()): %v\n", err)
	}

	if !ok {
		t.Fatalf("ok = %v, want = %v\n", ok, true)
	}

	if m.Time.Equal(time.Unix(0, 0)) {
		t.Fatalf("m.Time = %v, want = %v\n", m.Time, originalTime)
	}

	now := time.Now()

	fields := map[string]any{
		"id":           10,
		"null_time":    now,
		"non_existent": 12345,
	}

	where := query.WhereIn("id", List("id", mm...))

	if _, err := store.UpdateMany(ctx, fields, where); err != nil {
		t.Fatalf("store.UpdateMany(ctx, fields, where): %v\n", err)
	}

	mm2, err := store.Select(ctx, query.Columns("*"))

	if err != nil {
		t.Fatalf("store.Select(ctx, query.Columns(%q)): %v\n", "*", err)
	}

	for i, m := range mm2 {
		if m.ID == 10 {
			t.Errorf("mm2[%v] = %v, want = %v\n", i, m.ID, mm[i].ID)
		}

		if !m.NullTime.V.Equal(now) {
			t.Errorf("mm2[%v] = %v, want = %v\n", i, m.NullTime.V, now)
		}
	}

	if _, err := store.Delete(ctx); err != nil {
		t.Fatalf("store.Delete(ctx): %v\n", err)
	}

	rows2, err := store.QueryContext(ctx, q.Build(), q.Args()...)

	if err != nil {
		t.Fatalf("store.QueryContext(ctx, %q, q.Args()...): %v\n", q.Build(), err)
	}

	if rows2.Next() {
		if err := rows2.Scan(&count); err != nil {
			t.Fatalf("rows2.Scan(&count): %v\n", err)
		}
	}

	if err := rows2.Err(); err != nil {
		t.Fatalf("rows2.Err(): %v\n", err)
	}

	rows2.Close()

	if count == 0 {
		t.Fatal("count == 0")
	}

	if _, err := store.Delete(ctx, mm...); err != nil {
		t.Fatalf("store.Delete(ctx, mm...): %v\n", err)
	}
}

func TestStoreTx(t *testing.T) {
	ctx := t.Context()
	db := NewDB(t)

	if _, err := db.ExecContext(ctx, modelSchema); err != nil {
		t.Fatalf("db.ExecContext(ctx, %q): %v\n", modelSchema, err)
	}

	tx, err := db.BeginTx(ctx, nil)

	if err != nil {
		t.Fatalf("db.BeginTx(ctx, nil): %v\n", err)
	}

	defer tx.Rollback()

	store := NewStore[*M](db, func() *M {
		return &M{}
	})

	for i := 0; i < 10; i++ {
		blob := make([]byte, 16)

		if _, err := rand.Read(blob); err != nil {
			t.Fatalf("rand.Read(blob): %v\n", err)
		}

		m := M{
			ID:     int64(i),
			Str:    "string",
			BigStr: "bigstring",
			Int:    1 * i,
			BigInt: 32 << i,
			Bool:   true,
			Blob:   blob,
			Time:   time.Now(),
		}

		if err := store.CreateTx(ctx, tx, &m); err != nil {
			t.Fatalf("store.CreateTx(ctx, tx, &m): %v\n", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("tx.Commit(): %v\n", err)
	}

	mm, err := store.Select(ctx, query.Columns("*"))

	if err != nil {
		t.Fatalf("store.Select(ctx, query.Columns(%q)): %v\n", "*", err)
	}

	tx2, err := db.BeginTx(ctx, nil)

	if err != nil {
		t.Fatalf("db.BeginTx(ctx, nil): %v\n", err)
	}

	defer tx2.Rollback()

	m, ok, err := store.Get(ctx, query.WhereEq("id", query.Arg(0)))

	if err != nil {
		t.Fatalf("store.Get(ctx, m.PrimaryKey().Where()): %v\n", err)
	}

	if !ok {
		t.Fatalf("ok = %v, want = %v\n", ok, true)
	}

	originalTime := m.Time
	m.Time = time.Unix(0, 0)

	if _, err := store.UpdateTx(ctx, tx2, m); err != nil {
		t.Fatalf("store.Update(ctx, tx2, m): %v\n", err)
	}

	m, ok, err = store.Get(ctx, m.PrimaryKey().Where())

	if err != nil {
		t.Fatalf("store.Get(ctx, m.PrimaryKey().Where()): %v\n", err)
	}

	if !ok {
		t.Fatalf("ok = %v, want = %v\n", ok, true)
	}

	if m.Time.Equal(time.Unix(0, 0)) {
		t.Fatalf("m.Time = %v, want = %v\n", m.Time, originalTime)
	}

	if err := tx2.Commit(); err != nil {
		t.Fatalf("tx2.Commit(): %v\n", err)
	}

	tx3, err := db.BeginTx(ctx, nil)

	if err != nil {
		t.Fatalf("db.BeginTx(ctx, nil): %v\n", err)
	}

	defer tx3.Rollback()

	now := time.Now()

	fields := map[string]any{
		"id":           10,
		"null_time":    now,
		"non_existent": 12345,
	}

	where := query.WhereIn("id", List("id", mm...))

	if _, err := store.UpdateManyTx(ctx, tx3, fields, where); err != nil {
		t.Fatalf("store.UpdateManyTx(ctx, tx3, fields, where): %v\n", err)
	}

	if err := tx3.Commit(); err != nil {
		t.Fatalf("tx3.Commit(): %v\n", err)
	}

	mm2, err := store.Select(ctx, query.Columns("*"))

	if err != nil {
		t.Fatalf("store.Select(ctx, query.Columns(%q)): %v\n", "*", err)
	}

	for i, m := range mm2 {
		if m.ID == 10 {
			t.Errorf("mm2[%v] = %v, want = %v\n", i, m.ID, mm[i].ID)
		}

		if !m.NullTime.V.Equal(now) {
			t.Errorf("mm2[%v] = %v, want = %v\n", i, m.NullTime.V, now)
		}
	}

	tx4, err := db.BeginTx(ctx, nil)

	if err != nil {
		t.Fatalf("db.BeginTx(ctx, nil): %v\n", err)
	}

	defer tx4.Rollback()

	if _, err := store.DeleteTx(ctx, tx4, mm...); err != nil {
		t.Fatalf("store.DeleteTx(ctx, tx4, mm...): %v\n", err)
	}

	if err := tx4.Commit(); err != nil {
		t.Fatalf("tx4.Commit(): %v\n", err)
	}
}

const userPostSchema = `
CREATE TABLE IF NOT EXISTS users (
	id    INTEGER UNIQUE NOT NULL,
	email VARCHAR UNIQUE NOT NULL,
	PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS posts (
	id      INTEGER UNIQUE NOT NULL,
	user_id INTEGER NOT NULL,
	title   TEXT NOT NULL,
	PRIMARY KEY (id),
	FOREIGN KEY (user_id) REFERENCES users(id)
);
`

type User struct {
	ID    int64
	Email string
}

func (u *User) Table() string { return "users" }

func (u *User) PrimaryKey() *PrimaryKey {
	return &PrimaryKey{
		Columns: []string{"id"},
		Values:  []any{u.ID},
	}
}

func (u *User) Params() Params {
	return Params{
		"id":    CreateOnlyParam(u.ID),
		"email": MutableParam(u.Email),
	}
}

type Post struct {
	ID    int64
	User  *User `db:"user_id:id,users.*:*"`
	Title string
}

func (p *Post) Table() string { return "posts" }

func (p *Post) PrimaryKey() *PrimaryKey {
	return &PrimaryKey{
		Columns: []string{"id"},
		Values:  []any{p.ID},
	}
}

func (p *Post) Params() Params {
	return Params{
		"id":      CreateOnlyParam(p.ID),
		"user_id": CreateOnlyParam(p.User.ID),
		"title":   MutableParam(p.Title),
	}
}

func RandomUser(t *testing.T, users *Store[*User]) *User {
	t.Helper()

	q := "SELECT * FROM users ORDER BY RANDOM() LIMIT 1"

	rows, err := users.QueryContext(t.Context(), q)

	if err != nil {
		t.Fatalf("users.QueryContext(t.Context(), %q): %v\n", q, err)
	}

	defer rows.Close()

	sc, err := NewScanner(rows)

	if err != nil {
		t.Fatalf("NewScanner(rows): %v\n", err)
	}

	uu := make([]*User, 0, 1)

	for rows.Next() {
		u := users.new()

		if err := sc.Scan(u); err != nil {
			t.Fatalf("sc.Scan(u): %v\n", err)
		}
		uu = append(uu, u)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(): %v\n", err)
	}
	return uu[0]
}

func TestRelations(t *testing.T) {
	ctx := t.Context()
	db := NewDB(t)

	if _, err := db.ExecContext(ctx, userPostSchema); err != nil {
		t.Fatalf("db.ExecContext(ctx, %q): %v\n", modelSchema, err)
	}

	users := NewStore(db, func() *User {
		return &User{}
	})

	posts := NewStore(db, func() *Post {
		return &Post{
			User: &User{},
		}
	})

	for i := 0; i < 3; i++ {
		u := User{
			ID:    int64(i),
			Email: rand.Text(),
		}

		if err := users.Create(ctx, &u); err != nil {
			t.Fatalf("users.Create(ctx, &u): %v\n", err)
		}
	}

	for i := 0; i < 10; i++ {
		u := RandomUser(t, users)

		p := Post{
			ID:    int64(i),
			User:  u,
			Title: fmt.Sprintf("Post %d", i+1),
		}

		if err := posts.Create(ctx, &p); err != nil {
			t.Fatalf("posts.Create(ctx, &p): %v\n", err)
		}
	}

	pp, err := posts.Select(
		ctx,
		Columns(&Post{User: &User{}}, &User{}),
		Join(&User{}, "user_id"),
	)

	if err != nil {
		t.Fatalf("posts.Select(ctx, Columns(&Post{}, &User{}), Join(&Post{}, &User{})): %v\n", err)
	}

	for _, p := range pp {
		u, ok, err := users.Get(ctx, p.User.PrimaryKey().Where())

		if err != nil {
			t.Fatalf("users.Get(ctx, p.User.PrimaryKey().Where()): %v\n", err)
		}

		if !ok {
			t.Fatalf("ok = %v, want = %v\n", ok, true)
		}

		if *p.User != *u {
			t.Fatalf("p.User = %v, want = %v\n", p.User, u)
		}
	}
}
