package textutils

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func FuzzNormalizeSearchQuery(f *testing.F) {
	seeds := []string{
		"",
		"café",
		"electrónica",
		"père",
		"plain ascii text",
		"MiXeD Café 123",
		"日本語",
		"á",
		"�",
		"noël garçon",
		"\x00control",
		"tab\tand\nnew",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		normalized := NormalizeSearchQuery(s)
		require.True(t, utf8.ValidString(normalized), "NormalizeSearchQuery must return valid UTF-8")
		require.Equal(t, normalized, NormalizeSearchQuery(normalized), "NormalizeSearchQuery must be idempotent")

		accentless := RemoveAccents(s)
		require.True(t, utf8.ValidString(accentless), "RemoveAccents must return valid UTF-8")
		require.Equal(t, accentless, RemoveAccents(accentless), "RemoveAccents must be idempotent")
	})
}
