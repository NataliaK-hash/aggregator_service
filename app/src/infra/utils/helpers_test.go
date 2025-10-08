package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptyFallback(t *testing.T) {
	t.Log("проверяем возврат значения по умолчанию")
	assert.Equal(t, "fallback", EmptyFallback("", "fallback"))
	t.Log("проверяем возврат исходного значения")
	assert.Equal(t, "value", EmptyFallback("value", "fallback"))
}

func TestPtr(t *testing.T) {
	t.Log("создаём указатель на значение и проверяем его")
	value := 42
	pointer := Ptr(value)

	assert.NotNil(t, pointer)
	assert.Equal(t, value, *pointer)
}
