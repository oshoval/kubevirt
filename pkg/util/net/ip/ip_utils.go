package util

import (
	"bufio"
	"net"
	"os"
)

const DISABLE_IPV6_PATH = "/proc/sys/net/ipv6/conf/default/disable_ipv6"
const IPV4_BIND_ADDRESS = "0.0.0.0"
const IPV6_BIND_ADDRESS = "[::]"
const IPV4_LOOPBACK_ADDRESS = "127.0.0.1"
const IPV6_LOOPBACK_ADDRESS = "[::1]"

// GetIPBindAddress returns IP bind address (either 0.0.0.0 or [::] according sysctl disable_ipv6)
func GetIPBindAddress() string {
	var disableIPv6 string
	file, err := os.Open(DISABLE_IPV6_PATH)
	if err != nil {
		return IPV4_BIND_ADDRESS
	}

	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		disableIPv6 = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return IPV4_BIND_ADDRESS
	}

	if disableIPv6 == "1" {
		return IPV4_BIND_ADDRESS
	}

	return IPV6_BIND_ADDRESS
}

// GetLoopbackAddress returns loopback IP (either 127.0.0.1 or [::1] according sysctl disable_ipv6)
func GetLoopbackAddress() string {
	var disableIPv6 string
	file, err := os.Open(DISABLE_IPV6_PATH)
	if err != nil {
		return IPV4_LOOPBACK_ADDRESS
	}

	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		disableIPv6 = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return IPV4_LOOPBACK_ADDRESS
	}

	if disableIPv6 == "1" {
		return IPV4_LOOPBACK_ADDRESS
	}

	return IPV6_LOOPBACK_ADDRESS
}

// IsLoopbackAddress checks if the address is IPv4 / IPv6 loopback address
func IsLoopbackAddress(ipAddress string) bool {
	Loopback := net.ParseIP(ipAddress)
	return Loopback.IsLoopback()
}
