package database

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewpillar/database/query"
)

type M2 struct {
	*M `db:"*:*"`
}

func TestScanEmbed(t *testing.T) {
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

	if err := store.Create(ctx, mm...); err != nil {
		t.Fatalf("store.Create(ctx, mm...): %v\n", err)
	}

	store2 := NewStore[*M2](db, func() *M2 {
		return &M2{
			M: &M{},
		}
	})

	mm2, err := store2.Select(ctx, query.Columns("*"))

	if err != nil {
		t.Fatalf("store2.Select(ctx, query.Columns(%q)): %v\n", "*", err)
	}
	t.Log(mm2)
}

type I int

type Number struct {
	I      I
	Uint   uint
	Uint8  uint8
	Uint16 uint16
	Uint32 uint32
	Uint64 uint64

	Float32 float32
	Float64 float64
}

const numberSchema = `CREATE TABLE IF NOT EXISTS numbers (
	i       INTEGER NOT NULL,
	uint    INTEGER NOT NULL,
	uint8   INTEGER NOT NULL,
	uint16  INTEGER NOT NULL,
	uint32  INTEGER NOT NULL,
	uint64  INTEGER NOT NULL,
	float32 REAL NOT NULL,
	float64 REAL NOT NULL
);`

func (n *Number) Table() string { return "numbers" }

func (n *Number) PrimaryKey() *PrimaryKey { return nil }

func (n *Number) Params() Params {
	return Params{
		"i":       CreateOnlyParam(n.I),
		"uint":    CreateOnlyParam(n.Uint),
		"uint8":   CreateOnlyParam(n.Uint8),
		"uint16":  CreateOnlyParam(n.Uint16),
		"uint32":  CreateOnlyParam(n.Uint32),
		"uint64":  CreateOnlyParam(n.Uint64),
		"float32": CreateOnlyParam(n.Float32),
		"float64": CreateOnlyParam(n.Float64),
	}
}

func TestNumberScanning(t *testing.T) {
	ctx := t.Context()
	db := NewDB(t)

	if _, err := db.ExecContext(ctx, numberSchema); err != nil {
		t.Fatalf("db.ExecContext(ctx, %q): %v\n", numberSchema, err)
	}

	store := NewStore[*Number](db, func() *Number {
		return &Number{}
	})

	for i := 0; i < 10; i++ {
		n := Number{
			I:       I(i),
			Uint:    uint(i),
			Uint8:   uint8(i),
			Uint16:  uint16(i),
			Uint32:  uint32(i),
			Uint64:  uint64(i),
			Float32: float32(32.123),
			Float64: float64(64.123),
		}

		if err := store.Create(ctx, &n); err != nil {
			t.Fatalf("store.Create(ctx, &n): %v\n", err)
		}
	}

	for i := 0; i < 10; i++ {
		n, ok, err := store.Get(ctx, query.WhereEq("i", query.Arg(i)))

		if err != nil {
			t.Fatalf("store.Get(ctx, query.WhereEq(%q, query.Arg(%v))): %v\n", "i", i, err)
		}

		if !ok {
			t.Fatalf("ok = %v, want = %v\n", ok, true)
		}

		if n.I != I(i) {
			t.Fatalf("n.I = %v, want = %v\n", n.I, i)
		}

		if n.Uint != uint(i) {
			t.Fatalf("n.Uint = %v, want = %v\n", n.Uint, i)
		}

		if n.Uint8 != uint8(i) {
			t.Fatalf("n.Uint8 = %v, want = %v\n", n.Uint8, i)
		}

		if n.Uint16 != uint16(i) {
			t.Fatalf("n.Uint16 = %v, want = %v\n", n.Uint16, i)
		}

		if n.Uint32 != uint32(i) {
			t.Fatalf("n.Uint32 = %v, want = %v\n", n.Uint32, i)
		}

		if n.Uint64 != uint64(i) {
			t.Fatalf("n.Uint64 = %v, want = %v\n", n.Uint64, i)
		}

		if n.Float32 != 32.123 {
			t.Fatalf("n.Float32 = %v, want = %v\n", n.Float32, 32.123)
		}

		if n.Float64 != 64.123 {
			t.Fatalf("n.Float64 = %v, want = %v\n", n.Float64, 32.123)
		}
	}
}

type Pointer struct {
	Time *time.Time
}

func (p *Pointer) Table() string { return "pointers" }

func (p *Pointer) PrimaryKey() *PrimaryKey { return nil }

func (p *Pointer) Params() Params {
	return Params{
		"time": CreateOnlyParam(p.Time),
	}
}

const pointerSchema = `CREATE TABLE IF NOT EXISTS pointers (
	time TIMESTAMP NOT NULL
);`

func TestPointerScanning(t *testing.T) {
	ctx := t.Context()
	db := NewDB(t)

	if _, err := db.ExecContext(ctx, pointerSchema); err != nil {
		t.Fatalf("db.ExecContext(ctx, %q): %v\n", pointerSchema, err)
	}

	store := NewStore(db, func() *Pointer {
		return &Pointer{}
	})

	now := time.Now()

	p := &Pointer{
		Time: &now,
	}

	if err := store.Create(ctx, p); err != nil {
		t.Fatalf("store.Create(ctx, p): %v\n", err)
	}

	p, ok, err := store.Get(ctx)

	if err != nil {
		t.Fatalf("store.Get(ctx): %v\n", err)
	}

	if !ok {
		t.Fatalf("ok = %v, want = %v\n", ok, true)
	}

	if !p.Time.Equal(now) {
		t.Fatalf("p.Time = %v, want = %v\n", p.Time, now)
	}
}

type Data map[string]any

func (d Data) Value() (driver.Value, error) {
	return json.Marshal(d)
}

type Notification struct {
	ID   int64
	Data Data
}

func (n *Notification) Table() string { return "notifications" }

func (n *Notification) PrimaryKey() *PrimaryKey {
	return &PrimaryKey{
		Columns: []string{"id"},
		Values:  []any{n.ID},
	}
}

func (n *Notification) Params() Params {
	return Params{
		"id":   CreateOnlyParam(n.ID),
		"data": CreateOnlyParam(n.Data),
	}
}

func (n *Notification) Scan(r *Row) error {
	var data string

	err := r.Scan(map[string]any{
		"id":   &n.ID,
		"data": &data,
	})

	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), &n.Data)
}

const notificationSchema = `CREATE TABLE IF NOT EXISTS notifications (
	id   INTEGER NOT NULL,
	data TEXT NOT NULL,
	PRIMARY KEY (id)
);`

func TestRowScanner(t *testing.T) {
	ctx := t.Context()
	db := NewDB(t)

	if _, err := db.ExecContext(ctx, notificationSchema); err != nil {
		t.Fatalf("db.ExecContext(ctx, %q): %v\n", notificationSchema, err)
	}

	store := NewStore(db, func() *Notification {
		return &Notification{}
	})

	n := &Notification{
		ID: 10,
		Data: Data{
			"field": "value",
			"object": map[string]any{
				"field": 10,
			},
		},
	}

	if err := store.Create(ctx, n); err != nil {
		t.Fatalf("store.Create(ctx, n): %v\n", err)
	}

	n, ok, err := store.Get(ctx)

	if err != nil {
		t.Fatalf("store.Get(ctx): %v\n", err)
	}

	if !ok {
		t.Fatalf("ok = %v, want = %v\n", ok, true)
	}
	t.Log(n.Data)
}
