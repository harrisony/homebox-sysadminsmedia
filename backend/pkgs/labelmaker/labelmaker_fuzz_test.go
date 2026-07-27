package labelmaker

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// stripSpace uses the same Unicode whitespace classification as
// `splitCommandTemplate`.
func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}

		return r
	}, s)
}

func completeTemplateActions(s string) []string {
	actions := []string{}

	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			return actions
		}

		s = s[start:]

		end := strings.Index(s, "}}")
		if end < 0 {
			return actions
		}

		end += len("}}")
		actions = append(actions, s[:end])
		s = s[end:]
	}
}

// FuzzSplitCommandTemplate requires tokenization to preserve non-whitespace
// UTF-8 content and keep complete template actions within one argument.
// Empty tokens are forbidden.
func FuzzSplitCommandTemplate(f *testing.F) {
	seeds := []string{
		"",
		"lp -o raw",
		"lp {{.FileName}}",
		"echo {{ .TitleText }} > out.png",
		"cmd {{.A}}{{.B}}",
		"a {{ b c }} d",
		"  spaced   out  ",
		"lp -d printer {{ .FileName }}",
		"{{",
		"}}",
		"{{ unclosed",
		"trailing }}",
		"{{ .A }}{{ .B }} tail",
		"tab\tsep\nnewline",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		parts := splitCommandTemplate(s)
		for _, p := range parts {
			require.NotEmpty(t, p, "split produced an empty token for %q", s)
		}

		if utf8.ValidString(s) {
			require.Equal(t, stripSpace(s), stripSpace(strings.Join(parts, "")),
				"tokenizing must preserve all non-whitespace content for %q", s)

			for _, action := range completeTemplateActions(s) {
				require.Condition(t, func() bool {
					for _, part := range parts {
						if strings.Contains(part, action) {
							return true
						}
					}

					return false
				}, "template action %q was split across argv tokens for %q", action, s)
			}
		}
	})
}
