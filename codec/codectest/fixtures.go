// Package codectest holds shared test fixtures for the codec package
// and its cross-package consumers. The fixtures are real captured
// inverter replies; tests build off them rather than fabricating bytes.
package codectest

import (
	"encoding/hex"
	"strings"
)

// QS1AFixtureHex is a real captured QS1A reply for SA=0x5011
// (999900000003). 100 bytes.
const QS1AFixtureHex = "" +
	"fcfc5011bfa5999900000003" +
	"fbfb51b1" +
	"04010f3b6800b5707224417421c173248174049b9d0f0000000000070988676406fcc7346d0718c07f97051c369e5c1f000305db00000000000000000000000000000000000000000000000000000000fa2d0000" +
	"fefe"

// DS3FixtureHex is a real captured DS3 reply for SA=0x61F0
// (999900000001). 111 bytes.
const DS3FixtureHex = "" +
	"fcfc61f0cea5999900000001" +
	"fbfb5cbb" +
	"bb2000030043ffff000000000000000007670769009f0059035a138f97f70069002bffff04b908b6041ea2f402e58b5600ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff39fc" +
	"fefe"

// QS1AFixture decodes QS1AFixtureHex once.
var QS1AFixture = mustHex(QS1AFixtureHex)

// DS3Fixture decodes DS3FixtureHex once.
var DS3Fixture = mustHex(DS3FixtureHex)

func mustHex(s string) []byte {
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' {
			return -1
		}
		return r
	}, s)
	b, err := hex.DecodeString(clean)
	if err != nil {
		panic("codectest.mustHex: " + err.Error())
	}
	return b
}
