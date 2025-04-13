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
	"log"

	// "os"
	"time"
	"unsafe"
	// "github.com/go-gorp/gorp"
)

type Scd41 struct {
	Device      int       // Device and measurement condition identifier
	Ts          time.Time // Measurement time
	Co2         int       // CO2 concentration [ppm]
	Temperature float32   // [deg.C]
	Humidity    float32   // Relative Humidity [%RH]
	Status      int       // Execution Result
}

func get_sensor_variant(fd int) (data [3]byte, err error) {
	// AHT10の読み取りコマンド
	data[0] = 0x20
	data[1] = 0x2f

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data), 2)
	if v != 2 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// 待機（75msec以上）
	time.Sleep(100 * time.Millisecond)

	// 測定結果受信
	v = C.read(C.int(fd), (unsafe.Pointer)(&data), 3)
	if v != 3 {
		err = fmt.Errorf("read() returned %d", v)
		return
	}
	var sd []byte = data[0:2]
	crc := crc8(sd)
	if crc != data[2] {
		err = fmt.Errorf("CRC error")
	}
	return
}

func get_data_ready_status(fd int) (status int, err error) {

	var data [3]byte
	data[0] = 0xe4
	data[1] = 0xb8

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data), 2)
	if v != 2 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// Wait 1msec(max)
	time.Sleep(1 * time.Millisecond)

	// 測定結果受信
	v = C.read(C.int(fd), (unsafe.Pointer)(&data), 3)
	if v != 3 {
		err = fmt.Errorf("read() returned %d", v)
		return
	}
	var sd []byte = data[0:2]
	crc := crc8(sd)
	if crc != data[2] {
		err = fmt.Errorf("CRC error")
	}

	// log.Printf(" res3=0x%02x%02x%02x\n", res3[0], res3[1], res3[2])

	status = int(data[0])
	status = status<<8 + int(data[1])

	// Least significant 11 bits are 0 → data not ready
	// status = status & 0x03FF
	status = status & 0x07FF
	/*
		例 2025年4月4日朝に発生した異常
		i=0 status=8000         not ready
		i=1 status=8000         not ready
		i=2 status=8000         not ready
		i=3 status=8000         not ready
		i=4 status=8000         not ready
		i=5 status=8006         data ready
		i=0 status=8000         not ready
		i=1 status=8000         not ready
		i=2 status=8000         not ready
		i=3 status=8000         not ready
		i=4 status=8000         not ready
		i=5 status=8006         data ready
		i=0 status=8000         not ready
		i=1 status=8000         not ready
		i=2 status=8000         not ready
		i=3 status=0000         ?
		i=4 status=0000         ?
		i=5 status=0000         ?
		i=6 status=0000         ?
		i=7 status=0000         ?

	*/

	return
}

func read_measurement(fd int, device int) (scd41 *Scd41, err error) {
	// AHT10の読み取りコマンド
	var data [9]byte
	data[0] = 0xec
	data[1] = 0x05

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data), 2)
	if v != 2 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// Wait 1msec(max)
	time.Sleep(1 * time.Millisecond)

	// 測定結果受信
	v = C.read(C.int(fd), (unsafe.Pointer)(&data), 9)
	if v != 9 {
		err = fmt.Errorf("read() returned %d", v)
		return
	}
	var sd []byte
	for i := 0; i < 9; i = i + 3 {
		sd = data[i : i+2]
		// log.Printf(" sd[%d]=%v\n", i, sd)
		crc := crc8(sd)
		if crc != data[i+2] {
			err = fmt.Errorf("CRC error")
			return
		}
	}

	sco2 := "data="
	for i := 0; i < 9; i++ {
		// log.Printf("%02x", res9[i])
		sco2 += fmt.Sprintf("%02x", data[i])
		if i%3 == 2 {
			// log.Printff(" ")
			sco2 += " "
		}
	}
	// log.Printf("\n")
	// loh.Printf("%s\n", sco2)

	co2 := uint16(data[0])*256 + uint16(data[1])
	t := -45.0 + 175.0*(float64(data[3])*256+float64(data[4]))/65535.0
	rh := 100.0 * (float64(data[6])*256.0 + float64(data[7])) / 65535.0
	// log.Printf("%s co2=%dppm t=%6.2fdeg.C rh=%5.1fRH%%\n", sco2, co2, t, rh)

	nt := time.Now().Truncate(time.Second)
	scd41 = &Scd41{
		Device:      device,
		Ts:          nt,
		Co2:         int(co2),
		Temperature: float32(t),
		Humidity:    float32(rh),
		Status:      0,
	}
	return
}

func perform_self_test(fd int) (data [3]byte, err error) {
	// AHT10の読み取りコマンド
	data[0] = 0x36
	data[1] = 0x39

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data), 2)
	if v != 2 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// Wait 1msec(max)
	time.Sleep(10000 * time.Millisecond)

	// 測定結果受信
	v = C.read(C.int(fd), (unsafe.Pointer)(&data), 3)
	if v != 3 {
		err = fmt.Errorf("read() returned %d", v)
		return
	}
	sd := data[0:2]
	crc := crc8(sd)
	if crc != data[2] {
		err = fmt.Errorf("CRC error")
	}
	return
}

func Measure_single_shot(fd int) (err error) {
	// AHT10の読み取りコマンド
	var data [2]byte
	data[0] = 0x21
	data[1] = 0x9d

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data), 2)
	if v != 2 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// Wait 5,000msec(max)
	time.Sleep(5000 * time.Millisecond)

	return
}

