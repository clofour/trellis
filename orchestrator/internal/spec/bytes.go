package spec

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ByteSize is a byte count that accepts human-readable binary or decimal units
// when decoded from a YAML manifest. Its JSON representation remains a number
// so the HTTP API stays machine-oriented.
type ByteSize int64

var byteSizePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*([A-Za-z]*)$`)

// ParseByteSize parses values such as 64MiB, 512Mi, 1GB, or a bare byte count.
func ParseByteSize(value string) (ByteSize, error) {
	value = strings.TrimSpace(value)
	matches := byteSizePattern.FindStringSubmatch(value)
	if matches == nil {
		return 0, fmt.Errorf("invalid byte size %q", value)
	}
	amount, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return 0, fmt.Errorf("invalid byte size %q", value)
	}
	units := map[string]float64{
		"": 1, "b": 1,
		"kb": 1_000, "mb": 1_000_000, "gb": 1_000_000_000, "tb": 1_000_000_000_000,
		"ki": 1 << 10, "kib": 1 << 10,
		"mi": 1 << 20, "mib": 1 << 20,
		"gi": 1 << 30, "gib": 1 << 30,
		"ti": 1 << 40, "tib": 1 << 40,
	}
	multiplier, ok := units[strings.ToLower(matches[2])]
	if !ok {
		return 0, fmt.Errorf("invalid byte-size unit %q", matches[2])
	}
	bytes := amount * multiplier
	if bytes > math.MaxInt64 {
		return 0, fmt.Errorf("byte size %q is too large", value)
	}
	return ByteSize(math.Round(bytes)), nil
}

func (b ByteSize) String() string {
	value := int64(b)
	if value == 0 {
		return "0B"
	}
	units := []struct {
		size int64
		name string
	}{
		{1 << 40, "TiB"},
		{1 << 30, "GiB"},
		{1 << 20, "MiB"},
		{1 << 10, "KiB"},
	}
	for _, unit := range units {
		if value >= unit.size && value%unit.size == 0 {
			return fmt.Sprintf("%d%s", value/unit.size, unit.name)
		}
	}
	return fmt.Sprintf("%dB", value)
}
