package utils

import (
	"cmp"
	"iter"
	"slices"
)

type StableMap[K comparable, V any] []Store[K, V]

type Store[K comparable, V any] struct {
	Key   K
	Value V
}

func CompareValue[K comparable, V cmp.Ordered]() func(a, b Store[K, V]) int {
	return func(a, b Store[K, V]) int {
		return cmp.Compare(a.Value, b.Value)
	}
}

func SortMapByValue[K comparable, V cmp.Ordered](s StableMap[K, V]) {
	slices.SortFunc(s, CompareValue[K, V]())
}

func MapToSlice[M ~map[K]V, K comparable, V any](m M) StableMap[K, V] {
	if m == nil {
		return nil
	}
	s := make(StableMap[K, V], 0, len(m))
	for key, value := range m {
		s = append(s, Store[K, V]{Key: key, Value: value})
	}
	return s
}

func (s StableMap[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, value := range slices.Backward(s) {
			if !yield(value.Key, value.Value) {
				return
			}
		}
	}
}

func (s StableMap[K, V]) Seq2() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, value := range s {
			if !yield(value.Key, value.Value) {
				return
			}
		}
	}
}

func CountEqual[M ~map[K]V, K comparable, V comparable](m M, v V) int {
	var i int
	for _, value := range m {
		if value == v {
			i++
		}
	}
	return i
}