func start_periodic_measurement(fd int) (err error) {
	// AHT10の読み取りコマンド
	var data [2]byte
	data[0] = 0x21
	data[1] = 0xb1

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data), 2)
	if v != 2 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// Wait 5,000msec(max)
	// time.Sleep(5000 * time.Millisecond)

	return
}

func stop_periodic_measurement(fd int) (err error) {
	// AHT10の読み取りコマンド
	var data [2]byte
	data[0] = 0x3f
	data[1] = 0x86

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data), 2)
	if v != 2 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// Wait 500msec(max)
	time.Sleep(500 * time.Millisecond)

	return
}

func Perform_forced_recalibration(fd int, target int) (FRCcorrection int, err error) {
	// AHT10の読み取りコマンド
	var data5 [5]byte
	data5[0] = 0x36
	data5[1] = 0x2f
	data5[2] = byte(target >> 8)
	data5[3] = byte(target & 0xff)
	data5[4] = crc8(data5[2:4])
	log.Printf("perform_forced_recalibration() data5(write)=%v\n", data5)

	// コマンド送信
	v := C.write(C.int(fd), (unsafe.Pointer)(&data5), 5)
	if v != 5 {
		err = fmt.Errorf("write() returned %d", v)
		return
	}

	// Wait 400msec(max)
	time.Sleep(400 * time.Millisecond)

	// 測定結果受信
	v = C.read(C.int(fd), (unsafe.Pointer)(&data5), 3)
	if v != 3 {
		err = fmt.Errorf("read() returned %d", v)
		return
	}
	log.Printf("perform_forced_recalibration() data5i(read)=%v\n", data5[0:3])

	sd := data5[0:2]
	crc := crc8(sd)
	if crc != data5[2] {
		err = fmt.Errorf("CRC error")
	}
	if sd[0] == 0xff && sd[1] == 0xff {
		err = fmt.Errorf("perform_forced_recalibration() failed FRC")
	}

	FRCcorrection = int(sd[0])<<8 + int(sd[1]) - 0x8000

	return
}

func ContinuousMeasurement(fd int, dev int, term time.Duration, isOnlyLastOne bool) (err error) {
	// 測定状態フラグを設定
	measuringLock.Lock()
	measuring = true
	measuringLock.Unlock()
	// 関数終了時に測定状態フラグをリセット
	defer func() {
		measuringLock.Lock()
		measuring = false
		measuringLock.Unlock()
	}()

	err = start_periodic_measurement(fd)
	if err != nil {
		err = fmt.Errorf("start_periodic_measurement(): err=%w", err)
		// FIXME: startが失敗したときstopが必要かは不明
		log.Printf("start_periodic_measurement(): err=%s\n", err.Error())
		err = stop_periodic_measurement(fd)
		if err != nil {
			err = fmt.Errorf("stop_periodic_measurement(): err=%w", err)
		}
		return
	}

	tb := time.Now()
	scd41 := new(Scd41)
	for {
		// // シャットダウン要求があれば、測定を停止して終了
		// if shutdownRequested {
		// 	log.Printf("シャットダウン要求を検出しました。測定を終了します。")
		// 	break
		// }

		ie := 10
		for i := 0; i < ie+1; i++ {
			// // シャットダウン要求があれば、測定を停止して終了
			// if shutdownRequested {
			// 	break
			// }

			status := 0
			status, err = get_data_ready_status(fd)
			if err != nil {
				err = fmt.Errorf("get_data_ready_status(): err=%w", err)
				return
			}
			// log.Printf(" i=%d status=0x%04x\n", i, status)
			if status != 0 {
				break
			}
			if i == ie {
				err = fmt.Errorf("SCD41 not ready")
				return
			}
			time.Sleep(1 * time.Second)
		}

		// // シャットダウン要求があれば、次のデータ読み取りをスキップして終了
		// if shutdownRequested {
		// 	break
		// }

		scd41, err = read_measurement(fd, dev)
		if err != nil {
			err = fmt.Errorf("read_measurement(): err=%w", err)
			return
		}
		if !isOnlyLastOne {
			// device,ts,co2,temperature,humidity,status
			// 98,"2024-12-22 16:10:15",2312,20.4658,52.4575,0
			err = Dbmap.Insert(scd41)
			if err != nil {
				// log.Printf("Dbmap.Insert(scd41) error: %s\n", err.Error())
				fmt.Fprintf(Logfile, "%d,\"%s\",%d,%.4f,%.4f,%d\n",
					scd41.Device, scd41.Ts, scd41.Co2, scd41.Temperature, scd41.Humidity, scd41.Status)
			}
		}
		if time.Since(tb) > term || shutdownRequested {
			break
		}
	}

	// 最終データを保存 (シャットダウン中でもデータは保存する)
	if isOnlyLastOne && scd41 != nil {
		err = Dbmap.Insert(scd41)
		if err != nil {
			log.Printf("%d,\"%s\",%d,%f.4,%f.4,%d\n",
				scd41.Device, scd41.Ts, scd41.Co2, scd41.Temperature, scd41.Humidity, scd41.Status)
			// log.Printf("Dbmap.Insert(scd41) error: %s\n", err.Error())
		}
	}

	// 測定の停止処理
	stopErr := stop_periodic_measurement(fd)
	if stopErr != nil {
		log.Printf("stop_periodic_measurement(): err=%s\n", stopErr.Error())
		if err == nil {
			err = fmt.Errorf("stop_periodic_measurement(): err=%w", stopErr)
		}
	}

	return
}
