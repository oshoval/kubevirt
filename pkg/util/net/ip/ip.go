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
	"os"

	netutils "k8s.io/utils/net"
)

const (
	// sysfs
	DISABLE_IPV6_PATH = "/proc/sys/net/ipv6/conf/default/disable_ipv6"

	IPV4_LOOPBACK_ADDRESS = "127.0.0.1"
)

// GetIPZeroAddress returns INADDR_ANY or INADDR6_ANY (according sysctl disable_ipv6)
func GetIPZeroAddress() string {
	disableIPv6, err := readSysfsFile(DISABLE_IPV6_PATH)
	if err != nil || disableIPv6 == "1" {
		return string(net.IPv4zero)
	}

	return string(net.IPv6zero)
}

// GetLoopbackAddress returns loopback IP (either 127.0.0.1 or [::1] according sysctl disable_ipv6)
func GetLoopbackAddress() string {
	disableIPv6, err := readSysfsFile(DISABLE_IPV6_PATH)
	if err != nil || disableIPv6 == "1" {
		return IPV4_LOOPBACK_ADDRESS
	}

	return string(net.IPv6loopback)
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

func readSysfsFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
