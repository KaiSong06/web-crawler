package main

import (
	"fmt"
	"hash/fnv"
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

// Set of crawled URLs
type CrawledURLs struct {
	urls   map[uint64]bool
	number int
	mu     sync.Mutex
}

func (c *CrawledURLs) add(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.urls[hashUrl(url)] = true
	c.number++
}

func (c *CrawledURLs) contains(url string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.urls[hashUrl(url)]
}

func (c *CrawledURLs) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.number
}

func hashUrl(url string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(url))
	return h.Sum64()
}

func main() {
	fmt.Println("Hello, World!")
}
