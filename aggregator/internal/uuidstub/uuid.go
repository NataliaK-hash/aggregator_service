package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type UUID [16]byte

func New() UUID {
	var u UUID
	if _, err := rand.Read(u[:]); err != nil {
		panic(fmt.Errorf("uuid: failed to read random bytes: %w", err))
	}

	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80

	return u
}

func (u UUID) String() string {
	var buf [36]byte

	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], u[10:16])

	return string(buf[:])
}

func NewString() string {
	return New().String()
}
