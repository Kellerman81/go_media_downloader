package database

import (
	"database/sql"

	"github.com/GoAdminGroup/go-admin/modules/config"
	"github.com/GoAdminGroup/go-admin/modules/db"
)

// Sqlite is a Connection of sqlite.
type Mysqlite struct {
	db.Base
}

// GetSqliteDB return the global sqlite connection.
func GetSqliteDB() *Mysqlite {
	return &Mysqlite{
		Base: db.Base{
			DbList: make(map[string]*sql.DB),
		},
	}
}

// Name implements the method Connection.Name.
func (d *Mysqlite) Name() string {
	return "sqlite"
}

// GetDelimiter implements the method Connection.GetDelimiter.
func (d *Mysqlite) GetDelimiter() string {
	return "`"
}

// GetDelimiter2 implements the method Connection.GetDelimiter2.
func (d *Mysqlite) GetDelimiter2() string {
	return "`"
}

// GetDelimiters implements the method Connection.GetDelimiters.
func (d *Mysqlite) GetDelimiters() []string {
	return []string{"`", "`"}
}

// QueryWithConnection implements the method Connection.QueryWithConnection.
func (d *Mysqlite) QueryWithConnection(con string, query string, args ...interface{}) ([]map[string]interface{}, error) {
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	return db.CommonQuery(d.DbList[con], query, args...)
}

// ExecWithConnection implements the method Connection.ExecWithConnection.
func (d *Mysqlite) ExecWithConnection(con string, query string, args ...interface{}) (sql.Result, error) {

	return db.CommonExec(d.DbList[con], query, args...)
}

// Query implements the method Connection.Query.
func (d *Mysqlite) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	return db.CommonQuery(d.DbList["default"], query, args...)
}

// Exec implements the method Connection.Exec.
func (d *Mysqlite) Exec(query string, args ...interface{}) (sql.Result, error) {

	return db.CommonExec(d.DbList["default"], query, args...)
}

func (d *Mysqlite) QueryWith(tx *sql.Tx, conn, query string, args ...interface{}) ([]map[string]interface{}, error) {
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if tx != nil {
		return d.QueryWithTx(tx, query, args...)
	}
	return d.QueryWithConnection(conn, query, args...)
}

func (d *Mysqlite) ExecWith(tx *sql.Tx, conn, query string, args ...interface{}) (sql.Result, error) {

	if tx != nil {
		return d.ExecWithTx(tx, query, args...)
	}
	return d.ExecWithConnection(conn, query, args...)
}

// InitDB implements the method Connection.InitDB.
func (d *Mysqlite) InitDB(cfgList map[string]config.Database) db.Connection {
	d.Configs = cfgList
	d.Once.Do(func() {
		for conn, cfg := range cfgList {
			if conn == "default" {
				sqlDB, err := sql.Open("sqlite3", cfg.GetDSN())

				if err != nil {
					panic(err)
				}

				sqlDB.SetMaxIdleConns(cfg.MaxIdleCon)
				sqlDB.SetMaxOpenConns(cfg.MaxOpenCon)

				d.DbList[conn] = sqlDB

				if err := sqlDB.Ping(); err != nil {
					panic(err)
				}
			}
			if conn == "media" {
				d.DbList[conn] = DB.DB
			}
		}
	})
	return d
}

// BeginTxWithReadUncommitted starts a transaction with level LevelReadUncommitted.
func (d *Mysqlite) BeginTxWithReadUncommitted() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelReadUncommitted)
}

// BeginTxWithReadCommitted starts a transaction with level LevelReadCommitted.
func (d *Mysqlite) BeginTxWithReadCommitted() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelReadCommitted)
}

// BeginTxWithRepeatableRead starts a transaction with level LevelRepeatableRead.
func (d *Mysqlite) BeginTxWithRepeatableRead() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelRepeatableRead)
}

// BeginTx starts a transaction with level LevelDefault.
func (d *Mysqlite) BeginTx() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelDefault)
}

// BeginTxWithLevel starts a transaction with given transaction isolation level.
func (d *Mysqlite) BeginTxWithLevel(level sql.IsolationLevel) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], level)
}

// BeginTxWithReadUncommittedAndConnection starts a transaction with level LevelReadUncommitted and connection.
func (d *Mysqlite) BeginTxWithReadUncommittedAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelReadUncommitted)
}

// BeginTxWithReadCommittedAndConnection starts a transaction with level LevelReadCommitted and connection.
func (d *Mysqlite) BeginTxWithReadCommittedAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelReadCommitted)
}

// BeginTxWithRepeatableReadAndConnection starts a transaction with level LevelRepeatableRead and connection.
func (d *Mysqlite) BeginTxWithRepeatableReadAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelRepeatableRead)
}

// BeginTxAndConnection starts a transaction with level LevelDefault and connection.
func (d *Mysqlite) BeginTxAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelDefault)
}

// BeginTxWithLevelAndConnection starts a transaction with given transaction isolation level and connection.
func (d *Mysqlite) BeginTxWithLevelAndConnection(conn string, level sql.IsolationLevel) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], level)
}

// QueryWithTx is query method within the transaction.
func (d *Mysqlite) QueryWithTx(tx *sql.Tx, query string, args ...interface{}) ([]map[string]interface{}, error) {
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	return db.CommonQueryWithTx(tx, query, args...)
}

// ExecWithTx is exec method within the transaction.
func (d *Mysqlite) ExecWithTx(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {

	return db.CommonExecWithTx(tx, query, args...)
}
