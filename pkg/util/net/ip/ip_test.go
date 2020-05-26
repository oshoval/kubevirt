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
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IP utils test", func() {

	Context("IsLoopbackAddress", func() {

		It("should detect IPv4 loopback address", func() {
			result := IsLoopbackAddress(IPv4Loopback)
			Expect(result).To(BeTrue(), "Is Loopback")
		})

		It("should detect IPv4 non loopback address", func() {
			result := IsLoopbackAddress("128.0.0.1")
			Expect(result).To(BeFalse(), "Is not Loopback")
		})

		It("should detect IPv6 loopback address", func() {
			result := IsLoopbackAddress(net.IPv6loopback.String())
			Expect(result).To(BeTrue(), "Is Loopback")
		})

		It("should detect IPv6 non loopback address", func() {
			result := IsLoopbackAddress("fd00:10:244:0:1::e")
			Expect(result).To(BeFalse(), "Is not Loopback")
		})
	})

	Context("NormalizeIPAddress", func() {

		It("should not normalize IPv4 address", func() {
			address := NormalizeIPAddress(IPv4Loopback)
			Expect(address).To(Equal(IPv4Loopback), "address was kept correct")
		})

		It("should normalize non normalized IPv6 address", func() {
			address := NormalizeIPAddress("fd00:10:244:0:1::e")
			Expect(address).To(Equal("[fd00:10:244:0:1::e]"), "address was normalized correct")
		})

		It("should keep normalized IPv6 address", func() {
			address := NormalizeIPAddress("[fd00:10:244:0:1::e]")
			Expect(address).To(Equal("[fd00:10:244:0:1::e]"), "address was kept normalized correct")
		})
	})

	Context("GetIPZeroAddress", func() {

		It("should return IPv4 zero address", func() {
			oneFunc := func(filename string) ([]byte, error) { return []byte{'1'}, nil }
			StubReadFile(oneFunc)

			address := GetIPZeroAddress()
			Expect(address).To(Equal("0.0.0.0"), "address is correct")

			RestoreReadFile()
		})

		It("should return IPv6 zero address", func() {
			zeroFunc := func(filename string) ([]byte, error) { return []byte{'0'}, nil }
			StubReadFile(zeroFunc)

			address := GetIPZeroAddress()
			Expect(address).To(Equal("::"), "address is correct")

			RestoreReadFile()
		})
	})

	Context("GetLoopbackAddress", func() {

		It("should return IPv4 loopback address", func() {
			oneFunc := func(filename string) ([]byte, error) { return []byte{'1'}, nil }
			StubReadFile(oneFunc)

			address := GetLoopbackAddress()
			Expect(address).To(Equal("127.0.0.1"), "address is correct")

			RestoreReadFile()
		})

		It("should return IPv6 zero address", func() {
			zeroFunc := func(filename string) ([]byte, error) { return []byte{'0'}, nil }
			StubReadFile(zeroFunc)

			address := GetLoopbackAddress()
			Expect(address).To(Equal("::1"), "address is correct")

			RestoreReadFile()
		})
	})
})
