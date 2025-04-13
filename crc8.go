// Copyright (c) 2025 chouette.21.00@gmail.com
// Released under the MIT license
// https://opensource.org/licenses/mit-license.php
package main

import (
	// "fmt"
)

const CRC8_POLYNOMIAL = 0x31
const CRC8_INIT = 0xff

// func crc8(data []byte, count uint16) uint8 {
func crc8(data []byte) ( crc uint8) {
	var current_byte uint16
	// var crc uint8 = CRC8_INIT
	var crc_bit uint8

	crc = CRC8_INIT
	var count uint16 = uint16(len(data))

	/* calculates 8-Bit checksum with given polynomial */
	for current_byte = 0; current_byte < count; current_byte++ {
		crc ^= (data[current_byte])
		for crc_bit = 8; crc_bit > 0; crc_bit-- {
			if crc & 0x80 != 0 {
				crc = (crc << 1) ^ CRC8_POLYNOMIAL
			} else {
				crc = (crc << 1)
			}
		}
	}
	return crc
}

