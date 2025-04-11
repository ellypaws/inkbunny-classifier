package utils

import (
	"iter"
)

func Map[Slice ~[]E, E any](s Slice, transform func(E) E) iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		for i, v := range s {
			if !yield(i, transform(v)) {
				return
			}
		}
	}
}
