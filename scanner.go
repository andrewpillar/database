package database

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Row represents a single row from a set of multiple rows queried from the
// database.
type Row struct {
	scan func(dest ...any) error

	// List of column names for the row that has been queried.
	Columns []string
}

// Scan the rows data into the given map. The given map is expected to contain
// the column names that point to the pointers into which the row data is
// scanned. If a column name does not appear in the given map, then that value
// will not be scanned into the pointer.
func (r *Row) Scan(desttab map[string]any) error {
	dest := make([]any, 0, len(desttab))

	for _, col := range r.Columns {
		if d, ok := desttab[col]; ok {
			dest = append(dest, d)
		}
	}
	return r.scan(dest...)
}

// RowScanner is the interface that is used to allow for Models to define how
// row data should be scanned into them.
//
// Scan is given the [Row] that is currently being scanned from a set of rows.
// The implementation of Scan should scall the [Row.Scan] method, passing it a
// map of pointers into which the row data is scanned.
type RowScanner interface {
	Scan(r *Row) error
}

type structField struct {
	name string

	// fold is used for doing a case insentive comparison between a column name
	// and a struct field name. So the column "id" would match with the struct
	// field of "ID".
	fold func(s, t []byte) bool
	val  reflect.Value
}

type structFields struct {
	arr []*structField
	tab map[string]int
}

func (s *structFields) put(name string, fld *structField) {
	if s.tab == nil {
		s.tab = make(map[string]int)
	}

	if _, ok := s.tab[name]; !ok {
		s.arr = append(s.arr, fld)
		s.tab[name] = len(s.arr) - 1
	}
}

func (s *structFields) get(name string) (*structField, bool) {
	if idx, ok := s.tab[name]; ok {
		return s.arr[idx], true
	}

	for _, fld := range s.arr {
		if fld.fold([]byte(fld.name), []byte(name)) {
			s.put(name, fld)
			return fld, true
		}
	}
	return nil, false
}

// Scanner is used for scanning row data into Models.
type Scanner struct {
	rows *sql.Rows
	cols []string
	dest []any
}

// NewScanner returns a [Scanner] for scanning the given [database.sql.Rows]
// into Models.
func NewScanner(rows *sql.Rows) (*Scanner, error) {
	cols, err := rows.Columns()

	if err != nil {
		return nil, err
	}

	return &Scanner{
		rows: rows,
		cols: cols,
		dest: make([]any, 0, len(cols)),
	}, nil
}

type StructFieldError struct {
	Tag    string
	Struct string
	Field  string
	Err    error
}

func (e *StructFieldError) Error() string {
	if e.Tag != "" {
		return fmt.Sprintf("struct tag %q for field %s.%s: %s", e.Tag, e.Struct, e.Field, e.Err)
	}
	return fmt.Sprintf("struct field %s.%s: %s", e.Struct, e.Field, e.Err)
}

const scanAliasTag = "db"

func (sc *Scanner) getFields(rv reflect.Value) (*structFields, error) {
	if rv.IsNil() {
		return nil, errors.New("target cannot be nil")
	}

	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, errors.New("target must be struct or pointer to struct")
	}

	var fields structFields

	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		sf := rt.Field(i)
		sv := rv.Field(i)

		if v := sf.Tag.Get(scanAliasTag); v != "" {
			if v == "-" {
				continue
			}

			for _, col := range strings.Split(v, ",") {
				if strings.Contains(col, ":") {
					parts := strings.SplitN(col, ":", 2)

					col = parts[0]
					target := parts[1]

					if target == "" {
						return nil, &StructFieldError{
							Tag:    col,
							Struct: rt.Name(),
							Field:  sf.Name,
							Err:    errors.New("missing mapping target"),
						}
					}

					if sv.IsNil() {
						continue
					}

					nested, err := sc.getFields(sv)

					if err != nil {
						return nil, &StructFieldError{
							Struct: rt.Name(),
							Field:  sf.Name,
							Err:    err,
						}
					}

					if strings.Contains(col, ".") {
						parts := strings.SplitN(col, ".", 2)

						prefix := parts[0]

						if prefix == "" {
							return nil, &StructFieldError{
								Tag:    col,
								Struct: rt.Name(),
								Field:  sf.Name,
								Err:    errors.New("missing mapping prefix"),
							}
						}

						if parts[1] == "*" {
							for _, fld := range nested.arr {
								fld.name = prefix + "." + fld.name
								fields.put(fld.name, fld)
							}
							continue
						}
					}

					if fld, ok := nested.get(target); ok {
						fields.put(col, fld)
						continue
					}

					if col == "*" && target == "*" {
						for _, fld := range nested.arr {
							fields.put(fld.name, fld)
						}
					}
					continue
				}

				fields.put(col, &structField{
					name: col,
					fold: foldFunc([]byte(col)),
					val:  sv,
				})
			}
			continue
		}

		fields.put(sf.Name, &structField{
			name: sf.Name,
			fold: foldFunc([]byte(sf.Name)),
			val:  sv,
		})
	}
	return &fields, nil
}

