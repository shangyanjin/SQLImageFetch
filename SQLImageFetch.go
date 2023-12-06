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
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: program <sql文件路径>")
		return
	}
	filePath := os.Args[1]

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	result := parseSQLData(string(fileContent))
	uniqueURLs := removeDuplicates(result)

	totalFiles := len(uniqueURLs)
	fmt.Printf("共发现 %d 个唯一的远程文件\n", totalFiles)

	imageDir := filepath.Join(filepath.Dir(filePath), "image")
	if err := os.MkdirAll(imageDir, os.ModePerm); err != nil {
		panic(err)
	}

	downloadedFiles := 0
	for i, url := range uniqueURLs {
		fmt.Printf("正在下载 %d/%d: %s\n", i+1, totalFiles, url)
		if downloadImage(url, imageDir) {
			downloadedFiles++
		}
	}
	fmt.Printf("下载结束。共 %d 个文件，本次下载 %d 个。\n", totalFiles, downloadedFiles)
}

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

func downloadImage(url, dir string) bool {
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	filePath := filepath.Join(dir, fileName)

	if fileInfo, err := os.Stat(filePath); !os.IsNotExist(err) {
		resp, err := http.Head(url)
		if err != nil {
			fmt.Println("获取远程文件信息失败:", url, err)
			return false
		}
		defer resp.Body.Close()

		if resp.ContentLength == fileInfo.Size() {
			fmt.Println("文件已存在，且大小一致，跳过下载:", url)
			return false
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("下载失败:", url, err)
		return false
	}
	defer resp.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("创建文件失败:", filePath, err)
		return false
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		fmt.Println("写入文件失败:", filePath, err)
		return false
	}

	return true
}
