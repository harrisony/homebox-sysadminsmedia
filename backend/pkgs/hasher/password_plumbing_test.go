package hasher

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func requirePasswordProtection(t *testing.T) {
	t.Helper()

	if !enabled {
		t.Skip("requires password protection to be enabled before package initialization")
	}
}

func TestDecodeHash(t *testing.T) {
	requirePasswordProtection(t)

	pw := "correct horse battery staple"
	encoded, err := HashPasswordCtx(t.Context(), pw)
	require.NoError(t, err)

	assert.Len(t, strings.Split(encoded, "$"), 6)

	t.Run("valid_hash_decodes_params_salt_key", func(t *testing.T) {
		out, salt, hash, err := decodeHash(encoded)

		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, p.memory, out.memory)
		assert.Equal(t, p.iterations, out.iterations)
		assert.Equal(t, p.parallelism, out.parallelism)
		assert.Len(t, salt, int(p.saltLength))
		assert.Len(t, hash, int(p.keyLength))
		assert.Equal(t, p.saltLength, out.saltLength)
		assert.Equal(t, p.keyLength, out.keyLength)
	})

	t.Run("wrong_segments_rejected", func(t *testing.T) {
		_, _, _, err := decodeHash("$argon2id$v=19$m=65536,t=3,p=2")
		require.ErrorContains(t, err, "expected 6 segments")
	})

	t.Run("bad_version_segment_rejected", func(t *testing.T) {
		_, _, _, err := decodeHash("$argon2id$NOPE$m=65536,t=3,p=2$c2FsdA$ZmFrZS1rZXk=")
		require.ErrorContains(t, err, "invalid version segment")
	})

	t.Run("unsupported_version_rejected", func(t *testing.T) {
		_, _, _, err := decodeHash("$argon2id$v=42$m=65536,t=3,p=2$c2FsdA$ZmFrZS1rZXk=")
		require.ErrorContains(t, err, "unsupported argon2 version")
		assert.ErrorContains(t, err, "want ")
	})

	t.Run("bad_params_segment_rejected", func(t *testing.T) {
		_, _, _, err := decodeHash("$argon2id$v=19$NOPE$c2FsdA$ZmFrZS1rZXk=")
		require.ErrorContains(t, err, "invalid params segment")
	})

	t.Run("bad_salt_base64_rejected", func(t *testing.T) {
		_, _, _, err := decodeHash("$argon2id$v=19$m=65536,t=3,p=2$!!!not-base64!!!$ZmFrZS1rZXk=")
		require.ErrorContains(t, err, "invalid salt")
	})

	t.Run("bad_key_base64_rejected", func(t *testing.T) {
		_, _, _, err := decodeHash("$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!not-base64!!!")
		require.ErrorContains(t, err, "invalid key")
	})
}

func TestComparePasswordAndHash(t *testing.T) {
	requirePasswordProtection(t)

	pw := "hunter2#hunter2"
	encoded, err := HashPasswordCtx(t.Context(), pw)
	require.NoError(t, err)

	t.Run("matching_password_returns_true", func(t *testing.T) {
		match, decodeErr, compareErr := comparePasswordAndHash(t.Context(), pw, encoded)
		require.NoError(t, decodeErr)
		require.NoError(t, compareErr)
		assert.True(t, match, "matching password must compare equal via constant-time path")
	})

	t.Run("wrong_password_returns_false_nil_errors", func(t *testing.T) {
		match, decodeErr, compareErr := comparePasswordAndHash(t.Context(), "wrong-password", encoded)
		require.NoError(t, decodeErr, "decode must succeed for a well-formed hash")
		require.NoError(t, compareErr, "compare must not error on mismatch - it returns false")
		assert.False(t, match)
	})

	t.Run("malformed_hash_returns_decode_error", func(t *testing.T) {
		match, decodeErr, compareErr := comparePasswordAndHash(t.Context(), pw, "not-a-real-hash")
		assert.False(t, match)
		require.Error(t, decodeErr, "malformed hash must surface decodeErr")
		assert.NoError(t, compareErr, "compareErr must be nil when decode failed")
	})
}

func TestClassifyDecodeError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		expect string
	}{
		{"nil_returns_empty", nil, ""},
		{"segment_count", errors.New("invalid hash format: expected 6 segments, got 4"), "format_segment_count"},
		{"version_parse", errors.New("invalid version segment \"NOPE\": bad"), "version_parse"},
		{"version_unsupported", errors.New("unsupported argon2 version: got 42, want 19"), "version_unsupported"},
		{"params_parse", errors.New("invalid params segment \"NOPE\": bad"), "params_parse"},
		{"salt_b64", errors.New("invalid salt: base64 error"), "salt_b64"},
		{"key_b64", errors.New("invalid key: base64 error"), "key_b64"},
		{"other_falls_through", errors.New("something completely different"), "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, classifyDecodeError(tt.err))
		})
	}
}

