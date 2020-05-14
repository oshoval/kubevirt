package util

import (
	"bufio"
	"os"
)

const DISABLE_IPV6_PATH = "/proc/sys/net/ipv6/conf/default/disable_ipv6"

// GetIPBindAddress returns IP bind address (either 0.0.0.0 or [::] according sysctl disable_ipv6)
func GetIPBindAddress() string {
	var disableIPv6 string
	file, err := os.Open(DISABLE_IPV6_PATH)
	if err != nil {
		return "0.0.0.0"
	}

	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		disableIPv6 = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "0.0.0.0"
	}

	if disableIPv6 == "1" {
		return "0.0.0.0"
	}

	return "[::]"
}

// GetLoopbackAddress returns loopback IP (either 127.0.0.1 or [::1] according sysctl disable_ipv6)
func GetLoopbackAddress() string {
	var disableIPv6 string
	file, err := os.Open(DISABLE_IPV6_PATH)
	if err != nil {
		return "127.0.0.1"
	}

	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		disableIPv6 = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "127.0.0.1"
	}

	if disableIPv6 == "1" {
		return "127.0.0.1"
	}

	return "[::1]"
}
