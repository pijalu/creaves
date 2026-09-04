//go:build sqlite
// +build sqlite

package actions

import (
	"database/sql"
	"fmt"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// driverNameCreavesSQLite registers a SQLite driver whose connections provide
// the MySQL scalar functions the production report queries rely on
// (CONCAT_WS). SQLite has no built-in equivalent, and pop cannot quote
// per-dialect functions, so the annual-report SQL would otherwise be
// untestable on the SQLite test harness.
const driverNameCreavesSQLite = "sqlite3_creaves"

func init() {
	sql.Register(driverNameCreavesSQLite, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			// CONCAT_WS(sep, v1, v2, ...) — MySQL semantics: skip NULL
			// arguments, return "" when nothing remains. NULL arrives as
			// nil or as an empty []byte via the driver's TEXT conversion.
			concatWS := func(sep string, vals ...interface{}) (string, error) {
				parts := make([]string, 0, len(vals))
				for _, v := range vals {
					switch b := v.(type) {
					case nil:
						continue
					case []byte:
						if len(b) == 0 {
							continue
						}
						parts = append(parts, string(b))
					case string:
						parts = append(parts, b)
					default:
						parts = append(parts, fmt.Sprintf("%v", v))
					}
				}
				return strings.Join(parts, sep), nil
			}
			return conn.RegisterFunc("CONCAT_WS", concatWS, true)
		},
	})
}
