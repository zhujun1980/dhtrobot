package dht

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	_ "github.com/go-sql-driver/mysql"
)

const (
	UPTNODE = "REPLACE INTO Nodes(nodeid, routing, utime) VALUES(?, ?, CURRENT_TIMESTAMP)"
	ALLNODE = "SELECT nodeid FROM Nodes"
	SELNODE = "SELECT routing FROM Nodes WHERE nodeid = ?"

	ADDRESU = "REPLACE INTO Resources(infohash) VALUES(?)"
	ADDPEER = "INSERT INTO Peers(infohash, peers) VALUES(?, ?)"
	SELPEER = "SELECT peers FROM Peers WHERE infohash = ? ORDER BY ctime DESC LIMIT 10"
	DELPEER = "DELETE FROM Peers WHERE DATEDIFF(CURRENT_TIMESTAMP(), ctime) >= 1"
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
		}
	}
	return GPersist
}

func (persist *Persist) AddResource(infohash string) error {
	stmt, err := persist.db.Prepare(ADDRESU)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(infohash)
	return err
}

func (persist *Persist) DeleteOldPeers() error {
	stmt, err := persist.db.Prepare(DELPEER)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	return err
}

func (persist *Persist) AddPeer(infohash string, peer []byte) error {
	stmt, err := persist.db.Prepare(ADDPEER)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(infohash, base64.StdEncoding.EncodeToString(peer))
	return err
}

func (persist *Persist) LoadPeers(infohash string) ([]string, error) {
	stmt, err := persist.db.Prepare(SELPEER)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, _ := stmt.Query(infohash)
	defer rows.Close()
	var link string
	var ret []string
	for rows.Next() {
		rows.Scan(&link)
		data, _ := base64.StdEncoding.DecodeString(link)
		ret = append(ret, bytes.NewBuffer(data).String())
	}
	return ret, nil
}

func (persist *Persist) UpdateNodeInfo(id Identifier, routing []byte) error {
	stmt, err := persist.db.Prepare(UPTNODE)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(id.HexString(), base64.StdEncoding.EncodeToString(routing))
	return err
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

func (persist *Persist) LoadNodeInfo(id Identifier) ([]byte, error) {
	stmt, err := persist.db.Prepare(SELNODE)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, _ := stmt.Query(id.HexString())
	var routing string
	for rows.Next() {
		rows.Scan(&routing)
	}
	defer rows.Close()

	data, err := base64.StdEncoding.DecodeString(routing)
	if err != nil {
		return nil, err
	}
	return data, nil
}
