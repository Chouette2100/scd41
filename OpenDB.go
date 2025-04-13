package main

// Copyright © 2025 chouette.21.00@gmail.com
// Released under the MIT license
// https://opensource.org/licenses/mit-license.php

import (
	"fmt"

	"github.com/goark/sshql"
	"github.com/goark/sshql/mysqldrv"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	//	"github.com/PuerkitoBio/goquery"
	//	"github.com/dustin/go-humanize"

	"github.com/go-gorp/gorp"

	//	"github.com/Chouette2100/exsrapi"
	//	ghsrapi "github.com/Chouette2100/srapi"
)

/*

	00AA00	新規作成
	01AA00	DBConfig,SSHConfigを一本化する。インターフェースを変更する。


*/

type DBConfig struct {
	UseSSH    bool   `yaml:"UseSSH"`
	DBhost    string `yaml:"DBhost"`
	DBport    string `yaml:"DBport"`
	DBname    string `yaml:"DBname"`
	DBuser    string `yaml:"DBuser"`
	DBpswd    string `yaml:"DBpswd"`
	SRacct    string `yaml:"SRacct"`
	SRpswd    string `yaml:"SRpswd"`
	SSHhost   string `yaml:"SSHhost"`
	SSHport   int    `yaml:"SSHport"`
	SSHuser   string `yaml:"SSHuser"`
	SSHpswd   string `yaml:"SSHpswd"`
	SSHprvkey string `yaml:"SSHprvkey"`
}

var Dialer sshql.Dialer
var Db *sql.DB  //  プログラム中では一貫してこの変数を使うこと
var Dberr error //  プログラム中では一貫してこの変数を使うこと
var Dbmap *gorp.DbMap

//	var Tevent = "event"
//	var Teventuser = "eventuser"
//	var Tuser = "user"
//	var Tuserhistory = "userhistory"


func OpenDb(filenameofdbconfig string) (dbconfig *DBConfig, err error) {

	//	https://leben.mobi/go/mysql-connect/practice/
	//	OS := runtime.GOOS

	//	https://ssabcire.hatenablog.com/entry/2019/02/13/000722
	//	https://konboi.hatenablog.com/entry/2016/04/12/100903

	dbconfig = new(DBConfig)
	err = LoadConfig(filenameofdbconfig, &dbconfig)
	if err != nil {
		err = fmt.Errorf("Loadconfig(): %w", err)
		return
	}

	if dbconfig.DBhost == "" {
		dbconfig.DBhost = "localhost"
	}
	if dbconfig.DBport == "" {
		dbconfig.DBport = "3306"
	}
	cnc := "@tcp"
	if dbconfig.UseSSH {
		Dialer.Hostname = dbconfig.SSHhost
		Dialer.Port = dbconfig.SSHport
		Dialer.Username = dbconfig.SSHuser
		Dialer.Password = dbconfig.SSHpswd
		Dialer.PrivateKey = dbconfig.SSHprvkey

		mysqldrv.New(&Dialer).RegisterDial("ssh+tcp")
		cnc = "@ssh+tcp"
	}
	cnc += "(" + dbconfig.DBhost + ":" + dbconfig.DBport + ")"
	Db, Dberr = sql.Open("mysql", dbconfig.DBuser+":"+dbconfig.DBpswd+cnc+"/"+dbconfig.DBname+"?parseTime=true&loc=Asia%2FTokyo")
	if Dberr != nil {
		err = fmt.Errorf("sql.Open(): %w", Dberr)
	}

	return
}
