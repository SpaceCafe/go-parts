package middleware

import (
	"testing"

	"github.com/spacecafe/go-parts/pkg/validate"
	"github.com/stretchr/testify/assert"
)

func TestDummyPassword(t *testing.T) {
	t.Parallel()

	const bcryptHash = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"

	assert.Equal(t, bcryptHash, dummyPassword(map[string]validate.Secret{
		"alice": "plain-pass",
		"bob":   bcryptHash,
	}))
	assert.Empty(t, dummyPassword(map[string]validate.Secret{"alice": "plain-pass"}))
}
