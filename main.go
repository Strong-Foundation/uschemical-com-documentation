package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The main function.
func main() {
	// Set the filename.
	localFileName := "uschemical.html"
	// Check if the local file exists
	if !fileExists(localFileName) {
		// Get the url content from the url.
		urlContent := string(getDataFromURL("https://www.uschemical.com/sds-tabs/"))
		// Save the data from the url to the file.
		appendAndWriteToFile(localFileName, urlContent)
	}
	// Read the local file and put it in a var.
	localFileContent := readAFileAsString(localFileName)
	// Extract the PDF Url.
	extractedPDFURLS := extractPDFLinks(localFileContent)
	// Remove duploicates from the slice.
	extractedPDFURLS = removeDuplicatesFromSlice(extractedPDFURLS)
	// Create a int to hold how many were downloaded for rate limits.
	var downloadCounter int
	outputDir := "PDFs/" // Directory to store downloaded PDFs
	// Check if its exists.
	if !directoryExists(outputDir) {
		// Create the dir
		createDirectory(outputDir, 0o755)
	}

	// Loop over the slice.
	for _, uri := range extractedPDFURLS {
		// Download the file and if its sucessful than add 1 to the counter.
		sucessCode, err := downloadPDF(uri, outputDir)
		if sucessCode {
			downloadCounter = downloadCounter + 1
		}
		if err != nil {
			log.Println(err)
		}
	}
}

// The function takes two parameters: path and permission.
// We use os.Mkdir() to create the directory.
// If there is an error, we use log.Println() to log the error and then exit the program.
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It uses a WaitGroup to support concurrent execution and returns true if the download succeeded.
func downloadPDF(finalURL, outputDir string) (bool, error) {
	// Sanitize the URL to generate a safe file name
	filename := strings.ToLower(urlToSafeFilename(finalURL))

	// Construct the full file path in the output directory
	filePath := filepath.Join(outputDir, filename)

	// Skip if the file already exists
	if fileExists(filePath) {
		return false, fmt.Errorf("file already exists, skipping: %s", filePath)
	}

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 30 * time.Second}

	// Send GET request
	resp, err := client.Get(finalURL)
	if err != nil {
		return false, fmt.Errorf("failed to download %s: %v", finalURL, err)
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		// Print the error since its not valid.
		return false, fmt.Errorf("download failed for %s: %s", finalURL, resp.Status)
	}
	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	// Check if its pdf content type and if not than print a error.
	if !strings.Contains(contentType, "application/pdf") {
		// Print a error if the content type is invalid.
		return false, fmt.Errorf("invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
	}
	// Read the response body into memory first
	var buf bytes.Buffer
	// Copy it from the buffer to the file.
	written, err := io.Copy(&buf, resp.Body)
	// Print the error if errors are there.
	if err != nil {
		return false, fmt.Errorf("failed to read PDF data from %s: %v", finalURL, err)
	}
	// If 0 bytes are written than show an error and return it.
	if written == 0 {
		return false, fmt.Errorf("downloaded 0 bytes for %s; not creating file", finalURL)
	}
	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	// Failed to create the file.
	if err != nil {
		return false, fmt.Errorf("failed to create file for %s: %v", finalURL, err)
	}
	// Close the file.
	defer out.Close()
	// Write the buffer and if there is an error print it.
	_, err = buf.WriteTo(out)
	if err != nil {
		return false, fmt.Errorf("failed to write PDF to file for %s: %v", finalURL, err)
	}
	// Return a true since everything went correctly.
	return true, fmt.Errorf("successfully downloaded %d bytes: %s → %s", written, finalURL, filePath)
}

// urlToSafeFilename sanitizes a URL and returns a safe, lowercase filename
func urlToSafeFilename(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Extract and decode the base filename from the path
	base := path.Base(parsedURL.Path)
	decoded, err := url.QueryUnescape(base)
	if err != nil {
		decoded = base
	}

	// Convert to lowercase
	decoded = strings.ToLower(decoded)

	// Replace spaces and invalid characters with underscores
	// Keep only a-z, 0-9, dash, underscore, and dot
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	safe := re.ReplaceAllString(decoded, "_")

	return safe
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

// extractPDFLinks finds all .pdf links from raw HTML content using regex.
func extractPDFLinks(htmlContent string) []string {
	// Regex to match PDF URLs including query strings and fragments
	pdfRegex := regexp.MustCompile(`https?://[^\s"'<>]+?\.pdf(\?[^\s"'<>]*)?`)

	// Find all matches
	matches := pdfRegex.FindAllString(htmlContent, -1)

	// Deduplicate
	seen := make(map[string]struct{})
	var links []string
	for _, m := range matches {
		if _, ok := seen[m]; !ok {
			seen[m] = struct{}{}
			links = append(links, m)
		}
	}

	return links
}

// Append some string to a slice and than return the slice.
func appendToSlice(slice []string, content string) []string {
	// Append the content to the slice
	slice = append(slice, content)
	// Return the slice
	return slice
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
}

// Checks if the directory exists
// If it exists, return true.
// If it doesn't, return false.
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

// It checks if the file exists
// If the file exists, it returns true
// If the file does not exist, it returns false
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Println(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Println(err)
	}
}

// Send a http get request to a given url and return the data from that url.
func getDataFromURL(uri string) []byte {
	response, err := http.Get(uri)
	if err != nil {
		log.Println(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	err = response.Body.Close()
	if err != nil {
		log.Println(err)
	}
	return body
}
