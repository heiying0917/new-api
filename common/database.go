package common

const (
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeSQLite     = "sqlite"
	DatabaseTypePostgreSQL = "postgres"
)

var UsingSQLite = false
var UsingPostgreSQL = false
var LogSqlType = DatabaseTypeSQLite // Default to SQLite for logging SQL queries
var UsingMySQL = false
var UsingClickHouse = false

// SQLitePath is the DSN for the default SQLite database. It uses WAL journal
// mode so readers are never blocked by the single writer, plus a 30s busy
// timeout for writers to queue. (Upstream #7030 / commit 1751f43ee.)
//
//  1. The busy timeout must be passed as `_pragma=busy_timeout(30000)`; the
//     pure-Go driver (modernc.org/sqlite via glebarez/sqlite) silently ignores
//     the plain `_busy_timeout=` form, leaving SQLite's 5s default.
//  2. `_txlock=immediate` (BEGIN IMMEDIATE) takes the write lock up front so a
//     SELECT-then-write transaction cannot die on SQLITE_BUSY_SNAPSHOT.
var SQLitePath = "tokenki.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate"
