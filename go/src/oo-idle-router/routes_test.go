package main

import (
	//"fmt"
	"testing"
)

func TestRoute(t *testing.T) {
	b := NewRoute([]string{"www.example.com", "www.example2.com"}, "localhost", 22001)
	if b.Id() != "localhost:22001" {
		t.Errorf("Id should match target")
	}
}

// func TestChannel(t *testing.T) {
// 	done := make(chan bool)
// 	fmt.Println("About to enter select")
// 	select {
// 	case <-done:
// 		t.Errorf("Expected to pass through")
// 		return
// 	}
// }
