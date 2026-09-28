package typeconv

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ByteSize represents a size in bytes that can be parsed from human-readable SI/IEC strings like
// "100K", "512Mi", "2G", "1.5TB".
type ByteSize uint64

//nolint:varnamelen // SI/IEC prefixes.
const (
	KB ByteSize = 1000
	MB ByteSize = 1000 * KB
	GB ByteSize = 1000 * MB
	TB ByteSize = 1000 * GB
	PB ByteSize = 1000 * TB
	EB ByteSize = 1000 * PB

	KiB ByteSize = 1024
	MiB ByteSize = 1024 * KiB
	GiB ByteSize = 1024 * MiB
	TiB ByteSize = 1024 * GiB
	PiB ByteSize = 1024 * TiB
	EiB ByteSize = 1024 * PiB
)

// byteUnits lists every unit, largest first, as MarshalText writes it.
//
//nolint:gochecknoglobals // Read-only unit table.
var byteUnits = []struct {
	suffix string
	size   ByteSize
}{
	{"EiB", EiB},
	{"EB", EB},
	{"PiB", PiB},
	{"PB", PB},
	{"TiB", TiB},
	{"TB", TB},
	{"GiB", GiB},
	{"GB", GB},
	{"MiB", MiB},
	{"MB", MB},
	{"KiB", KiB},
	{"KB", KB},
}

// byteSuffixes maps each lower-case suffix ParseByteSize accepts to its unit: every unit in
// byteUnits with and without the trailing "b" ("kib", "ki", "kb", "k"), and "b" or no suffix for
// plain bytes.
//
//nolint:gochecknoglobals // Read-only lookup built from byteUnits.
var byteSuffixes = func() map[string]ByteSize {
	suffixes := map[string]ByteSize{"b": 1, "": 1}

	for _, unit := range byteUnits {
		suffix := strings.ToLower(unit.suffix)
		suffixes[suffix] = unit.size
		suffixes[strings.TrimSuffix(suffix, "b")] = unit.size
	}

	return suffixes
}()

// ParseByteSize parses a size such as "512", "1.5GB" or "4_096KiB". The number may use "." as the
// decimal point and "_" to group digits; the optional suffix is an SI (k, M, G, …) or IEC (Ki, Mi,
// Gi, …) unit, case-insensitive, with or without a trailing "B". An empty input is 0.
func ParseByteSize(input string) (ByteSize, error) {
	var (
		numStr strings.Builder
		suffix string
	)

	if input == "" {
		return 0, nil
	}

	for i, char := range input {
		if !unicode.IsDigit(char) && char != '.' {
			// Only "_" is a digit separator. A comma is rejected rather than skipped: "1,5G" would
			// otherwise silently become 15 GB for anyone writing a decimal comma.
			if char == '_' {
				continue
			}

			suffix = strings.ToLower(strings.TrimSpace(input[i:]))

			break
		}

		numStr.WriteRune(char)
	}

	// An integer is parsed exactly: float64 has 53 bits of precision, so larger byte counts, such as
	// the ones MarshalText writes, would otherwise change.
	if !strings.Contains(numStr.String(), ".") {
		return parseIntegerByteSize(numStr.String(), suffix)
	}

	num, err := strconv.ParseFloat(numStr.String(), 64)
	if err != nil {
		return 0, byteSizeNumberError(err)
	}

	if multiplier, ok := byteSuffixes[suffix]; ok {
		resultFloat := num * float64(multiplier)

		// Check for float64 and uint64 overflow. float64(math.MaxUint64) rounds up to 2^64, which does
		// not fit, so the comparison must be >=.
		if math.IsInf(resultFloat, 0) || math.IsNaN(resultFloat) || resultFloat >= math.MaxUint64 {
			return 0, fmt.Errorf("%w: byte size overflows", ErrInvalidValue)
		}

		return ByteSize(resultFloat), nil
	}

	return 0, fmt.Errorf("%w: unknown byte size suffix", ErrInvalidValue)
}

// parseIntegerByteSize multiplies the integer numStr by the unit of suffix without going through
// float64.
func parseIntegerByteSize(numStr, suffix string) (ByteSize, error) {
	num, err := strconv.ParseUint(numStr, 10, 64)
	if err != nil {
		return 0, byteSizeNumberError(err)
	}

	multiplier, ok := byteSuffixes[suffix]
	if !ok {
		return 0, fmt.Errorf("%w: unknown byte size suffix", ErrInvalidValue)
	}

	if num > math.MaxUint64/uint64(multiplier) {
		return 0, fmt.Errorf("%w: byte size overflows", ErrInvalidValue)
	}

	return ByteSize(num) * multiplier, nil
}

// byteSizeNumberError wraps a failure to parse the number part of a byte size.
func byteSizeNumberError(err error) error {
	return fmt.Errorf("%w: cannot parse byte size number: %w", ErrInvalidValue, numErrCause(err))
}

// Int64 returns the size as an int64. It returns math.MaxInt64 if the value overflows.
func (b ByteSize) Int64() int64 {
	if b > ByteSize(math.MaxInt64) {
		return math.MaxInt64
	}

	return int64(b)
}

// MarshalText implements encoding.TextMarshaler. Unlike String, it is exact, so the value survives a
// round trip through UnmarshalText: it uses the largest IEC or SI unit that divides the size evenly
// ("1GiB", "1MB"), or plain bytes ("1537B").
func (b ByteSize) MarshalText() ([]byte, error) {
	if b != 0 {
		for _, unit := range byteUnits {
			if b%unit.size == 0 {
				return []byte(strconv.FormatUint(uint64(b/unit.size), 10) + unit.suffix), nil
			}
		}
	}

	return []byte(strconv.FormatUint(uint64(b), 10) + "B"), nil
}

// String returns a human-readable IEC representation (e.g. "1.5GiB").
func (b ByteSize) String() string {
	switch {
	case b >= EiB:
		return fmt.Sprintf("%.1fEiB", float64(b)/float64(EiB))
	case b >= PiB:
		return fmt.Sprintf("%.1fPiB", float64(b)/float64(PiB))
	case b >= TiB:
		return fmt.Sprintf("%.1fTiB", float64(b)/float64(TiB))
	case b >= GiB:
		return fmt.Sprintf("%.1fGiB", float64(b)/float64(GiB))
	case b >= MiB:
		return fmt.Sprintf("%.1fMiB", float64(b)/float64(MiB))
	case b >= KiB:
		return fmt.Sprintf("%.1fKiB", float64(b)/float64(KiB))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// Uint64 returns the size as a plain uint64.
func (b ByteSize) Uint64() uint64 {
	return uint64(b)
}

// UnmarshalText implements encoding.TextUnmarshaler, which is used by JSON and YAML decoders to
// parse quoted strings.
func (b *ByteSize) UnmarshalText(text []byte) error {
	parsed, err := ParseByteSize(string(text))
	if err != nil {
		return err
	}

	*b = parsed

	return nil
}
