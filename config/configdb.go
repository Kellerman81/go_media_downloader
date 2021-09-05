package config

import (
	"github.com/recoilme/pudge"
)

var ConfigDB *pudge.Db

func OpenConfig(file string) (db *pudge.Db, err error) {
	cfg := &pudge.Config{
		SyncInterval: 1} // every second fsync
	return pudge.Open(file, cfg)
}
