package main

import (
	"fmt"
	"slices"
	"testing"
)

type Set[T comparable] struct {
	m map[T]struct{}
}

func newSet[T comparable]() *Set[T] {
	return &Set[T]{make(map[T]struct{})}
}

func (s *Set[T]) Add(key T) {
	s.m[key] = struct{}{}
}

func (s *Set[T]) Contains(key T) bool {
	_, ok := s.m[key]
	return ok
}

func (s *Set[T]) Remove(key T) {
	delete(s.m, key)
}

func (s *Set[T]) Len() int {
	return len(s.m)
}

func (s *Set[T]) Slice() []T {
	result := make([]T, 0, s.Len())
	for k := range s.m {
		result = append(result, k)
	}
	return result
}

func Union[T comparable](s *Set[T], other *Set[T]) *Set[T] {
	set := newSet[T]()
	for k := range s.m {
		set.Add(k)
	}
	for k := range other.m {
		set.Add(k)
	}
	return set
}

func Intersect[T comparable](s *Set[T], other *Set[T]) *Set[T] {
	set := newSet[T]()
	for k := range other.m {
		if s.Contains(k) {
			set.Add(k)
		}
	}
	return set
}

func Map[T, U any](s []T, f func(T) U) []U {
	m := make([]U, len(s))

	for i, v := range s {
		m[i] = f(v)
	}
	return m
}

func Filter[T any](s []T, f func(T) bool) []T {
	m := make([]T, 0, len(s))
	for _, v := range s {
		if f(v) {
			m = append(m, v)
		}
	}
	return m
}

func Reduce[T, U any](s []T, init U, f func(U, T) U) U {
	acc := init
	for _, v := range s {
		acc = f(acc, v)
	}
	return acc
}

type Number interface {
	~int | ~int64 | ~float64
}

func Sum[T Number](nums []T) T {
	var zero T
	return Reduce(nums, zero, func(a T, b T) T { return a + b })
}

func TestMap(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"doubles", []int{1, 2, 3}, []int{2, 4, 6}},
		{"empty", []int{}, []int{}},
		{"single", []int{5}, []int{10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Map(tt.in, func(n int) int { return n * 2 })
			if !slices.Equal(got, tt.want) {
				t.Errorf("Map(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestUnion(t *testing.T) {
	tests := []struct {
		name string
		a, b []int
		want []int
	}{
		{"some overlap", []int{1, 2, 3}, []int{2, 3, 4}, []int{1, 2, 3, 4}},
		{"empty", []int{}, []int{}, []int{}},
		{"no overlap", []int{1, 2, 3}, []int{4, 5, 6}, []int{1, 2, 3, 4, 5, 6}},
		{"all overlap", []int{1, 2, 3, 4}, []int{1, 2, 3, 4}, []int{1, 2, 3, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newSet[int]()
			for _, i := range tt.a {
				a.Add(i)
			}
			b := newSet[int]()
			for _, i := range tt.b {
				b.Add(i)
			}
			want := newSet[int]()
			for _, i := range tt.want {
				want.Add(i)
			}
			got := Union[int](a, b)
			gotS := got.Slice()
			wantS := want.Slice()
			slices.Sort(gotS)
			slices.Sort(wantS)
			if !slices.Equal(gotS, wantS) {
				t.Errorf("Union(%v,%v) = %v, want %v", a.Slice(), b.Slice(), got.Slice(), want.Slice())
			}
		})
	}
}

func main() {
	nums := []int{1, 2, 3, 4}
	result := Sum(nums)
	fmt.Println(result)

}
