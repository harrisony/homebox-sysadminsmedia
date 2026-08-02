package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	topicTemplate   = "mem://{{.Topic}}"
	topicThumbnails = "thumbnails"
)

func TestGenerateSubPubConn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pubSubConn string
		topic      string
		want       string
		wantErr    string
	}{
		{
			name:       "placeholder is substituted",
			pubSubConn: "nats://user:pass@localhost:4222/{{.Topic}}",
			topic:      topicThumbnails,
			want:       "nats://user:pass@localhost:4222/thumbnails",
		},
		{
			name:       "placeholder is substituted at every occurrence",
			pubSubConn: "mem://{{.Topic}}?ack={{.Topic}}",
			topic:      "exports",
			want:       "mem://exports?ack=exports",
		},
		{
			name:       "placeholder with surrounding whitespace",
			pubSubConn: "mem://{{ .Topic }}",
			topic:      "exports",
			want:       "mem://exports",
		},
		{
			name:       "connection string without a placeholder is returned verbatim",
			pubSubConn: "mem://static-topic",
			topic:      topicThumbnails,
			want:       "mem://static-topic",
		},
		{
			name:       "empty connection string yields an empty result",
			pubSubConn: "",
			topic:      topicThumbnails,
			want:       "",
		},
		{
			name:       "empty topic substitutes an empty string",
			pubSubConn: topicTemplate,
			topic:      "",
			want:       "mem://",
		},
		{
			name:       "topic is not re-evaluated as a template",
			pubSubConn: topicTemplate,
			topic:      "not-a-placeholder .Topic",
			want:       "mem://not-a-placeholder .Topic",
		},
		{
			name:       "topic containing an opening placeholder is rejected",
			pubSubConn: topicTemplate,
			topic:      "{{.Topic}}",
			wantErr:    "topic contains template placeholders",
		},
		{
			name:       "topic containing only an opening brace pair is rejected",
			pubSubConn: topicTemplate,
			topic:      "evil{{",
			wantErr:    "topic contains template placeholders",
		},
		{
			name:       "topic containing only a closing brace pair is rejected",
			pubSubConn: topicTemplate,
			topic:      "evil}}",
			wantErr:    "topic contains template placeholders",
		},
		{
			name:       "unterminated action in the connection string fails to parse",
			pubSubConn: "mem://{{.Topic",
			topic:      topicThumbnails,
			wantErr:    "failed to parse template",
		},
		{
			name:       "unknown function in the connection string fails to parse",
			pubSubConn: "mem://{{nope .Topic}}",
			topic:      topicThumbnails,
			wantErr:    "failed to parse template",
		},
		{
			name:       "field access on the topic string fails to execute",
			pubSubConn: "mem://{{.Topic.Name}}",
			topic:      topicThumbnails,
			wantErr:    "failed to parse template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GenerateSubPubConn(tt.pubSubConn, tt.topic)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Empty(t, got, "no connection string may be returned alongside an error")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateSubPubConnRejectsMissingKey(t *testing.T) {
	t.Parallel()

	got, err := GenerateSubPubConn("mem://{{.Subject}}", topicThumbnails)

	require.Error(t, err)
	assert.Empty(t, got)
	assert.ErrorContains(t, err, "map has no entry for key")
}

// FuzzGenerateSubPubConn requires accepted topics to remain verbatim.
// Template delimiters must not inject actions.
func FuzzGenerateSubPubConn(f *testing.F) {
	f.Add(topicThumbnails)
	f.Add("")
	f.Add("{{.Topic}}")
	f.Add("a/b?c=d")
	f.Add("{{")

	const tmpl = "mem://host/"

	f.Fuzz(func(t *testing.T, topic string) {
		got, err := GenerateSubPubConn(tmpl+"{{.Topic}}", topic)

		if strings.Contains(topic, "{{") || strings.Contains(topic, "}}") {
			require.Error(t, err)
			require.Empty(t, got)

			return
		}

		require.NoError(t, err)
		require.Equal(t, tmpl+topic, got)
	})
}
