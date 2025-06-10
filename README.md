# Web Crawler with S3 Storage

A concurrent web crawler written in Go that stores crawled page information in AWS S3 by traversing in a Depth First First pattern.

## Prerequisites

- Go 1.16 or later
- AWS CLI configured with appropriate credentials
- AWS S3 bucket created
- Required Go packages:
  - github.com/aws/aws-sdk-go-v2/config
  - github.com/aws/aws-sdk-go-v2/service/s3
  - golang.org/x/net/html

## Installation

1. Clone the repository:
```bash
git clone https://github.com/KaiSong06/web-crawler.git
cd web_crawler
```

2. Install dependencies:
```bash
go mod init web_crawler
go mod tidy
```

## AWS Configuration

1. Configure AWS credentials using one of these methods:
   - Set environment variables:
     ```bash
     export AWS_ACCESS_KEY_ID=your_access_key
     export AWS_SECRET_ACCESS_KEY=your_secret_key
     export AWS_REGION=your_region
     ```
   - Use AWS CLI: `aws configure`
   - Create `~/.aws/credentials` file

2. Create an S3 bucket in your AWS account

## Usage

1. Run the crawler:
```bash
go run main.go
```

2. When prompted, enter:
   - S3 bucket name
   - Starting URL to crawl
   - Maximum number of pages to crawl

The crawler will:
- Start from the given URL
- Follow links recursively
- Store JSON files in S3 with page information
- Filter out unwanted content (social media, binary files, etc.)

## Output Format

Each crawled page creates a JSON file in S3:
```json
{
    "url": "https://example.com",
    "title": "Page Title",
    "url_count": 42
}
```

## Filtering

The crawler automatically filters:
- Common social media domains
- CDN and analytics URLs
- Binary and document files
- Asset files (images, CSS, JS)

## Configuration

Edit the `filter()` function in `main.go` to modify:
- Excluded domains
- Excluded file extensions
- URL patterns to skip

## Error Handling

- Failed requests are skipped
- Non-HTML content is ignored
