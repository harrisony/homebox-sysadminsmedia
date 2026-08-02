package hasher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	t.Parallel()
	requirePasswordProtection(t)

	type args struct {
		password      string
		invalidInputs []string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "letters_and_numbers",
			args: args{
				password:      "password123456788",
				invalidInputs: []string{"testPassword", "AnotherBadPassword", "ThisShouldNeverWork", "1234567890"},
			},
		},
		{
			name: "letters_number_and_special",
			args: args{
				password:      "!2afj3214pofajip3142j;fa",
				invalidInputs: []string{"testPassword", "AnotherBadPassword", "ThisShouldNeverWork", "1234567890"},
			},
		},
		{
			name: "extra_long_password",
			args: args{
				password:      "this_is_a_very_long_password_that_should_be_hashed_properly_and_still_work_with_the_check_function",
				invalidInputs: []string{"testPassword", "AnotherBadPassword", "ThisShouldNeverWork", "1234567890"},
			},
		},
		{
			name: "empty_password",
			args: args{
				password:      "",
				invalidInputs: []string{"testPassword", "AnotherBadPassword", "ThisShouldNeverWork", "1234567890"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HashPassword(tt.args.password)
			require.NoError(t, err)

			check, needsRehash := CheckPasswordHash(tt.args.password, got)
			assert.True(t, check, "failed to validate password=%v against hash=%v", tt.args.password, got)
			assert.False(t, needsRehash, "a freshly minted hash asked to be rehashed")

			for _, invalid := range tt.args.invalidInputs {
				check, _ := CheckPasswordHash(invalid, got)
				assert.False(t, check, "improperly validated password=%v against hash=%v", invalid, got)
			}
		})
	}
}
