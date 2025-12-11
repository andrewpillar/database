package main

import (
	"time"

	"github.com/andrewpillar/database"
)

const UserSchema = `CREATE TABLE IF NOT EXISTS users (
	id         INTEGER NOT NULL,
	username   VARCHAR UNIQUE NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY (id)
);`

type User struct {
	ID        int64
	Username  string
	CreatedAt time.Time `db:"created_at"`
}

var DefaultUsers = [...]string{
	"brian",
	"rob",
	"ken",
}

func (u *User) Table() string { return "users" }

func (u *User) PrimaryKey() *database.PrimaryKey {
	return &database.PrimaryKey{
		Columns: []string{"id"},
		Values:  []any{u.ID},
	}
}

func (u *User) Params() database.Params {
	return database.Params{
		"id":         database.CreateOnlyParam(u.ID),
		"username":   database.CreateOnlyParam(u.Username),
		"created_at": database.CreateOnlyParam(u.CreatedAt),
	}
}
