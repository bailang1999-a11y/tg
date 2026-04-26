package handlers

import (
	"reflect"
	"testing"
)

func TestPickDistributedValue(t *testing.T) {
	values := []string{"A", "B", "C"}
	got := []string{}
	for i := 0; i < 6; i++ {
		got = append(got, pickDistributedValue(values, i, 6))
	}
	want := []string{"A", "A", "B", "B", "C", "C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("distributed values = %#v, want %#v", got, want)
	}
}

func TestCleanProfileValues(t *testing.T) {
	got := cleanProfileValues([]string{" Alice ", "", "\tBob\n", "   "})
	want := []string{"Alice", "Bob"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanProfileValues = %#v, want %#v", got, want)
	}
}
