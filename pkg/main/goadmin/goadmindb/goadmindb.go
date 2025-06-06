package goadmindb

import (
	"database/sql"

	"github.com/GoAdminGroup/go-admin/modules/config"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// Sqlite is a Connection of sqlite.
type mysqlite struct {
	db.Base
}

// GetSqliteDB return the global sqlite connection.
func GetSqliteDB() *mysqlite {
	return &mysqlite{
		Base: db.Base{
			DbList: make(map[string]*sql.DB),
		},
	}
}

// Name implements the method Connection.Name.
func (*mysqlite) Name() string {
	return "sqlite"
}

// GetDelimiter implements the method Connection.GetDelimiter.
func (*mysqlite) GetDelimiter() string {
	return "`"
}

// GetDelimiter2 implements the method Connection.GetDelimiter2.
func (*mysqlite) GetDelimiter2() string {
	return "`"
}

// GetDelimiters implements the method Connection.GetDelimiters.
func (*mysqlite) GetDelimiters() []string {
	return []string{"`", "`"}
}

// QueryWithConnection implements the method Connection.QueryWithConnection.
func (d *mysqlite) QueryWithConnection(
	con string,
	query string,
	args ...any,
) ([]map[string]any, error) {
	database.GetMutex().RLock()
	defer database.GetMutex().RUnlock()
	return db.CommonQuery(d.DbList[con], query, args...)
}

// ExecWithConnection implements the method Connection.ExecWithConnection.
func (d *mysqlite) ExecWithConnection(con string, query string, args ...any) (sql.Result, error) {
	database.GetMutex().Lock()
	defer database.GetMutex().Unlock()

	return db.CommonExec(d.DbList[con], query, args...)
}

// Query implements the method Connection.Query.
func (d *mysqlite) Query(query string, args ...any) ([]map[string]any, error) {
	database.GetMutex().RLock()
	defer database.GetMutex().RUnlock()
	return db.CommonQuery(d.DbList["default"], query, args...)
}

// Exec implements the method Connection.Exec.
func (d *mysqlite) Exec(query string, args ...any) (sql.Result, error) {
	database.GetMutex().Lock()
	defer database.GetMutex().Unlock()
	return db.CommonExec(d.DbList["default"], query, args...)
}

func (d *mysqlite) QueryWith(
	tx *sql.Tx,
	conn, query string,
	args ...any,
) ([]map[string]any, error) {
	if tx != nil {
		return d.QueryWithTx(tx, query, args...)
	}
	return d.QueryWithConnection(conn, query, args...)
}

func (d *mysqlite) ExecWith(tx *sql.Tx, conn, query string, args ...any) (sql.Result, error) {
	if tx != nil {
		return d.ExecWithTx(tx, query, args...)
	}
	return d.ExecWithConnection(conn, query, args...)
}

// InitDB implements the method Connection.InitDB.
func (d *mysqlite) InitDB(cfgList map[string]config.Database) db.Connection {
	d.Configs = cfgList
	d.Once.Do(func() {
		var sqlDB *sql.DB
		var err error

		for conn, cfg := range cfgList {
			if conn == "default" {
				sqlDB, err = sql.Open("sqlite3", cfg.GetDSN())
				if err != nil {
					panic(err)
				}

				// sqlDB.SetMaxIdleConns(cfg.MaxIdleCon)
				// sqlDB.SetMaxOpenConns(cfg.MaxOpenCon)

				d.DbList[conn] = sqlDB

				if err = sqlDB.Ping(); err != nil {
					panic(err)
				}
			}
			if conn == "media" {
				d.DbList[conn] = database.Getdb(false).DB
			}
		}
	})
	return d
}

// BeginTxWithReadUncommitted starts a transaction with level LevelReadUncommitted.
func (d *mysqlite) BeginTxWithReadUncommitted() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelReadUncommitted)
}

// BeginTxWithReadCommitted starts a transaction with level LevelReadCommitted.
func (d *mysqlite) BeginTxWithReadCommitted() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelReadCommitted)
}

// BeginTxWithRepeatableRead starts a transaction with level LevelRepeatableRead.
func (d *mysqlite) BeginTxWithRepeatableRead() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelRepeatableRead)
}

// BeginTx starts a transaction with level LevelDefault.
func (d *mysqlite) BeginTx() *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], sql.LevelDefault)
}

// BeginTxWithLevel starts a transaction with given transaction isolation level.
func (d *mysqlite) BeginTxWithLevel(level sql.IsolationLevel) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList["default"], level)
}

// BeginTxWithReadUncommittedAndConnection starts a transaction with level LevelReadUncommitted and connection.
func (d *mysqlite) BeginTxWithReadUncommittedAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelReadUncommitted)
}

// BeginTxWithReadCommittedAndConnection starts a transaction with level LevelReadCommitted and connection.
func (d *mysqlite) BeginTxWithReadCommittedAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelReadCommitted)
}

// BeginTxWithRepeatableReadAndConnection starts a transaction with level LevelRepeatableRead and connection.
func (d *mysqlite) BeginTxWithRepeatableReadAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelRepeatableRead)
}

// BeginTxAndConnection starts a transaction with level LevelDefault and connection.
func (d *mysqlite) BeginTxAndConnection(conn string) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], sql.LevelDefault)
}

// BeginTxWithLevelAndConnection starts a transaction with given transaction isolation level and connection.
func (d *mysqlite) BeginTxWithLevelAndConnection(conn string, level sql.IsolationLevel) *sql.Tx {
	return db.CommonBeginTxWithLevel(d.DbList[conn], level)
}

// QueryWithTx is query method within the transaction.
func (*mysqlite) QueryWithTx(tx *sql.Tx, query string, args ...any) ([]map[string]any, error) {
	database.GetMutex().RLock()
	defer database.GetMutex().RUnlock()
	return db.CommonQueryWithTx(tx, query, args...)
}

// ExecWithTx is exec method within the transaction.
func (*mysqlite) ExecWithTx(tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	database.GetMutex().Lock()
	defer database.GetMutex().Unlock()
	return db.CommonExecWithTx(tx, query, args...)
}
