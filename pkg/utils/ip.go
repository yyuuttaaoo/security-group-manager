package utils

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

func GetCurrentIP() (string, error) {
	response, err := http.Get("https://cip.cc")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	// 解析返回的内容，查找IP地址
	// cip.cc返回格式: "IP\t: 117.143.169.175"
	content := string(body)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if strings.Contains(line, "IP") && strings.Contains(line, ":") {
			// 使用正则表达式提取IP地址
			re := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				return matches[1], nil
			}
		}
	}

	return "", fmt.Errorf("无法从响应中解析IP地址")
}