type ColumnScanError struct {
	Table  string
	Column string
	Value  string
	Type   reflect.Type
	Struct string
	Field  string
}

func colScanError(m Model, col string, fld *structField, val reflect.Value) error {
	rv := reflect.ValueOf(m)

	return &ColumnScanError{
		Table:  m.Table(),
		Column: col,
		Value:  val.Kind().String(),
		Type:   fld.val.Type(),
		Struct: rv.Elem().Type().Name(),
		Field:  fld.name,
	}
}

func (e *ColumnScanError) Error() string {
	return fmt.Sprintf("cannot scan column %s.%s of type %s into Go struct field %s.%s of type %s", e.Table, e.Column, e.Value, e.Struct, e.Field, e.Type)
}

func (sc *Scanner) toString(src any) string {
	switch v := src.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}

	rv := reflect.ValueOf(src)

	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	case reflect.Float32:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
	case reflect.Bool:
		return strconv.FormatBool(rv.Bool())
	}
	return fmt.Sprintf("%v", src)
}

// Scan the current row of data into the given [Model]. It is expected for the
// given Model to be a pointer. If the Model implements [RowScanner], then this
// is used, otherwise reflection is.
//
// If "db" struct tags are specified on the Model fields then these are used to
// map the columns to the field. Struct tags can take a variety for formats.
//
// `db:"id"` Maps the column "id" to the field, irrespective of the field's
// name.
//
// `db:"user_id:id"` Maps the column "user_id" to the "id" field of the field,
// only if the field is a struct.
//
// `db:"*:*"` Maps all of the columns to the underlying struct, useful for
// working with embedded structs.
//
// `db:"users.*:*"` Maps all columns with the prefix of "users." to the
// underlying struct, useful for working with related models via joins.
//
// If no struct tags are specified then a comparison is done on the column name
// and the field name to determine if the column should be scanned into the
// field.
func (sc *Scanner) Scan(m Model) error {
	if scanner, ok := m.(RowScanner); ok {
		row := Row{
			scan:    sc.rows.Scan,
			Columns: sc.cols,
		}

		if err := scanner.Scan(&row); err != nil {
			return err
		}
		return nil
	}

	sc.dest = sc.dest[0:0]

	for range sc.cols {
		var val any
		sc.dest = append(sc.dest, &val)
	}

	rv := reflect.ValueOf(m)

	if rv.Kind() != reflect.Pointer {
		return errors.New("model must be a pointer")
	}

	fields, err := sc.getFields(rv)

	if err != nil {
		return err
	}

	if err := sc.rows.Scan(sc.dest...); err != nil {
		return err
	}

	for i, col := range sc.cols {
		fld, ok := fields.get(col)

		if !ok {
			continue
		}

		rv := reflect.ValueOf(sc.dest[i])
		el := rv.Elem()

		if src := el.Interface(); src != nil {
			val := reflect.ValueOf(src)

			fv := reflect.New(fld.val.Type())

			// If the struct field implements sql.Scanner then call scan and
			// use that value instead of reflect.ValueOf(p).
			if scanner, ok := fv.Interface().(sql.Scanner); ok {
				if err := scanner.Scan(src); err != nil {
					return err
				}
				val = fv.Elem()
			}

			switch fld.val.Kind() {
			case reflect.Pointer:
				if fld.val.IsNil() && src != nil {
					ptr := reflect.New(val.Type())
					ptr.Elem().Set(val)

					fld.val.Set(ptr)
				}
			case reflect.Bool:
				var b bool

				switch val.Kind() {
				case reflect.Bool:
					b = val.Bool()
				case reflect.Int64:
					b = val.Int() == 1
				default:
					s := sc.toString(src)

					v, err := strconv.ParseBool(s)

					if err != nil {
						return fmt.Errorf("cannot parse %T (%q) as bool: %v", src, s, err)
					}
					b = v
				}
				fld.val.SetBool(b)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				s := sc.toString(src)

				i64, err := strconv.ParseInt(s, 10, fld.val.Type().Bits())

				if err != nil {
					return fmt.Errorf("cannot parse %T (%q) as int: %v", src, s, err)
				}
				fld.val.SetInt(i64)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				s := sc.toString(src)

				u64, err := strconv.ParseUint(s, 10, fld.val.Type().Bits())

				if err != nil {
					return fmt.Errorf("cannot parse %T (%q) as uint: %v", src, s, err)
				}
				fld.val.SetUint(u64)
			case reflect.Float32, reflect.Float64:
				s := sc.toString(src)

				f64, err := strconv.ParseFloat(s, fld.val.Type().Bits())

				if err != nil {
					return fmt.Errorf("cannot parse %T (%q) as float: %v", src, s, err)
				}
				fld.val.SetFloat(f64)
			default:
				want := fld.val.Kind()
				got := val.Kind()

				if want != got {
					return colScanError(m, col, fld, val)
				}
				fld.val.Set(val)
			}
		}
	}
	return nil
}
