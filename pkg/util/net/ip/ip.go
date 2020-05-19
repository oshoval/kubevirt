/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2020 Red Hat, Inc.
 *
 */
package ip

import (
	"io/ioutil"
	"net"
	"strings"

	netutils "k8s.io/utils/net"
)

const (
	// sysfs
	disableIPv6Path = "/proc/sys/net/ipv6/conf/default/disable_ipv6"

	ipv4Loopback = "127.0.0.1"
)

// GetIPZeroAddress returns INADDR_ANY or INADDR6_ANY (according sysctl disable_ipv6)
func GetIPZeroAddress() string {
	if isIPv6Disabled(disableIPv6Path) {
		return net.IPv4zero.String()
	}

	return net.IPv6zero.String()
}

// GetLoopbackAddress returns loopback IP (either 127.0.0.1 or [::1] according sysctl disable_ipv6)
func GetLoopbackAddress() string {
	if isIPv6Disabled(disableIPv6Path) {
		return ipv4Loopback
	}

	return net.IPv6loopback.String()
}

// IsLoopbackAddress checks if the address is IPv4 / IPv6 loopback address
func IsLoopbackAddress(ipAddress string) bool {
	loopback := net.ParseIP(ipAddress)
	return loopback.IsLoopback()
}

// NormalizeIPAddress returns normalized IP, adding square brackets for IPv6 if needed
func NormalizeIPAddress(ipAddress string) string {
	if netutils.IsIPv6String(ipAddress) {
		return "[" + ipAddress + "]"
	}

	return ipAddress
}

func isIPv6Disabled(path string) bool {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return true
	}

	disableIPv6 := strings.TrimSuffix(string(data), "\n")
	return disableIPv6 == "1"
}
