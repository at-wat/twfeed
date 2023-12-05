package main

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	testCases := map[string]struct {
		a        []string
		b        []string
		expected []string
	}{
		"Overwrap": {
			a: []string{"aaa", "bbb", "ccc"},
			b: []string{"bbb", "ccc", "ddd", "eee"},
			expected: []string{
				"aaa", "bbb", "ccc", "ddd", "eee",
			},
		},
		"FullOverwrap": {
			a: []string{"aaa", "bbb", "ccc"},
			b: []string{"bbb", "ccc"},
			expected: []string{
				"aaa", "bbb", "ccc",
			},
		},
		"Same": {
			a: []string{"aaa", "bbb", "ccc"},
			b: []string{"aaa", "bbb", "ccc"},
			expected: []string{
				"aaa", "bbb", "ccc",
			},
		},
		"NoOverwrap": {
			a: []string{"aaa", "bbb", "ccc"},
			b: []string{"ddd", "eee", "fff"},
			expected: []string{
				"aaa", "bbb", "ccc", "ddd", "eee", "fff",
			},
		},
	}

	for name, tt := range testCases {
		tt := tt
		t.Run(name, func(t *testing.T) {
			out := merge(tt.a, tt.b)
			if !reflect.DeepEqual(tt.expected, out) {
				t.Errorf("output differs\nexpected: %v\n     got: %v", tt.expected, out)
			}
		})
	}
}
