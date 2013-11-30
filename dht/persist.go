package dht

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	_ "github.com/go-sql-driver/mysql"
	"io"
)

const (
	UPTNODE = "REPLACE INTO nodes(nodeid, routing, utime) VALUES(?, ?, CURRENT_TIMESTAMP)"
	ALLNODE = "SELECT nodeid FROM nodes"
	SELNODE = "SELECT routing FROM nodes WHERE nodeid = ?"
)

type Persist struct {
	db *sql.DB
}

var GPersist *Persist

func GetPersist() *Persist {
	if GPersist == nil {
		var err error
		GPersist = new(Persist)

		GPersist.db, err = sql.Open("mysql", DSN)
		if err != nil {
			panic(err)
			return nil
		}
	}
	return GPersist
}

func (persist *Persist) UpdateNodeInfo(id Identifier, routing []byte) error {
	stmt, err := persist.db.Prepare(UPTNODE)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(id.HexString(), base64.StdEncoding.EncodeToString(routing))
	if err != nil {
		return err
	}
	return nil
}

func (persist *Persist) LoadAllNodeIDs() ([]string, error) {
	rows, err := persist.db.Query(ALLNODE)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var id string
	var ret []string
	for rows.Next() {
		rows.Scan(&id)
		ret = append(ret, id)
	}
	return ret, nil
}

func (persist *Persist) LoadNodeInfo(id string, logger io.Writer) (*Node, error) {
	stmt, err := persist.db.Prepare(SELNODE)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, _ := stmt.Query(id)
	var routing string
	for rows.Next() {
		rows.Scan(&routing)
	}
	defer rows.Close()

	data, err := base64.StdEncoding.DecodeString(routing)
	if err != nil {
		return nil, err
	}
	node, err := LoadNode(bytes.NewBuffer(data), logger)
	if err != nil {
		return nil, err
	}
	return node, nil
}
