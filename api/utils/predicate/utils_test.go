package predicate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MatchAll(t *testing.T) {
	mockPred := func(match bool) func(v any) bool {
		return func(_ any) bool {
			return match
		}
	}
	assert.True(t, MatchAll[any](mockPred(true), mockPred(true))(nil))
	assert.False(t, MatchAll[any](mockPred(true), mockPred(false))(nil))
}
