package utils

import (
	"fmt"
	"io"
	"net"
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

// ValidateIPOrCIDR checks if the input is a valid IP or CIDR.
// If it's a CIDR, it ensures the prefix length is >= 22.
func ValidateIPOrCIDR(input string) error {
	// Check if it's a simple IP
	if ip := net.ParseIP(input); ip != nil {
		return nil
	}

	// Check if it's a CIDR
	_, ipNet, err := net.ParseCIDR(input)
	if err != nil {
		return fmt.Errorf("invalid IP or CIDR format: %v", err)
	}

	// Check prefix length
	ones, _ := ipNet.Mask.Size()
	if ones < 22 {
		return fmt.Errorf("CIDR prefix length must be >= 22, got /%d", ones)
	}

	return nil
}
