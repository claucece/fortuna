Fortuna
=======

An implementation of Ferguson and Schneier's Fortuna_ random number
generator in Go.

Copyright (C) 2013  Jochen Voss

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

The homepage of this package is at <http://www.seehuhn.de/pages/fortuna>.
Please send any comments or bug reports to the program's author,
Jochen Voss <voss@seehuhn.de>.

.. _Fortuna: http://en.wikipedia.org/wiki/Fortuna_(PRNG)

Overview
--------

Fortuna is a `cryptographically strong`_ random number generator (RNG).
The term "cryptographically strong" indicates that even a very clever
and active attacker, who knows some of the random outputs of the RNG,
cannot use this knowledge to predict future or past outputs.  This
property allows, for example, to use the output of the RNG to generate
keys for encryption schemes, and to generate session tokens for web
pages.

.. _cryptographically strong: http://en.wikipedia.org/wiki/Cryptographically_secure_pseudorandom_number_generator

Random number generators are hard to implement and easy to get wrong;
even seemingly small details can make a huge difference to the
security of the method.  For this reason, this implementation tries to
follow the original description of the Fortuna generator (chapter 10
of [FS03]_) as closely as possible.  In addition, some effort was made
to ensure that, given identical seeds, the output of this
implementation coincides with the output of the implementation from
the `Python Cryptography Toolkit`_.

.. [FS03] Niels Ferguson, Bruce Schneier: *Practical Cryptography*, Wiley, 2003.
.. _Python Cryptography Toolkit: https://www.dlitz.net/software/pycrypto/


Installation
------------

This package can be installed using the ``go get`` command::

    go get github.com/seehuhn/fortuna


Usage
-----

The Fortuna random number generator consists of two parts: The
accumulator collects caller-provided randomness (e.g. timings between
the user's key presses).  This randomness is then used to seed a
pseudo random number generator.  During operation, the randomness from
the accumulator is also used to periodically reseed the generator,
thus allowing to recover from limited compromises of the generator's
state.

The accumulator and the generator are described in separate sections,
below.  Detailed usage instructions are available via the package's
online help, either on godoc.org_ or on the command line::

    godoc github.com/seehuhn/fortuna

.. _godoc.org: http://godoc.org/github.com/seehuhn/fortuna


Accumulator
...........

The usual way to use the Fortuna random number generator is by
creating an object of type ``Accumulator``.  A new ``Accumulator`` can
be allocated using the ``NewRNG()`` function::

    rng, err := fortuna.NewRNG(seedFileName)
    if err != nil {
	panic("cannot initialise the RNG: " + err.Error())
    }
    defer rng.Close()

The argument ``seedFileName`` is the name of a file where a small
amount of randomness can be stored between runs of the program.  The
program must be able to both read and write this file, and the
contents must be kept confidential.  If the ``seedFileName`` argument
equals the empty string ``""``, no entropy is stored between runs.  In
this case, the initial seed is only based on the current time of day,
the current user name, the list of currently installed network
interfaces, and output of the system random number generator.  Not
using a seed file can lead to more predictable output in the initial
period after the generator has been created; a seed file must be used
in security sensitive applications.

If a seed file is used, the Accumulator must be closed using the
``Close()`` method after use.

Randomness can be extracted from the Accumulator using the
``RandomData(n uint)`` and ``Read()`` methods.  For example, a slice of 16
random bytes can be obtained using the following command::

    data := rng.RandomData(16)


Entropy Pools
.............

The Accumulator uses 32 entropy pools to collect randomness from the
environment.  The use of external entropy helps to recover from
situations where an attacker obtained (partial) knowledge of the
generator state.

Any program using the Fortuna generator should continuously collect
random/unpredictable data and should submit this data to the
Accumulator.  For example, code like the following could be used to
submit the times between requests in a web-server::

    sink := rng.NewEntropyTimeStampSink()
    defer close(sink)
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	sink <- time.Now()

	...
    })


Generator
.........

The ``Generator`` class provides a pseudo random number generator
which forms the basis of the accumulator described above.  New
instances of the Fortuna pseudo random number generator can be created
using the ``NewGenerator()`` function.  The argument ``newCipher``
should normally be ``aes.NewCipher`` from the ``crypto/aes`` package,
but the Serpent_ or Twofish_ ciphers can also be used::

    gen := fortuna.NewGenerator(aes.NewCipher)

.. _Serpent: http://en.wikipedia.org/wiki/Serpent_(cipher)
.. _Twofish: http://en.wikipedia.org/wiki/Twofish

The generator can be seeded using the ``.Seed()`` or ``.Reseed()``
methods::

    gen.Seed(1234)

The method ``.Seed()`` should be used if reproducible output is
required, whereas ``.Reseed()`` can be used to add entropy in order to
achieve less predictable output.

Uniformly distributed random bytes can then be extracted using the
``.PseudoRandomData()`` method::

    data := gen.PseudoRandomData(16)

``Generator`` implements the ``rand.Source`` interface and thus the
functions from the ``math/rand`` package can be used to obtain pseudo
random samples from more complicated distributions.
