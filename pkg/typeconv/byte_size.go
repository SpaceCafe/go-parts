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
//
//nolint:recvcheck // Reference receiver is required on unmarshaling methods.
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

//nolint:gochecknoglobals // byteSuffixes maps unit strings to their byte multiplier.
var byteSuffixes = map[string]float64{
	// IEC binary
	"eib": float64(EiB),
	"pib": float64(PiB),
	"tib": float64(TiB),
	"gib": float64(GiB),
	"mib": float64(MiB),
	"kib": float64(KiB),
	"ei":  float64(EiB),
	"pi":  float64(PiB),
	"ti":  float64(TiB),
	"gi":  float64(GiB),
	"mi":  float64(MiB),
	"ki":  float64(KiB),
	// SI decimal
	"eb": float64(EB),
	"pb": float64(PB),
	"tb": float64(TB),
	"gb": float64(GB),
	"mb": float64(MB),
	"kb": float64(KB),
	"e":  float64(EB),
	"p":  float64(PB),
	"t":  float64(TB),
	"g":  float64(GB),
	"m":  float64(MB),
	"k":  float64(KB),
	// Explicit bytes
	"b": 1,
	"":  1,
}

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
			if char == ',' || char == '_' {
				continue
			}

			suffix = strings.ToLower(strings.TrimSpace(input[i:]))

			break
		}

		numStr.WriteRune(char)
	}

	num, err := strconv.ParseFloat(numStr.String(), 64)
	if err != nil {
		return 0, fmt.Errorf("%w: cannot parse '%s' as float: %w", ErrInvalidValue, input, err)
	}

	if multiplier, ok := byteSuffixes[suffix]; ok {
		resultFloat := num * multiplier

		// Check for float64 and uint64 overflow.
		if math.IsInf(resultFloat, 0) || math.IsNaN(resultFloat) || resultFloat > math.MaxUint64 {
			return 0, fmt.Errorf("%w: value '%s' overflows", ErrInvalidValue, input)
		}

		return ByteSize(resultFloat), nil
	}

	return 0, fmt.Errorf("%w: cannot parse '%s' as byte size", ErrInvalidValue, input)
}

// Int64 returns the size as an int64. It returns math.MaxInt64 if the value overflows.
func (b ByteSize) Int64() int64 {
	if b > ByteSize(math.MaxInt64) {
		return math.MaxInt64
	}

	return int64(b)
}

// MarshalText implements encoding.TextMarshaler.
func (b ByteSize) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
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
