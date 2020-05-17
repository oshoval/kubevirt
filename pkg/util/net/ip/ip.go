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
)

const (
	// sysfs
	disableIPv6Path = "/proc/sys/net/ipv6/conf/default/disable_ipv6"

	// Special IPs
	IPV4_BIND_ADDRESS     = "0.0.0.0" //INADDR_ANY
	IPV4_LOOPBACK_ADDRESS = "127.0.0.1"
	IPV6_BIND_ADDRESS     = "[::]" //INADDR6_ANY
	IPV6_LOOPBACK_ADDRESS = "[::1]"
)

// GetIPZeroAddress returns INADDR_ANY or INADDR6_ANY (according sysctl disable_ipv6)
func GetIPZeroAddress() string {
	if isIPv6Disabled(disableIPv6Path) {
		return IPV4_BIND_ADDRESS
	}

	return IPV6_BIND_ADDRESS
}

// GetLoopbackAddress returns loopback IP (either 127.0.0.1 or [::1] according sysctl disable_ipv6)
func GetLoopbackAddress() string {
	if isIPv6Disabled(disableIPv6Path) {
		return IPV4_LOOPBACK_ADDRESS
	}

	return IPV6_LOOPBACK_ADDRESS
}

// IsLoopbackAddress checks if the address is IPv4 / IPv6 loopback address
func IsLoopbackAddress(ipAddress string) bool {
	loopback := net.ParseIP(ipAddress)
	return loopback.IsLoopback() || ipAddress == IPV6_LOOPBACK_ADDRESS
}

func isIPv6Disabled(path string) bool {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return true
	}

	disableIPv6 := strings.TrimSuffix(string(data), "\n")
	return disableIPv6 == "1"
}
