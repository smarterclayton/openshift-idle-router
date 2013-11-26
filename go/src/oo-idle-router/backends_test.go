package main

import (
	"testing"
)

func TestBackend(t *testing.T) {
	b := NewInactiveBackend([]string{"www.example.com", "www.example2.com"}, "localhost", 22001)
	if b.Id() != "localhost:22001" {
		t.Errorf("Id should match target")
	}
}
