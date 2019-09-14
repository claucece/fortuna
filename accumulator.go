// accumulator.go - an entropy accumulator for Fortuna
// Copyright (C) 2013  Jochen Voss <voss@seehuhn.de>
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
	"crypto/aes"
	"hash"
	"os"
	"sync"
	"time"

	"github.com/seehuhn/sha256d"
)

const (
	numPools               = 32
	minPoolSize            = 32
	minReseedInterval      = 100 * time.Millisecond
	seedFileUpdateInterval = 10 * time.Minute
)

// Accumulator holds the state of one instance of the Fortuna random
// number generator.  Randomness can be extracted using the
// RandomData() and Read() methods.  Entropy from the environment
// should be submitted regularly using channels allocated by the
// NewEntropyDataSink() or NewEntropyTimeStampSink() methods.
//
// It is safe to access an Accumulator object concurrently from
// different goroutines.
type Accumulator struct {
	seedFile     *os.File
	stopAutoSave chan<- bool

	genMutex sync.Mutex
	gen      *Generator

	poolMutex    sync.Mutex
	reseedCount  int
	nextReseed   time.Time
	pool         [numPools]hash.Hash
	poolZeroSize int

	sourceMutex sync.Mutex
	nextSource  uint8
	stopSources chan bool
	sources     sync.WaitGroup
}

// NewRNG allocates a new instance of the Fortuna random number
// generator.
//
// The argument seedFileName gives the name of a file where a small
// amount of randomness can be stored between runs of the program; the
// program must be able to both read and write this file.  The
// contents of the seed file must be kept secret and seed files must
// not be shared between concurrently running instances of the random
// number generator.
//
// In case the seed file does not exist, a new seed file is created.
// If a corrupted seed file is found, ErrCorruptedSeed is returned.
// If a seed file with insecure file permissions is found,
// ErrInsecureSeed is returned.  If reading or writing the seed
// otherwise fails, the corresponding error is returned.
//
// The returned random generator must be closed using the .Close()
// method after use.
func NewRNG(seedFileName string) (*Accumulator, error) {
	return NewAccumulator(aes.NewCipher, seedFileName)
}

var (
	// NewAccumulatorAES is an alias for NewRNG, provided for backward
	// compatibility.  It should not be used in new code.
	NewAccumulatorAES = NewRNG
)

// NewAccumulator allocates a new instance of the Fortuna random
// number generator.  The argument 'newCipher' allows to choose a
// block cipher like Serpent or Twofish instead of the default AES.
// NewAccumulator(aes.NewCipher, seedFileName) is the same as
// NewRNG(seedFileName).  See the documentation for NewRNG() for more
// information.
func NewAccumulator(newCipher NewCipher, seedFileName string) (*Accumulator, error) {
	acc := &Accumulator{
		gen: NewGenerator(newCipher),
	}
	for i := 0; i < len(acc.pool); i++ {
		acc.pool[i] = sha256d.New()
	}
	acc.stopSources = make(chan bool)

	if seedFileName != "" {
		seedFile, err := os.OpenFile(seedFileName,
			os.O_RDWR|os.O_CREATE|os.O_SYNC, os.FileMode(0600))
		if err != nil {
			return nil, err
		}
		acc.seedFile = seedFile

		err = flock(acc.seedFile)
		if err != nil {
			acc.seedFile.Close()
			return nil, err
		}

		// The initial seed of the generator depends on the current
		// time.  This (partially) protects us against old seed files
		// being restored from backups, etc.
		err = acc.updateSeedFile()
		if err != nil {
			acc.seedFile.Close()
			return nil, err
		}

		quit := make(chan bool)
		acc.stopAutoSave = quit
		go func() {
			ticker := time.NewTicker(seedFileUpdateInterval)
			defer ticker.Stop()
			for {
				select {
				case <-quit:
					return
				case <-ticker.C:
					acc.writeSeedFile()
				}
			}
		}()
	}

	return acc, nil
}

