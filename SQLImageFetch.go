/*
HostManager provides a lightweight command-line tool for managing the hosts file on Windows.
Author: Shang Yanjin
Email: shangyanjin@msn.com
*/
package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

func main() {
	// Default SQL file path
	var filePath string

	// Check if the SQL file path is provided as an argument
	if len(os.Args) < 2 {
		files, err := filepath.Glob("*.sql")
		if err != nil || len(files) == 0 {
			fmt.Println("Usage: program <SQL file path>")
			fmt.Println("No SQL file specified and no .sql file found in the current directory.")
			return
		}
		// Use the first found .sql file in the current directory
		filePath = files[0]
		fmt.Printf("No SQL file specified. Using %s as the default file.\n", filePath)
	} else {
		filePath = os.Args[1]
	}

	// Read the content of the SQL file
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	// Parse the SQL data to extract unique URLs
	result := parseSQLData(string(fileContent))
	uniqueURLs := removeDuplicates(result)

	// Print the total number of unique URLs found
	totalFiles := len(uniqueURLs)
	fmt.Printf("Found %d unique remote files.\n", totalFiles)

	// Create a directory to store downloaded images
	imageDir := filepath.Join(filepath.Dir(filePath), "image")
	if err := os.MkdirAll(imageDir, os.ModePerm); err != nil {
		panic(err)
	}

	// Download images using multiple threads
	var wg sync.WaitGroup
	urlChan := make(chan string, totalFiles)
	threadCount := 3
	progress := make(chan string, totalFiles)

	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				if downloadImage(url, imageDir) {
					progress <- url
				}
			}
		}()
	}

	// Start a goroutine to display progress
	go func() {
		downloadedFiles := 0
		for url := range progress {
			downloadedFiles++
			fmt.Printf("Downloaded %d/%d: %s\n", downloadedFiles, totalFiles, url)
		}
	}()

	for _, url := range uniqueURLs {
		urlChan <- url
	}
	close(urlChan)

	wg.Wait()
	close(progress)
	fmt.Printf("Download completed. Total files: %d.\n", totalFiles)
}

// removeDuplicates removes duplicate URLs from the parsed SQL data
func removeDuplicates(data [][]string) []string {
	urlMap := make(map[string]bool)
	var uniqueURLs []string

	for _, row := range data {
		url := row[1]
		if _, exists := urlMap[url]; !exists {
			urlMap[url] = true
			uniqueURLs = append(uniqueURLs, url)
		}
	}
	return uniqueURLs
}

// parseSQLData parses the SQL file content to extract image URLs
func parseSQLData(sqlData string) [][]string {
	var result [][]string
	scanner := bufio.NewScanner(strings.NewReader(sqlData))
	tableNamePattern := regexp.MustCompile(`INSERT\s+INTO\s+` + "`?" + `(\w+)` + "`?" + `\s+VALUES`)
	imageURLPattern := regexp.MustCompile(`(http|https):\/\/[\w\-\.\/]+\.(jpg|png|gif)`)

	for scanner.Scan() {
		line := scanner.Text()
		tableNameMatches := tableNamePattern.FindStringSubmatch(line)
		if len(tableNameMatches) < 2 {
			continue
		}
		tableName := tableNameMatches[1]
		imageURLs := imageURLPattern.FindAllString(line, -1)

		for _, url := range imageURLs {
			result = append(result, []string{tableName, url})
		}
	}
	return result
}

// downloadImage downloads an image from the specified URL and saves it to the specified directory
func downloadImage(url, dir string) bool {
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	filePath := filepath.Join(dir, fileName)

	// Check if the file already exists and its size matches the remote file
	if fileInfo, err := os.Stat(filePath); !os.IsNotExist(err) {
		resp, err := http.Head(url)
		if err != nil {
			fmt.Println("Failed to get remote file info:", url, err)
			return false
		}
		defer resp.Body.Close()

		if resp.ContentLength == fileInfo.Size() {
			fmt.Println("File already exists and size matches, skipping download:", url)
			return false
		}
	}

	// Download the image
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Download failed:", url, err)
		return false
	}
	defer resp.Body.Close()

	// Create the file and write the downloaded content to it
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Failed to create file:", filePath, err)
		return false
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		fmt.Println("Failed to write to file:", filePath, err)
		return false
	}

	return true
}
