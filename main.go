// Copyright © 2025 chouette.21.00@gmail.com
// Released under the MIT license
// https://opensource.org/licenses/mit-license.php
package main

/*
#cgo CFLAGS: -I~/OrangePi/wiringOP/wiringPi
#cgo LDFLAGS: -L/usr/local/lib -lwiringPi

#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>

#include <error.h>
#include <fcntl.h>
#include <sys/ioctl.h>
#include <asm/ioctl.h>

#include "wiringPi.h"
#include "wiringPiI2C.h"
*/
import "C"

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	// "unsafe"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/go-gorp/gorp"

	"github.com/Chouette2100/exsrapi/v2"
)

/*
000100	新規作成
000200	シグナルハンドリング機能を追加する
*/

const Version = "000200"

var Logfile *os.File

// シグナル処理のための変数
var (
	// シャットダウンリクエストを示すフラグ
	shutdownRequested bool
	// 測定中かどうかを示す変数
	measuring     bool
	measuringLock sync.Mutex
)

func main() {

	var err error

	logfilename := "log_scd41_" + Version + time.Now().Format("20060102") + ".txt"
	Logfile, err = os.OpenFile(logfilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		panic("cannnot open logfile: " + logfilename + err.Error())
	}
	defer Logfile.Close()

	// フォアグラウンド（端末に接続されているか）を判定
	isForeground := terminal.IsTerminal(int(os.Stdout.Fd()))

	var logOutput io.Writer
	if isForeground {
		// フォアグラウンドならログファイル + コンソール
		logOutput = io.MultiWriter(os.Stdout, Logfile)
	} else {
		// バックグラウンドならログファイルのみ
		logOutput = Logfile
	}

	// ロガーの設定
	log.SetOutput(logOutput)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// ログ出力テスト
	log.Println("アプリケーションを起動しました")

	// シグナルハンドリングのための設定
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// シグナル受信用のゴルーチンを開始
	go func() {
		sig := <-sigChan
		log.Printf("シグナル %v を受信しました。安全に停止します...", sig)
		// シャットダウンフラグを設定
		shutdownRequested = true

		// 測定中なら終了を待機
		for {
			measuringLock.Lock()
			if !measuring {
				measuringLock.Unlock()
				break
			}
			measuringLock.Unlock()
			log.Println("測定中のため、終了を待機しています...")
			time.Sleep(1 * time.Second)
		}

		log.Println("安全にシャットダウンします")
		os.Exit(0)
	}()

	// =========================

	// Check for the existence of a lock file to avoid double activation, and if not,
	// create a lock file. The lock file is deleted after the process is completed.
	// ロックファイルのパス
	lockFilePath := "/tmp/scd41.lock"

	// 既存のロックファイルをチェック
	if exsrapi.CheckExistingLock(lockFilePath) {
		log.Println("既に別のインスタンスが実行中です。終了します。")
		os.Exit(1)
	}

	// ロックファイルを作成・オープン
	lockFile, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("ロックファイル作成エラー: %v\n", err)
		os.Exit(1)
	}
	// 現在のプロセスIDを書き込む
	fmt.Fprintf(lockFile, "%d", os.Getpid())
	lockFile.Close()

	// 終了時のクリーンアップ
	defer func() {
		os.Remove(lockFilePath)
		log.Println("ロックファイルを削除しました")
	}()

	// =========================

	//      データベースとの接続をオープンする。
	dbconfig, err := OpenDb("DBConfig.yml")
	if err != nil {
		fmt.Printf("Database error. err=%s.\n", err.Error())
		return
	}
	if dbconfig.UseSSH {
		defer Dialer.Close()
	}
	defer Db.Close()
	log.Printf("dbconfig=%+v.\n", dbconfig)

	dial := gorp.MySQLDialect{Engine: "InnoDB", Encoding: "utf8mb4"}
	Dbmap = &gorp.DbMap{Db: Db, Dialect: dial, ExpandSliceArgs: true}

	// --------------------------------

	// Dbmap.AddTableWithName(Aht10{}, "aht10").SetKeys(false, "Device", "Ts")
	Dbmap.AddTableWithName(Scd41{}, "scd41").SetKeys(false, "Device", "Ts")

	// =========================

	log.Printf("wiringPiSetup start\n")
	C.wiringPiSetup()

	cfd := C.wiringPiI2CSetupInterface(C.CString("/dev/i2c-3"), 0x62)
	fd := int(cfd)
	if fd < 0 {
		log.Printf("wiringPiI2CSetupInterface() error %d\n", fd)
		os.Exit(1)
	}

	log.Printf("wiringPiI2CSetupInterface() 0x62:%d\n", fd)

	// =========================

	res, err := get_sensor_variant(fd)
	log.Printf(" get_sensor_variant(): 0x%02x%02x%02x\n", res[0], res[1], res[2])
	if err != nil {
		log.Printf("get_sensor_variant(): err=%s\n", err.Error())
		os.Exit(1)
	}

	res, err = perform_self_test(fd)
	if err != nil {
		log.Printf("perform_self_test(fd): error:%s\n", err.Error())
		os.Exit(1)
	}
	wres := uint(res[0])
	wres = wres<<8 | uint(res[1])
	if wres != 0 {
		log.Printf("perform_self_test() return 0x%04x\n", wres)
		os.Exit(1)
	} else {
		log.Printf("perform_self_test(): PASS\n")
	}

	dt := 5
	for {
		tnow := time.Now()
		// nt := tnow.Truncate(time.Duration(dt) * time.Minute).Add(time.Duration(dt) * time.Minute)
		nt := tnow.Truncate(time.Duration(dt) * time.Minute).Add(time.Duration(60*dt-130) * time.Second)
		if nt.Before(tnow) {
			nt = nt.Add(time.Duration(60*dt) * time.Second)
		}
		time.Sleep(nt.Sub(tnow))

		const isOnlyLastOne = true
		dev := 0x1000
		err = ContinuousMeasurement(fd, dev, 2*time.Minute, isOnlyLastOne)
		if err != nil {
			log.Printf("ContinuousMeasurement() error:%s\n", err.Error())
			continue
		}
	}
}