// tearDownPools is called during shutdown of the Accumulator.  The
// function frees all entropy pools and transfers the remaining
// entropy into the underlying generator so that it can go into the
// seed file.
func (acc *Accumulator) tearDownPools() {
	data := make([]byte, 0, numPools*sha256d.Size)

	acc.poolMutex.Lock()
	for i := 0; i < numPools; i++ {
		data = acc.pool[i].Sum(data)
		acc.pool[i] = nil
	}
	acc.poolZeroSize = 0 // prevent accidential last-minute reseeding
	acc.poolMutex.Unlock()

	acc.genMutex.Lock()
	acc.gen.Reseed(data)
	acc.genMutex.Unlock()
}

func (acc *Accumulator) tryReseeding() []byte {
	now := time.Now()

	acc.poolMutex.Lock()
	defer acc.poolMutex.Unlock()

	if acc.poolZeroSize >= minPoolSize && now.After(acc.nextReseed) {
		acc.nextReseed = now.Add(minReseedInterval)
		acc.poolZeroSize = 0
		acc.reseedCount++

		seed := make([]byte, 0, numPools*sha256d.Size)
		for i := uint(0); i < numPools; i++ {
			x := 1 << i
			if acc.reseedCount%x != 0 {
				break
			}
			seed = acc.pool[i].Sum(seed)
			acc.pool[i].Reset()
		}
		return seed
	}
	return nil
}

// RandomData returns a slice of n random bytes.  The result can be
// used as a replacement for a sequence of uniformly distributed and
// independent bytes, and will be difficult to guess for an attacker.
func (acc *Accumulator) RandomData(n uint) []byte {
	seed := acc.tryReseeding()
	acc.genMutex.Lock()
	defer acc.genMutex.Unlock()
	if seed != nil {
		acc.gen.Reseed(seed)
	}
	return acc.gen.PseudoRandomData(n)
}

func (acc *Accumulator) randomDataUnlocked(n uint) []byte {
	seed := acc.tryReseeding()
	if seed != nil {
		acc.gen.Reseed(seed)
	}
	return acc.gen.PseudoRandomData(n)
}

// Read allows to extract randomness from the Accumulator using the
// io.Reader interface.  Read fills the byte slice p with random
// bytes.  The method always reads len(p) bytes and never returns an
// error.
func (acc *Accumulator) Read(p []byte) (n int, err error) {
	copy(p, acc.RandomData(uint(len(p))))
	return len(p), nil
}

// Close must be called before the program exits to ensure that the
// seed file is correctly updated.  After Close has been called the
// Accumulator must not be used any more.
func (acc *Accumulator) Close() error {
	close(acc.stopSources)
	acc.sources.Wait()

	acc.tearDownPools()

	var err error
	if acc.seedFile != nil {
		acc.stopAutoSave <- true
		err = acc.writeSeedFile()
		acc.seedFile.Close()
		acc.seedFile = nil
	}

	// Reset the underlying PRNG to ensure that (1) the Accumulator
	// cannot be used any more after Close() has been called and (2)
	// information about the key is not retained in memory
	// indefinitely.
	acc.gen.reset()

	return err
}

// Int63 returns a positive random integer, uniformly distributed on
// the range 0, 1, ..., 2^63-1.  This function is part of the
// rand.Source interface.
func (acc *Accumulator) Int63() int64 {
	bytes := acc.RandomData(8)
	bytes[0] &= 0x7f
	return bytesToInt64(bytes)
}

// Uint64 returns a positive random integer, uniformly distributed on
// the range 0, 1, ..., 2^64-1.  This function is part of the
// rand.Source64 interface.
func (acc *Accumulator) Uint64() uint64 {
	bytes := acc.RandomData(8)
	return bytesToUint64(bytes)
}

// Seed is part of the rand.Source interface.  This method is only
// present so that Accumulator objects can be used with the functions
// from the math/rand package.  Do not call this method!
func (acc *Accumulator) Seed(seed int64) {
	panic("Seeding a cryptographic RNG is not safe.  Don't do this!")
}