func TestBcryptErrorKind(t *testing.T) {
	t.Run("nil_returns_empty", func(t *testing.T) {
		assert.Empty(t, bcryptErrorKind(nil))
	})
	t.Run("mismatch", func(t *testing.T) {
		assert.Equal(t, "mismatch", bcryptErrorKind(bcrypt.ErrMismatchedHashAndPassword))
	})
	t.Run("hash_too_short", func(t *testing.T) {
		assert.Equal(t, "hash_too_short", bcryptErrorKind(bcrypt.ErrHashTooShort))
	})
	t.Run("version_too_new", func(t *testing.T) {
		assert.Equal(t, "version_too_new", bcryptErrorKind(bcrypt.HashVersionTooNewError('3')))
	})
	t.Run("invalid_prefix", func(t *testing.T) {
		assert.Equal(t, "invalid_prefix", bcryptErrorKind(bcrypt.InvalidHashPrefixError('%')))
	})
	t.Run("other_for_unrecognized_error", func(t *testing.T) {
		assert.Equal(t, "other", bcryptErrorKind(errors.New("disk on fire")))
	})
}

func TestErrString(t *testing.T) {
	assert.Empty(t, errString(nil), "nil error must empty-string")
	assert.Equal(t, "boom", errString(errors.New("boom")))
}

func TestGenerateRandomBytes(t *testing.T) {
	t.Run("returns_requested_length", func(t *testing.T) {
		b, err := GenerateRandomBytes(32)
		require.NoError(t, err)
		assert.Len(t, b, 32)
	})

	t.Run("supports_zero_length", func(t *testing.T) {
		b, err := GenerateRandomBytes(0)
		require.NoError(t, err)
		assert.Empty(t, b)
	})

	t.Run("successive_calls_are_distinct", func(t *testing.T) {
		// A collision between two `crypto/rand` 16-byte samples is negligible.
		a, err := GenerateRandomBytes(16)
		require.NoError(t, err)
		b, err := GenerateRandomBytes(16)
		require.NoError(t, err)
		assert.NotEqual(t, a, b, "successive reads from crypto/rand must differ")
	})
}

func TestRecordSpanError(t *testing.T) {
	ctx := t.Context()
	_, span := hasherTracer().Start(ctx, "test.recordSpanError")

	t.Run("nil_error_is_no_op_no_panic", func(t *testing.T) {
		assert.NotPanics(t, func() { recordSpanError(span, nil) })
	})

	t.Run("non_nil_error_records_without_panic", func(t *testing.T) {
		assert.NotPanics(t, func() { recordSpanError(span, errors.New("record me")) })
	})

	span.End()
}

func TestCheckPasswordHashCtx_EnabledPaths(t *testing.T) {
	requirePasswordProtection(t)

	t.Run("argon2id_match_no_rehash", func(t *testing.T) {
		pw := "super-secret-value-123"
		encoded, err := HashPasswordCtx(t.Context(), pw)
		require.NoError(t, err)

		match, needsRehash := CheckPasswordHashCtx(t.Context(), pw, encoded)
		assert.True(t, match, "argon2id-hashed value must verify")
		assert.False(t, needsRehash, "argon2id match must NOT signal rehash")
	})

	t.Run("argon2id_wrong_password_rejected_via_constant_time", func(t *testing.T) {
		pw := "super-secret-value-123"
		encoded, err := HashPasswordCtx(t.Context(), pw)
		require.NoError(t, err)

		match, needsRehash := CheckPasswordHashCtx(t.Context(), "totally-wrong", encoded)
		assert.False(t, match)
		assert.False(t, needsRehash, "mismatch must not signal rehash")
	})

	t.Run("bcrypt_legacy_hash_matches_and_signals_rehash", func(t *testing.T) {
		// A matching legacy `bcrypt` hash must request an `argon2id` replacement.
		pw := "legacy-bcrypt-value"
		bcryptHash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
		require.NoError(t, err)

		match, needsRehash := CheckPasswordHashCtx(t.Context(), pw, string(bcryptHash))
		assert.True(t, match, "matching bcrypt hash must verify via fallback")
		assert.True(t, needsRehash, "bcrypt match MUST signal rehash to argon2id")
	})

	t.Run("malformed_hash_rejected_false_false", func(t *testing.T) {
		match, needsRehash := CheckPasswordHashCtx(t.Context(), "any", "totally-malformed")
		assert.False(t, match)
		assert.False(t, needsRehash)
	})
}

func TestCheckPasswordHashCtx_DisabledPath(t *testing.T) {
	if enabled {
		t.Skip("protection enabled - disabled-path behavior not applicable")
	}

	pw := "plain-text-when-disabled"
	encoded, err := HashPasswordCtx(t.Context(), pw)
	require.NoError(t, err)
	assert.Equal(t, pw, encoded, "disabled mode must store plaintext")

	match, needsRehash := CheckPasswordHashCtx(t.Context(), pw, encoded)
	assert.True(t, match)
	assert.False(t, needsRehash, "disabled path must never signal rehash")

	wrong, _ := CheckPasswordHashCtx(t.Context(), "not-the-plaintext", encoded)
	assert.False(t, wrong)
}
