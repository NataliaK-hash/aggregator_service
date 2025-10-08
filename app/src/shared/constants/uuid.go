package constants

import (
	"crypto/rand"
	"fmt"
	"strings"

	sharederrors "aggregator-service/app/src/shared/errors"
)

var hyphenPositions = map[int]struct{}{8: {}, 13: {}, 18: {}, 23: {}}

// GenerateUUID returns a randomly generated UUIDv4 string.
func GenerateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3],
		b[4], b[5],
		b[6], b[7],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15],
	)
}

// ParseUUID validates the supplied UUID string and returns its lowercase representation.
func ParseUUID(value string) (string, error) {
	if len(value) != 36 {
		return "", fmt.Errorf("%w: length %d", sharederrors.ErrInvalidUUID, len(value))
	}

	for i, r := range value {
		if _, ok := hyphenPositions[i]; ok {
			if r != '-' {
				return "", fmt.Errorf("%w: expected hyphen at position %d", sharederrors.ErrInvalidUUID, i)
			}
			continue
		}

		if !isHex(r) {
			return "", fmt.Errorf("%w: invalid character %q", sharederrors.ErrInvalidUUID, r)
		}
	}

	return strings.ToLower(value), nil
}

func isHex(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'a' && r <= 'f':
		return true
	case r >= 'A' && r <= 'F':
		return true
	default:
		return false
	}
}
