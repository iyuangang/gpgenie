//go:build amd64 && !purego

package vanity

import "golang.org/x/sys/cpu"

var useSHAInstructions = cpu.X86.HasSSE41 && hasSHAInstructions()

func hasSHAInstructions() bool

//go:noescape
func sha1KeyIDsSHANI(block *[16]uint32, timestamp uint32, output []uint64)

func keyIDsBatch(block *[16]uint32, template []byte, timestamp uint32, output []uint64) {
	if !useSHAInstructions {
		keyIDsGeneric(template, timestamp, output)
		return
	}
	even := len(output) &^ 1
	sha1KeyIDsSHANI(block, timestamp, output[:even])
	keyIDsGeneric(template, timestamp+uint32(even), output[even:])
}
