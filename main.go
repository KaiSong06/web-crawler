package main

import (
	"fmt"
	"sync"
)

// Stack
type Stack struct {
	elements []string
	mu       sync.Mutex
}

func (s *Stack) push(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.elements = append(s.elements, url)
}

func (s *Stack) pop() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.elements) == 0 {
		return "", false
	}
	url := s.elements[len(s.elements)-1]
	s.elements = s.elements[:len(s.elements)-1]
	return url, true
}

func (s *Stack) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.elements)
}

func main() {
	fmt.Println("Hello, World!")
}
