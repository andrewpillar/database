package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/andrewpillar/database"
	"github.com/andrewpillar/database/query"

	_ "modernc.org/sqlite"
)

var SQLitePragmas = [...]string{
	"busy_timeout=5000",
	"cache_size=1000000000",
	"foreign_keys=true",
	"journal_mode=WAL",
	"synchronous=NORMAL",
	"temp_store=memory",
}

func OpenDB() (*sql.DB, error) {
	dbname := fmt.Sprintf("%s.sqlite", os.Args[0])

	url, err := url.Parse(dbname)

	if err != nil {
		return nil, err
	}

	q := url.Query()

	for _, pragma := range SQLitePragmas {
		q.Add("_pragma", pragma)
	}

	url.RawQuery = q.Encode()

	db, err := sql.Open("sqlite", url.String())

	if err != nil {
		return nil, err
	}
	return db, nil
}

//go:embed home.tmpl
var homeTmpl []byte

func main() {
	db, err := OpenDB()

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, schema := range []string{
		UserSchema,
		PostSchema,
	} {
		if _, err := db.ExecContext(ctx, schema); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	tmpl, err := template.New("home.tmpl").Parse(string(homeTmpl))

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	users := database.NewStore(db, func() *User {
		return &User{}
	})

	for _, username := range DefaultUsers {
		_, ok, err := users.Get(ctx, query.WhereEq("username", query.Arg(username)))

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if !ok {
			now := time.Now().UTC()

			u := User{
				ID:        now.UnixNano(),
				Username:  username,
				CreatedAt: now,
			}

			if err := users.Create(ctx, &u); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}

	posts := database.NewStore(db, func() *Post {
		return &Post{
			User: &User{},
		}
	})

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", HomeHandler(tmpl, posts, users))
	mux.HandleFunc("POST /posts", CreatePostHandler(posts, users))

	srv := http.Server{
		Addr:    "localhost:8000",
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch

	srv.Shutdown(ctx)
}
