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
)

const (
	// sysfs
	DISABLE_IPV6_PATH = "/proc/sys/net/ipv6/conf/default/disable_ipv6"

	// Special IPs
	IPV4_BIND_ADDRESS     = "0.0.0.0" //INADDR_ANY
	IPV4_LOOPBACK_ADDRESS = "127.0.0.1"
	IPV6_BIND_ADDRESS     = "[::]" //INADDR6_ANY
	IPV6_LOOPBACK_ADDRESS = "[::1]"
)

// GetIPZeroAddress returns INADDR_ANY or INADDR6_ANY (according sysctl disable_ipv6)
func GetIPZeroAddress() string {
	disableIPv6, err := readSysfsFile(DISABLE_IPV6_PATH)
	if err != nil || disableIPv6 == "1" {
		return IPV4_BIND_ADDRESS
	}

	return IPV6_BIND_ADDRESS
}

// GetLoopbackAddress returns loopback IP (either 127.0.0.1 or [::1] according sysctl disable_ipv6)
func GetLoopbackAddress() string {
	disableIPv6, err := readSysfsFile(DISABLE_IPV6_PATH)
	if err != nil || disableIPv6 == "1" {
		return IPV4_LOOPBACK_ADDRESS
	}

	return IPV6_LOOPBACK_ADDRESS
}

// IsLoopbackAddress checks if the address is IPv4 / IPv6 loopback address
func IsLoopbackAddress(ipAddress string) bool {
	loopback := net.ParseIP(ipAddress)
	return loopback.IsLoopback() || ipAddress == IPV6_LOOPBACK_ADDRESS
}

func readSysfsFile(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
