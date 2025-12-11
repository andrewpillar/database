package main

import (
	"errors"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/andrewpillar/database"
	"github.com/andrewpillar/database/query"
)

func BadRequest(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, err.Error())
}

func InternalServerError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	io.WriteString(w, err.Error())
}

func HomeHandler(tmpl *template.Template, posts *database.Store[*Post], users *database.Store[*User]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		uu, err := users.Select(ctx, query.Columns("*"))

		if err != nil {
			InternalServerError(w, err)
			return
		}

		p := &Post{
			User: &User{},
		}

		opts := []query.Option{
			database.Join(p.User, "user_id"),
		}

		if tag := r.URL.Query().Get("tag"); tag != "" {
			opts = append(opts, WhereTag(tag))
		}

		pp, err := posts.Select(ctx, database.Columns(p, p.User), opts...)

		if err != nil {
			InternalServerError(w, err)
			return
		}

		if err := LoadTags(ctx, posts.DB, pp); err != nil {
			InternalServerError(w, err)
			return
		}

		var data struct {
			Users []*User
			Posts []*Post
		}

		data.Users = uu
		data.Posts = pp

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, data)
	}
}

func CreatePostHandler(posts *database.Store[*Post], users *database.Store[*User]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			InternalServerError(w, err)
			return
		}

		ctx := r.Context()

		// Contrived example, just pretend this is retrieved from a cookie in
		// the request as would normally be done in an _actual_ application.
		u, ok, err := users.Get(ctx, query.WhereEq("id", query.Arg(r.PostForm.Get("user_id"))))

		if err != nil {
			InternalServerError(w, err)
			return
		}

		if !ok {
			BadRequest(w, errors.New("no user found"))
			return
		}

		now := time.Now()

		p := Post{
			ID:        now.UnixNano(),
			User:      u,
			Title:     r.PostForm.Get("title"),
			Content:   r.PostForm.Get("content"),
			CreatedAt: now,
		}

		if err := posts.Create(ctx, &p); err != nil {
			InternalServerError(w, err)
			return
		}

		tags := strings.Split(r.PostForm.Get("tags"), ",")

		for _, tag := range tags {
			q := query.Insert("post_tags", query.Columns("post_id", "name"), query.Values(p.ID, tag))

			if _, err := posts.DB.ExecContext(ctx, q.Build(), q.Args()...); err != nil {
				InternalServerError(w, err)
				return
			}
		}
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
	}
}
