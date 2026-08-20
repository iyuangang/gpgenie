package vanity

// The v4 Ed25519 fingerprint material used by gpgenie is 54 bytes, so SHA-1
// needs exactly one 64-byte compression block. Each work item changes only the
// creation timestamp (word 1), hashes the block, and evaluates the low 64 bits
// that form the OpenPGP long key ID.
const openCLVanityKernel = `
inline uint sha1_rol(uint value, uint bits) {
    return rotate(value, bits);
}

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
        uint run = 1U;
        for (uint i = 1U; i < 16U; i++) {
            uint current = i < 8U
                ? ((lo >> (i * 4U)) & 15U)
                : ((hi >> ((i - 8U) * 4U)) & 15U);
            if (current != digit) {
                break;
            }
            run++;
        }
        return run;
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
    volatile __global uint *result
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

    uint w[16];
    uint block_offset = template_index * 16U;
    for (uint i = 0U; i < 16U; i++) {
        w[i] = blocks[block_offset + i];
    }
    w[1] = timestamp;
    uint a = 0x67452301U;
    uint b = 0xEFCDAB89U;
    uint c = 0x98BADCFEU;
    uint d = 0x10325476U;
    uint e = 0xC3D2E1F0U;

#define SHA1_STEP(function_value, constant_value, index) \
    do { \
        uint schedule_index = (index) & 15U; \
        if ((index) >= 16U) { \
            w[schedule_index] = sha1_rol( \
                w[((index) - 3U) & 15U] ^ w[((index) - 8U) & 15U] ^ \
                w[((index) - 14U) & 15U] ^ w[schedule_index], 1U); \
        } \
        uint temporary = sha1_rol(a, 5U) + (function_value) + e + (constant_value) + w[schedule_index]; \
        e = d; d = c; c = sha1_rol(b, 30U); b = a; a = temporary; \
    } while (0)

    for (uint i = 0U; i < 20U; i++) {
        SHA1_STEP((b & c) | ((~b) & d), 0x5A827999U, i);
    }
    for (uint i = 20U; i < 40U; i++) {
        SHA1_STEP(b ^ c ^ d, 0x6ED9EBA1U, i);
    }
    for (uint i = 40U; i < 60U; i++) {
        SHA1_STEP((b & c) | (b & d) | (c & d), 0x8F1BBCDCU, i);
    }
    for (uint i = 60U; i < 80U; i++) {
        SHA1_STEP(b ^ c ^ d, 0xCA62C1D6U, i);
    }
#undef SHA1_STEP

    uint digest_hi = d + 0x10325476U;
    uint digest_lo = e + 0xC3D2E1F0U;
    uint run = matching_run(digest_hi, digest_lo, scope, allowed);
    // The upper five bits hold the run and the lower 27 bits hold the work
    // item index. One atomic maximum therefore publishes a consistent winner
    // without a cross-workgroup spin lock (which can deadlock older GPUs).
    uint score = (run << 27U) | logical_index;
    atomic_max(result, score);
}
`
