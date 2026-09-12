package models

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/pop/v6/logging"
)

// DB is a connection to your database to be used
// throughout your application.
var DB *pop.Connection

var popStdLogger = log.New(os.Stderr, "[POP] ", log.LstdFlags)

func init() {
	var err error
	env := envy.Get("GO_ENV", "development")
	DB, err = pop.Connect(env)
	if err != nil {
		log.Fatal(err)
	}
	pop.Debug = env == "development"
	configureConnectionPool(DB)
	installSafePopTxLogger()
}

// sqlPool is the subset of *sql.DB pool controls pop exposes through its
// store (pop's Store is a *sqlx.DB, which embeds *sql.DB and therefore
// promotes these methods). A structural interface keeps this dependency-free.
type sqlPool interface {
	SetMaxOpenConns(int) error
	SetMaxIdleConns(int) error
	SetConnMaxLifetime(time.Duration)
	SetConnMaxIdleTime(time.Duration)
}

// envInt reads an integer env var via envy, falling back to def on
// absence or parse errors.
func envInt(key string, def int) int {
	v := envy.Get(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// configureConnectionPool applies explicit pool sizing (env-overridable).
// Without it, the production DATABASE_URL path gets database/sql defaults
// (MaxIdleConns=2, unlimited open) because pop cannot carry pool settings in
// a URL — dev database.yml already sets pool/idlepool, this guarantees the
// same bounds everywhere. Defaults mirror database.yml: 25 / 5 / 10m / 1m.
func configureConnectionPool(conn *pop.Connection) {
	if conn == nil {
		return
	}
	p, ok := conn.Store.(sqlPool)
	if !ok {
		return
	}
	maxOpen := envInt("DB_MAX_OPEN_CONNS", 25)
	maxIdle := envInt("DB_MAX_IDLE_CONNS", 5)
	lifetime := time.Duration(envInt("DB_CONN_MAX_LIFETIME_SECONDS", 600)) * time.Second
	idleTime := time.Duration(envInt("DB_CONN_MAX_IDLE_TIME_SECONDS", 60)) * time.Second

	if maxOpen > 0 {
		_ = p.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		_ = p.SetMaxIdleConns(maxIdle)
	}
	if lifetime > 0 {
		p.SetConnMaxLifetime(lifetime)
	}
	if idleTime > 0 {
		p.SetConnMaxIdleTime(idleTime)
	}
}

// installSafePopTxLogger replaces pop v6.1.0's default tx logger, which —
// when the log target is a raw store (genericCreate/genericUpdate log calls)
// — opens a REAL sql transaction just to read its ID and never closes it
// (logger.go `case store: typed.Transaction()`). In development
// (pop.Debug=true) every Create/Update outside a request transaction leaked
// one pooled connection: resync loops wedged the app (all goroutines parked
// in database/sql.(*DB).conn) or, with an unlimited pool, exhausted MySQL
// (Error 1040). Fixed upstream in pop v6.1.2; this override backports the
// fix without a dependency upgrade. Replicates the default log format.
func installSafePopTxLogger() {
	pop.SetTxLogger(func(lvl logging.Level, anon interface{}, s string, args ...interface{}) {
		if !pop.Debug && lvl <= logging.Debug {
			return
		}
		if lvl == logging.SQL && len(args) > 0 {
			xargs := make([]string, len(args))
			for i, a := range args {
				if str, ok := a.(string); ok {
					xargs[i] = fmt.Sprintf("%q", str)
				} else {
					xargs[i] = fmt.Sprintf("%v", a)
				}
			}
			s = fmt.Sprintf("%s - %s | %s", lvl, s, xargs)
		} else {
			s = fmt.Sprintf(s, args...)
			s = fmt.Sprintf("%s - %s", lvl, s)
		}

		connID := ""
		txID := 0
		switch typed := anon.(type) {
		case *pop.Connection:
			connID = typed.ID
			if typed.TX != nil {
				txID = typed.TX.ID
			}
		case *pop.Tx:
			txID = typed.ID
			// NOTE: no `store` case on purpose — that is the leaking one in
			// pop v6.1.0. Logging the tx id is not worth a leaked connection.
		}

		if connID != "" || txID != 0 {
			s = fmt.Sprintf("%s (conn=%v, tx=%v)", s, connID, txID)
		}
		if pop.Color {
			s = color.YellowString(s)
		}
		popStdLogger.Println(s)
	})
}
