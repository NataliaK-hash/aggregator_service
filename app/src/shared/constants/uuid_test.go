package constants

import (
	"strings"
	"testing"

	sharederrors "aggregator-service/app/src/shared/errors"

	"github.com/stretchr/testify/assert"
)

func TestGenerateUUIDFormat(t *testing.T) {
	t.Log("генерируем UUID и проверяем формат")
	id := GenerateUUID()

	assert.Len(t, id, 36)
	for pos, char := range id {
		switch pos {
		case 8, 13, 18, 23:
			assert.Equal(t, '-', char)
		default:
			assert.True(t, isHex(char))
		}
	}
	assert.Equal(t, '4', rune(id[14]))
	assert.True(t, id[19] == '8' || id[19] == '9' || id[19] == 'a' || id[19] == 'b')
}

func TestParseUUIDSuccess(t *testing.T) {
	t.Log("разбираем корректный UUID")
	input := "AABBCCDD-EEFF-1122-3344-5566778899AA"
	parsed, err := ParseUUID(input)

	assert.NoError(t, err)
	assert.Equal(t, strings.ToLower(input), parsed)
}

func TestParseUUIDErrors(t *testing.T) {
	t.Log("проверяем обработку некорректных строк")
	tests := []struct {
		name  string
		input string
	}{
		{"short", "123"},
		{"bad hyphen", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"invalid char", "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Пытаемся распарсить значение: %s", tc.input)
			_, err := ParseUUID(tc.input)
			assert.Error(t, err)
			assert.ErrorIs(t, err, sharederrors.ErrInvalidUUID)
		})
	}
}

func TestIsHex(t *testing.T) {
	t.Log("проверяем символы, допустимые в hex")
	hexChars := []rune{'0', '9', 'a', 'f', 'A', 'F'}
	for _, r := range hexChars {
		assert.True(t, isHex(r), "expected %q to be hex", r)
	}

	t.Log("убеждаемся, что недопустимые символы отбрасываются")
	assert.False(t, isHex('g'))
	assert.False(t, isHex('-'))
}
