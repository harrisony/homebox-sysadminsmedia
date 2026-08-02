package hasher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestTimingEqualizationHashIsValidArgon2 guards against regressing the login
// account-enumeration timing side channel. The "user not found" / "no password"
// login paths call CheckDummyPasswordHashCtx to equalize response timing with a
// real login. That only works if the dummy comparison runs the argon2id KDF, which
// requires a *valid* argon2id hash — an arbitrary string would fail to decode
// instantly and perform no cryptographic work, reopening the side channel.
func TestTimingEqualizationHashIsValidArgon2(t *testing.T) {
	requirePasswordProtection(t)

	h := timingEqualizationHash()
	require.NotEmpty(t, h, "timing equalization hash is empty")

	params, salt, hash, err := decodeHash(h)
	require.NoError(t, err, "timing equalization hash must be decodable argon2id: %q", h)
	require.NotNil(t, params)
	require.NotEmpty(t, salt)
	require.NotEmpty(t, hash)
}

// TestStaticDummyHashIsValid ensures the compiled-in fallback used when runtime
// hash generation fails is itself a valid argon2id hash.
func TestStaticDummyHashIsValid(t *testing.T) {
	params, salt, hash, err := decodeHash(staticDummyHash)
	require.NoError(t, err, "staticDummyHash must decode as argon2id")
	require.NotNil(t, params)
	require.NotEmpty(t, salt)
	require.NotEmpty(t, hash)
}

// TestCheckDummyPasswordHashDoesRealWork verifies the dummy comparison executes
// the `argon2id` KDF used to resist account-enumeration timing attacks.
func TestCheckDummyPasswordHashDoesRealWork(t *testing.T) {
	requirePasswordProtection(t)

	ctx, recorded := recordTrace(t, "dummy-password-hash")

	CheckDummyPasswordHashCtx(ctx)

	spans := recorded()

	checks := spansNamed(spans, "hasher.CheckPasswordHash")
	require.Len(t, checks, 1, "dummy comparison must reach the real password check")

	assert.Equal(t, "argon2id", spanAttr(t, checks[0], "password.hash.prefix").AsString(),
		"dummy comparison must run against a genuine argon2id hash")
	assert.True(t, spanAttr(t, checks[0], "password.argon2.decode_ok").AsBool(),
		"a hash that fails to decode performs no cryptographic work")
	assert.False(t, spanAttr(t, checks[0], "password.argon2.match").AsBool(),
		"the placeholder password must not match the equalization hash")

	assert.Len(t, spansNamed(spans, "hasher.comparePasswordAndHash.derive"), 1,
		"argon2.IDKey must actually run - this span is the KDF")
}

// recordTrace replaces the process-global tracer. Callers must remain serial.
func recordTrace(t *testing.T, name string) (context.Context, func() []sdktrace.ReadOnlySpan) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	previous := otel.GetTracerProvider()

	otel.SetTracerProvider(provider)

	t.Cleanup(func() {
		otel.SetTracerProvider(previous)

		_ = provider.Shutdown(context.Background())
	})

	ctx, root := provider.Tracer("hasher_test").Start(t.Context(), name)
	traceID := root.SpanContext().TraceID()

	return ctx, func() []sdktrace.ReadOnlySpan {
		root.End()

		var spans []sdktrace.ReadOnlySpan

		for _, span := range recorder.Ended() {
			if span.SpanContext().TraceID() == traceID {
				spans = append(spans, span)
			}
		}

		return spans
	}
}

func spansNamed(spans []sdktrace.ReadOnlySpan, name string) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan

	for _, span := range spans {
		if span.Name() == name {
			out = append(out, span)
		}
	}

	return out
}

func spanAttr(t *testing.T, span sdktrace.ReadOnlySpan, key attribute.Key) attribute.Value {
	t.Helper()

	for _, kv := range span.Attributes() {
		if kv.Key == key {
			return kv.Value
		}
	}

	t.Fatalf("span %q has no attribute %q", span.Name(), key)

	return attribute.Value{}
}
