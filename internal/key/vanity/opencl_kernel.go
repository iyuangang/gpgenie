package vanity

import (
	"fmt"
	"strings"
)

// The v4 Ed25519 fingerprint material used by gpgenie is 54 bytes, so SHA-1
// needs exactly one 64-byte compression block. Each work item changes only the
// creation timestamp (word 1), hashes the block, and evaluates the low 64 bits
// that form the OpenPGP long key ID.
var openCLVanityKernel = buildOpenCLVanityKernel()

func buildOpenCLVanityKernel() string {
	var source strings.Builder
	source.WriteString(openCLKernelHeader)
	// Literal indices let even older OpenCL compilers keep the schedule in
	// registers. Rotating variable names also avoids five moves per round.
	state := []string{"a", "b", "c", "d", "e"}
	for i := 0; i < 77; i++ {
		if i >= 16 {
			fmt.Fprintf(&source, "    w%d = rotate(w%d ^ w%d ^ w%d ^ w%d, 1U);\n",
				i%16, (i-3)%16, (i-8)%16, (i-14)%16, i%16)
		}
		a, b, c, d, e := state[0], state[1], state[2], state[3], state[4]
		function, constant := fmt.Sprintf("(%s ^ %s ^ %s)", b, c, d), "0xCA62C1D6U"
		switch {
		case i < 20:
			function, constant = fmt.Sprintf("bitselect(%s, %s, %s)", d, c, b), "0x5A827999U"
		case i < 40:
			constant = "0x6ED9EBA1U"
		case i < 60:
			function, constant = fmt.Sprintf("bitselect(%s, %s, %s ^ %s)", b, c, b, d), "0x8F1BBCDCU"
		}
		fmt.Fprintf(&source, "    %s += rotate(%s, 5U) + %s + %s + w%d;\n    %s = rotate(%s, 30U);\n",
			e, a, function, constant, i%16, b, b)
		state = []string{e, a, b, c, d}
	}
	// The long key ID only needs final D/E. Those are rotations of A/B
	// after round 77; rounds 78-80 only affect the unused digest words.
	fmt.Fprintf(&source, "    uint digest_hi = rotate(%s, 30U) + 0x10325476U;\n    uint digest_lo = rotate(%s, 30U) + 0xC3D2E1F0U;\n", state[0], state[1])
	source.WriteString(openCLKernelFooter)
	return source.String()
}

const openCLKernelHeader = `
inline uint key_digit(uint hi, uint lo, uint position) {
    if (position < 8U) {
        return (hi >> (28U - position * 4U)) & 15U;
    }
    return (lo >> (28U - (position - 8U) * 4U)) & 15U;
}

inline uint matching_run(uint hi, uint lo, uint scope, uint allowed) {
    if (scope == 0U) {
        uint digit = lo & 15U;
        if ((allowed & (1U << digit)) == 0U) {
            return 0U;
        }
        uint repeated = digit * 0x11111111U;
        uint different = lo ^ repeated;
        if (different != 0U) {
            return (31U - clz(different & (0U - different))) / 4U;
        }
        different = hi ^ repeated;
        return different == 0U ? 16U : 8U + (31U - clz(different & (0U - different))) / 4U;
    }

    uint previous = key_digit(hi, lo, 0U);
    uint current_run = 1U;
    uint best = (allowed & (1U << previous)) != 0U ? 1U : 0U;
    for (uint i = 1U; i < 16U; i++) {
        uint digit = key_digit(hi, lo, i);
        if (digit == previous) {
            current_run++;
        } else {
            previous = digit;
            current_run = 1U;
        }
        if ((allowed & (1U << digit)) != 0U && current_run > best) {
            best = current_run;
        }
    }
    return best;
}

__kernel void vanity_sha1(
    __global const uint *blocks,
    uint template_count,
    uint timestamp_start,
    uint timestamp_count,
    uint work_count,
    uint allowed,
    uint scope,
    uint min_run,
    volatile __global uint *result,
    uint initial_best
) {
    uint timestamp_index = (uint)get_global_id(0);
    uint template_index = (uint)get_global_id(1);
    if (timestamp_index >= timestamp_count || template_index >= template_count) {
        return;
    }
    uint logical_index = template_index * timestamp_count + timestamp_index;
    if (logical_index >= work_count) {
        return;
    }
    uint timestamp = timestamp_start + timestamp_index;

    uint block_offset = template_index * 16U;
    uint w0 = blocks[block_offset];
    uint w1 = timestamp;
    uint w2 = blocks[block_offset + 2U];
    uint w3 = blocks[block_offset + 3U];
    uint w4 = blocks[block_offset + 4U];
    uint w5 = blocks[block_offset + 5U];
    uint w6 = blocks[block_offset + 6U];
    uint w7 = blocks[block_offset + 7U];
    uint w8 = blocks[block_offset + 8U];
    uint w9 = blocks[block_offset + 9U];
    uint w10 = blocks[block_offset + 10U];
    uint w11 = blocks[block_offset + 11U];
    uint w12 = blocks[block_offset + 12U];
    uint w13 = blocks[block_offset + 13U];
    uint w14 = blocks[block_offset + 14U];
    uint w15 = blocks[block_offset + 15U];
    uint a = 0x67452301U;
    uint b = 0xEFCDAB89U;
    uint c = 0x98BADCFEU;
    uint d = 0x10325476U;
    uint e = 0xC3D2E1F0U;

`

const openCLKernelFooter = `
    uint run = matching_run(digest_hi, digest_lo, scope, allowed);
    // The upper five bits hold the run and the lower 27 bits hold the work
    // item index. One atomic maximum therefore publishes a consistent winner
    // without a cross-workgroup spin lock (which can deadlock older GPUs).
    uint score = (run << 27U) | logical_index;
    // Only possible improvements contend on the global winner. The baseline
    // is immutable for this dispatch, so no concurrent non-atomic read races
    // with atomic_max, and every actual improvement is still considered.
    if (run > initial_best) {
        atomic_max(&result[0], score);
    }
    if (run >= min_run) {
        uint hit_index = atomic_inc(&result[1]);
        // Keep this capacity synchronized with openCLHitCapacity in the host.
        if (hit_index < 4096U) {
            result[2U + hit_index] = score;
        }
    }
}
`
