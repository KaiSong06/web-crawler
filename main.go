package main

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"golang.org/x/net/html"
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

func (s *Stack) contains(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, element := range s.elements {
		if element == url {
			return true
		}
	}
	return false
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

func getHref(token html.Token) (pass bool, href string) {
	for _, a := range token.Attr {
		if a.Key == "href" {
			if len(a.Val) == 0 || !strings.HasPrefix(a.Val, "http") {
				pass = false
			}
			href = a.Val
			return true, href
		}
		href = a.Val
		pass = true
	}
	return pass, href
}

func fetchPage(url string, c chan []byte) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c <- []byte("")
		return
	}
	req.Header.Set("User-Agent", "GoatCrawler/1.0")

	resp, err := client.Do(req)
	if err != nil {
		c <- []byte("")
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		c <- []byte("")
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c <- []byte("")
		return
	}
	c <- body
}

func getTitle(doc *html.Node) string {
	var title string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = n.FirstChild.Data
			return
		}
		for child := n.FirstChild; child != nil && title == ""; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return title
}

func parseHTML(htmlContent []byte) ([]string, string) {
	var urls []string
	doc, err := html.Parse(bytes.NewReader(htmlContent))
	if err != nil {
		log.Printf("failed to parse HTML: %v", err)
		return urls, ""
	}

	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" && strings.HasPrefix(attr.Val, "http") {
					if !filter(attr.Val) {
						urls = append(urls, attr.Val)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	title := getTitle(doc)
	return urls, title
}

func filter(url string) bool {
	excludedDomains := []string{
		"fonts.googleapis.com",
		"fonts.gstatic.com",
		"cdn.jsdelivr.net",
		"cdnjs.cloudflare.com",
		"stackpath.bootstrapcdn.com",
		"ajax.googleapis.com",
		"code.jquery.com",
		"use.fontawesome.com",
		"google-analytics.com",
		"googletagmanager.com",
		"doubleclick.net",
		"facebook.net",
		"twitter.com",
		"instagram.com",
		"t.co",
		"linkedin.com",
		"ads.google.com",
		"?utm_", "?ref=", "?sessionid=", "?sort=", "?page=", "?offset=",
		"#",
		"accounts.google.com",
		"login.microsoftonline.com",
		"youtube.com/embed/",
		"facebook.com/plugins/",
		"instagram.com/embed/",
		"google.com",
	}

	excludedExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp",
		".mp4", ".mov", ".avi", ".mkv", ".webm",
		".mp3", ".wav", ".ogg", ".flac",
		".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx",
		".zip", ".rar", ".gz", ".tar", ".7z", ".exe", ".iso",
		".css", ".js", ".woff", ".woff2", ".ttf", ".eot", ".otf",
		".ico", ".apk", ".dmg",
	}

	for _, domain := range excludedDomains {
		if strings.Contains(strings.ToLower(url), domain) {
			return true
		}
	}

	// Check if URL ends with excluded extensions
	for _, ext := range excludedExtensions {
		if strings.HasSuffix(strings.ToLower(url), ext) {
			return true
		}
	}

	return false
}

func crawl(startURL string, s3Client *s3.Client, bucket string, maxPages int) {
	stack := &Stack{}
	crawled := &CrawledURLs{urls: make(map[uint64]bool)}
	stack.push(startURL)

	for stack.size() > 0 && crawled.size() < maxPages {
		curURL, pass := stack.pop()
		if !pass || crawled.contains(curURL) {
			continue
		}

		bodyChan := make(chan []byte)
		go fetchPage(curURL, bodyChan)
		body := <-bodyChan
		if len(body) == 0 {
			continue
		}

		crawled.add(curURL)

		urls, title := parseHTML(body)
		for _, url := range urls {
			if !crawled.contains(url) {
				stack.push(url)
			}
		}

		jsonData := []byte(fmt.Sprintf(`{"url": %q, "title": %q, "url_count": %d}`, curURL, title, len(urls)))
		key := fmt.Sprintf("pages/%d.json", time.Now().UnixNano())
		err := UploadToS3(s3Client, bucket, key, jsonData)
		if err != nil {
			log.Printf("failed to upload %s to S3: %v", curURL, err)
		} else {
			log.Printf("uploaded %s to S3 bucket %s with key %s", curURL, bucket, key)
		}
	}
}

func s3Setup() *s3.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	return s3.NewFromConfig(cfg)
}

func UploadToS3(s3Client *s3.Client, bucket, key string, body []byte) error {
	_, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        bytes.NewReader(body),
		ContentType: &([]string{"application/json"}[0]), // <- changed to JSON
		ACL:         types.ObjectCannedACLPrivate,       // Optional: change to PublicRead if needed
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}
	return nil
}

func main() {
	var bucket string
	var startURL string
	var maxPages int

	s3Client := s3Setup()

	fmt.Println("S3 Bucket Name:")
	fmt.Scanln(&bucket)
	fmt.Println("Start URL:")
	fmt.Scanln(&startURL)
	fmt.Println("Max Pages to Crawl:")
	fmt.Scanln(&maxPages)
	if bucket == "" || startURL == "" || maxPages <= 0 {
		log.Fatal("Invalid input. Please provide a valid S3 bucket name, start URL, and a positive number for max pages.")
	}

	crawl(startURL, s3Client, bucket, maxPages)
}
