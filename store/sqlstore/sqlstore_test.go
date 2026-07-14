package sqlstore_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	"github.com/dangernoodle-io/shesha/jsonutil"
	"github.com/dangernoodle-io/shesha/store"
	"github.com/dangernoodle-io/shesha/store/defaults"
	"github.com/dangernoodle-io/shesha/store/sqlstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// fakeRows is a minimal database/sql/driver.Rows fault-injector: it hands
// back a preset sequence of rows, then either io.EOF (clean end) or
// errAfter (a mid-iteration error, surfaced via sql.Rows.Err()). The
// key/value row it returns can also carry a value of a type Scan can't
// convert, to trip sql.Rows.Scan's error path.
type fakeRows struct {
	cols     []string
	rows     [][]driver.Value
	idx      int
	errAfter error
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		if r.errAfter != nil {
			return r.errAfter
		}

		return io.EOF
	}

	copy(dest, r.rows[r.idx])
	r.idx++

	return nil
}

// fakeConn is a database/sql/driver.Conn that answers every Query with
// canned fakeRows chosen by mode (passed as the sql.Open DSN). It never
// touches real SQL — sqlstore's queries are opaque to it.
type fakeConn struct{ mode string }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fakeConn: Prepare not implemented")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("fakeConn: Begin not implemented")
}

func (c *fakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "scanerr":
		// A chan value can't be Scanned into *string: trips rows.Scan's
		// error return.
		return &fakeRows{
			cols: []string{"key", "value"},
			rows: [][]driver.Value{{"k", make(chan int)}},
		}, nil
	case "rowserr":
		// One good row, then a non-EOF iteration error: trips rows.Err()
		// after the loop.
		return &fakeRows{
			cols:     []string{"key", "value"},
			rows:     [][]driver.Value{{"k", "v"}},
			errAfter: errors.New("fakeRows: injected iteration error"),
		}, nil
	default:
		return nil, errors.New("fakeConn: unknown mode " + c.mode)
	}
}

var _ driver.QueryerContext = (*fakeConn)(nil)

type fakeDriver struct{}

func (fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{mode: name}, nil
}

func init() {
	sql.Register("sqlstorefake", fakeDriver{})
}

// newFakeDB opens a sqlstorefake connection whose Query behavior is
// selected by mode (see fakeConn.Query).
func newFakeDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlstorefake", mode)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// newTestDB returns an in-memory sqlite database with a config table ready
// for sqlstore, plus a cleanup-registered close. ":memory:" databases are
// per-connection in database/sql's pooling model, so MaxOpenConns is
// clamped to 1 to ensure every operation hits the same in-memory database.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE config (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)

	return db
}

func TestNew_Valid(t *testing.T) {
	db := newTestDB(t)

	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)
	require.NotNil(t, s)
}

func TestNew_NilDB(t *testing.T) {
	_, err := sqlstore.New(nil, "config")
	require.Error(t, err)
}

func TestNew_EmptyTable(t *testing.T) {
	db := newTestDB(t)

	_, err := sqlstore.New(db, "")
	require.Error(t, err)
}

func TestNew_InvalidTableName(t *testing.T) {
	db := newTestDB(t)

	for _, name := range []string{"a b", "a;drop", "1abc", "a-b"} {
		_, err := sqlstore.New(db, name)
		assert.Errorf(t, err, "table name %q should be rejected", name)
	}
}

func TestGet_Present(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, s.Set(ctx, "a", "1"))

	v, ok, err := s.Get(ctx, "a")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "1", v)
}

func TestGet_QueryError(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	require.NoError(t, db.Close())

	_, ok, err := s.Get(context.Background(), "a")
	require.Error(t, err)
	assert.False(t, ok)
}

func TestGet_Absent(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	v, ok, err := s.Get(context.Background(), "missing")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, v)
}

func TestSet_InsertThenUpdate(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, s.Set(ctx, "a", "1"))
	v, ok, err := s.Get(ctx, "a")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "1", v)

	require.NoError(t, s.Set(ctx, "a", "2"))
	v, ok, err = s.Get(ctx, "a")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "2", v)
}

func TestLoad_Empty(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	m, err := s.Load(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Empty(t, m)
}

func TestLoad_MultipleKeys(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, s.Set(ctx, "a", "1"))
	require.NoError(t, s.Set(ctx, "b", "2"))

	m, err := s.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, m)
}

func TestLoad_QueryError(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	require.NoError(t, db.Close())

	_, err = s.Load(context.Background())
	require.Error(t, err)
}

func TestLoad_ScanError(t *testing.T) {
	db := newFakeDB(t, "scanerr")
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	_, err = s.Load(context.Background())
	require.Error(t, err)
}

func TestLoad_RowsErr(t *testing.T) {
	db := newFakeDB(t, "rowserr")
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	_, err = s.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected iteration error")
}

func TestDelete_Present(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, s.Set(ctx, "a", "1"))
	require.NoError(t, s.Delete(ctx, "a"))

	_, ok, err := s.Get(ctx, "a")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDelete_Absent(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	err = s.Delete(context.Background(), "missing")
	require.NoError(t, err)
}

func TestSet_ExecError(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	require.NoError(t, db.Close())

	err = s.Set(context.Background(), "a", "1")
	require.Error(t, err)
}

func TestDelete_ExecError(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	require.NoError(t, db.Close())

	err = s.Delete(context.Background(), "a")
	require.Error(t, err)
}

func TestSave_NoOp(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	err = s.Save(context.Background())
	require.NoError(t, err)
}

func TestJSONRoundTrip(t *testing.T) {
	db := newTestDB(t)
	s, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	type widget struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	ctx := context.Background()
	want := widget{Name: "gizmo", Count: 3}

	require.NoError(t, jsonutil.SetJSON(ctx, s, "widget", want))

	got, err := jsonutil.GetJSON[widget](ctx, s, "widget")
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = jsonutil.GetJSON[widget](ctx, s, "missing")
	assert.ErrorIs(t, err, jsonutil.ErrNotFound)
}

func TestChain_ComposesAsWritableLayer(t *testing.T) {
	db := newTestDB(t)
	sqlS, err := sqlstore.New(db, "config")
	require.NoError(t, err)

	def := defaults.New(map[string]string{"a": "default-a", "b": "default-b"})

	chain := store.NewChain(store.Read(def), store.Write(sqlS))

	ctx := context.Background()

	// Reads before any override fall through to defaults.
	v, ok, err := chain.Get(ctx, "a")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "default-a", v)

	// Writes route to the sql layer and take precedence on read.
	require.NoError(t, chain.Set(ctx, "a", "overridden"))
	v, ok, err = chain.Get(ctx, "a")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "overridden", v)

	// Keys only in the sql layer are still visible.
	require.NoError(t, chain.Set(ctx, "c", "sql-only"))
	v, ok, err = chain.Get(ctx, "c")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "sql-only", v)

	// Load merges both layers, sql layer taking precedence for "a".
	m, err := chain.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "overridden", "b": "default-b", "c": "sql-only"}, m)

	// Delete routes to the sql layer.
	require.NoError(t, chain.Delete(ctx, "c"))
	_, ok, err = chain.Get(ctx, "c")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, chain.Save(ctx))
}
