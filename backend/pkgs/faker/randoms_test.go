package faker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const loops = 500

func validateUnique(values []string) bool {
	for i := range values {
		for j := i + 1; j < len(values); j++ {
			if values[i] == values[j] {
				return false
			}
		}
	}
	return true
}

func Test_GetRandomString(t *testing.T) {
	t.Parallel()
	// Test that the function returns a string of the correct length
	generated := make([]string, loops)

	faker := NewFaker()

	for i := range loops {
		generated[i] = faker.Str(10)
		assert.Len(t, generated[i], 10)
	}

	if !validateUnique(generated) {
		t.Error("Generated values are not unique")
	}
}

func Test_GetRandomEmail(t *testing.T) {
	t.Parallel()
	// Test that the function returns a string of the correct length
	generated := make([]string, loops)

	faker := NewFaker()

	for i := range loops {
		generated[i] = faker.Email()
	}

	if !validateUnique(generated) {
		t.Error("Generated values are not unique")
	}
}

func Test_GetRandomBool(t *testing.T) {
	t.Parallel()

	trues := 0
	falses := 0

	faker := NewFaker()

	for range loops {
		if faker.Bool() {
			trues++
		} else {
			falses++
		}
	}

	if trues == 0 || falses == 0 {
		t.Error("Generated boolean don't appear random")
	}
}

func Test_RandomNumber(t *testing.T) {
	t.Parallel()

	f := NewFaker()

	const MIN = 0
	const MAX = 100

	seen := make(map[int]struct{}, MAX-MIN)

	for range loops {
		n := f.Num(MIN, MAX)

		assert.GreaterOrEqual(t, n, MIN)
		assert.Less(t, n, MAX)

		seen[n] = struct{}{}
	}

	assert.Greater(t, len(seen), 1, "Num returned a constant value across %d draws", loops)
}
