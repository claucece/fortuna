// accumulator_test.go - unit tests for common.go
// Copyright (C) 2014  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package fortuna

import (
	"bytes"
	"crypto/aes"
	"math"
	"testing"
)

func TestUint64ToBytes(t *testing.T) {
	testInts := []uint64{0, 1, 2, math.MaxUint64}
	for i := 0; i < 200; i++ {
		testInts = append(testInts, 10000000000*uint64(i)+100)
	}
	for _, x := range testInts {
		buf := uint64ToBytes(x)

		if (isZero(buf) && x != 0) || (!isZero(buf) && x == 0) {
			t.Error("isZero failed for x =", x)
		}

		y := bytesToUint64(buf)
		if x != y {
			t.Errorf("uint64<->bytes failed: %d != %d", x, y)
		}
	}
}

func TestBytesToUint64(t *testing.T) {
	buf := make([]byte, 8)
	x := bytesToUint64(buf)
	if x != 0 {
		t.Error("bytesToUint64 failed for x=0")
	}

	gen := NewGenerator(aes.NewCipher)
	gen.Seed(54321)
	for i := 0; i < 1000; i++ {
		buf = gen.PseudoRandomData(8)
		x := bytesToUint64(buf)
		buf2 := uint64ToBytes(x)
		if !bytes.Equal(buf, buf2) {
			t.Errorf("bytes<->uint64 failed:\n%v != %v", buf, buf2)
		}
	}
}

func TestInt64ToBytes(t *testing.T) {
	testInts := []int64{math.MinInt64, math.MaxInt64, -1, 0, 1}
	for i := -100; i <= 100; i++ {
		testInts = append(testInts, 10000000000*int64(i)+100)
	}
	for _, x := range testInts {
		buf := int64ToBytes(x)

		if (isZero(buf) && x != 0) || (!isZero(buf) && x == 0) {
			t.Error("isZero failed for x =", x)
		}

		y := bytesToInt64(buf)
		if x != y {
			t.Errorf("int64<->bytes failed: %d != %d", x, y)
		}
	}
}

func TestBytesToInt64(t *testing.T) {
	buf := make([]byte, 8)
	x := bytesToInt64(buf)
	if x != 0 {
		t.Error("bytesToInt64 failed for x=0")
	}

	gen := NewGenerator(aes.NewCipher)
	gen.Seed(12345)
	for i := 0; i < 1000; i++ {
		buf = gen.PseudoRandomData(8)
		x := bytesToInt64(buf)
		buf2 := int64ToBytes(x)
		if !bytes.Equal(buf, buf2) {
			t.Errorf("bytes<->int64 failed:\n%v != %v", buf, buf2)
		}
	}
}

func TestIsZero(t *testing.T) {
	buf := make([]byte, 100)
	if !isZero(buf) {
		t.Error("isZero failed (1)")
	}
	buf[99] = 1
	if isZero(buf) {
		t.Error("isZero failed (2)")
	}
}

func TestWipe(t *testing.T) {
	buf := []byte{1, 2, 3, 4, 5, 6, 7}
	wipe(buf)
	if !isZero(buf) {
		t.Error("wipe failed")
	}
}

func BenchmarkBytesToUint64(b *testing.B) {
	buf := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bytesToUint64(buf)
	}
}

func BenchmarkBytesToInt64(b *testing.B) {
	buf := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bytesToInt64(buf)
	}
}
