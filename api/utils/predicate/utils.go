package predicate

import "github.com/equinor/radix-common/utils/slice"

// MatchAll combines multiple predicates and returns true if aLL predicates return true
func MatchAll[T any](predicates ...func(T) bool) func(T) bool {
	return func(t T) bool {
		return slice.All(predicates, func(predFunc func(T) bool) bool { return predFunc(t) })
	}
}
