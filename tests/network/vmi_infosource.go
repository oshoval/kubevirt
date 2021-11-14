/*
 * This file is part of the kubevirt project
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
 * Copyright 2021 Red Hat, Inc.
 *
 */

package network

import (
	"context"
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kvirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/client-go/kubecli"
	netvmispec "kubevirt.io/kubevirt/pkg/network/vmispec"
	"kubevirt.io/kubevirt/tests"
	"kubevirt.io/kubevirt/tests/libvmi"
	"kubevirt.io/kubevirt/tests/util"
)

var _ = SIGDescribe("Infosource", func() {
	var virtClient kubecli.KubevirtClient

	BeforeEach(func() {
		var err error
		virtClient, err = kubecli.GetKubevirtClient()
		Expect(err).NotTo(HaveOccurred(), "Should successfully initialize an API client")

		tests.BeforeTestCleanup()
	})

	Context("with a vmi and a few interfaces", func() {
		var vmi *kvirtv1.VirtualMachineInstance

		const defaultInterfaceName = "default"
		const originalMac = "02:00:05:05:05:05"
		const updatedMac = "02:00:b5:b5:b5:b5"
		const dummyInterfaceMac = "02:00:b0:b0:b0:b0"
		const secondaryNetworkName = "infosrc"

		linuxBridgeNetwork1 := kvirtv1.Network{
			Name: "bridge1-unchanged",
			NetworkSource: kvirtv1.NetworkSource{
				Multus: &kvirtv1.MultusNetwork{
					NetworkName: secondaryNetworkName,
				},
			},
		}

		linuxBridgeNetwork2 := kvirtv1.Network{
			Name: "bridge2-setns",
			NetworkSource: kvirtv1.NetworkSource{
				Multus: &kvirtv1.MultusNetwork{
					NetworkName: secondaryNetworkName,
				},
			},
		}

		secondaryLinuxBridgeInterface1 := kvirtv1.Interface{
			Name: linuxBridgeNetwork1.Name,
			InterfaceBindingMethod: kvirtv1.InterfaceBindingMethod{
				Bridge: &kvirtv1.InterfaceBridge{},
			},
			MacAddress: "02:00:a0:a0:a0:a0",
		}

		secondaryLinuxBridgeInterface2 := kvirtv1.Interface{
			Name: linuxBridgeNetwork2.Name,
			InterfaceBindingMethod: kvirtv1.InterfaceBindingMethod{
				Bridge: &kvirtv1.InterfaceBridge{},
			},
			MacAddress: "02:00:a1:a1:a1:a1",
		}

		BeforeEach(func() {
			By("Create NetworkAttachmentDefinition")
			nad := newNetworkAttachmentDefinition(secondaryNetworkName)
			_, err := virtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(util.NamespaceTestDefault).Create(context.TODO(), nad, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			changeMacCmd := "sudo ip link set dev eth0 address " + updatedMac + "\n"
			createDummyInterfaceCmd := "ip link add dummy0 type dummy\n" +
				"ip link set dev dummy0 address " + dummyInterfaceMac + "\n"
			setnsCmd := "ip netns add testns\n" +
				"ip link set eth2 netns testns\n"

			defaultBridgeInterface := libvmi.InterfaceDeviceWithBridgeBinding(defaultInterfaceName)
			vmiSpec := libvmi.NewFedora(
				libvmi.WithInterface(*libvmi.InterfaceWithMac(&defaultBridgeInterface, originalMac)),
				libvmi.WithNetwork(kvirtv1.DefaultPodNetwork()),
				libvmi.WithInterface(secondaryLinuxBridgeInterface1),
				libvmi.WithInterface(secondaryLinuxBridgeInterface2),
				libvmi.WithNetwork(&linuxBridgeNetwork1),
				libvmi.WithNetwork(&linuxBridgeNetwork2),
				libvmi.WithCloudInitNoCloudUserData(
					"#!/bin/bash\n"+changeMacCmd+
						createDummyInterfaceCmd+
						setnsCmd,
					false))

			vmi, err = virtClient.VirtualMachineInstance(util.NamespaceTestDefault).Create(vmiSpec)
			Expect(err).NotTo(HaveOccurred())
			tests.WaitForSuccessfulVMIStart(vmi)
			tests.WaitAgentConnected(virtClient, vmi)
		})

		It("should have the expected entries in vmi status", func() {
			expectedInterfaces := []kvirtv1.VirtualMachineInstanceNetworkInterface{
				{
					InfoSource: netvmispec.InfoSourceDomain,
					MAC:        originalMac,
					Name:       defaultInterfaceName,
				},
				{
					InfoSource:    netvmispec.InfoSourceDomainAndGA,
					InterfaceName: "eth1",
					MAC:           secondaryLinuxBridgeInterface1.MacAddress,
					Name:          secondaryLinuxBridgeInterface1.Name,
				},
				{
					InfoSource: netvmispec.InfoSourceDomain,
					MAC:        secondaryLinuxBridgeInterface2.MacAddress,
					Name:       secondaryLinuxBridgeInterface2.Name,
				},
				{
					InfoSource:    netvmispec.InfoSourceGuestAgent,
					InterfaceName: "eth0",
					MAC:           updatedMac,
				},
				{
					InfoSource:    netvmispec.InfoSourceGuestAgent,
					InterfaceName: "dummy0",
					MAC:           dummyInterfaceMac,
				},
			}

			var vmiObj *kvirtv1.VirtualMachineInstance
			Eventually(func() bool {
				var err error
				vmiObj, err = virtClient.VirtualMachineInstance(vmi.Namespace).Get(vmi.Name, &metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				return dummyInterfaceExists(vmiObj)
			}, 140*time.Second, 2*time.Second).Should(Equal(true))

			Expect(len(vmiObj.Status.Interfaces)).To(Equal(len(expectedInterfaces)))

			canonizeInterfaceStatus(vmiObj)
			for i := range vmiObj.Status.Interfaces {
				expectedInterfaces[i].IP = vmiObj.Status.Interfaces[i].IP
				expectedInterfaces[i].IPs = vmiObj.Status.Interfaces[i].IPs
			}

			Expect(reflect.DeepEqual(expectedInterfaces, vmiObj.Status.Interfaces)).To(BeTrue(),
				fmt.Sprintf("interfaces doesn't match\n%v\n%v", expectedInterfaces, vmiObj.Status.Interfaces))
		})
	})
})

func newNetworkAttachmentDefinition(networkName string) *nadv1.NetworkAttachmentDefinition {
	config := fmt.Sprintf(`{"cniVersion": "0.3.1", "name": "%s", "type": "cnv-bridge", "bridge": "%s"}`, networkName, networkName)
	return &nadv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: networkName,
		},
		Spec: nadv1.NetworkAttachmentDefinitionSpec{Config: config},
	}
}

// dummyInterfaceExists waits for least the dummy interface to be reported by guest agent,
// which means there was a guest-agent report finally, and then we can compare the rest expected info.
func dummyInterfaceExists(vmi *kvirtv1.VirtualMachineInstance) bool {
	for i := range vmi.Status.Interfaces {
		if vmi.Status.Interfaces[i].InterfaceName == "dummy0" {
			return true
		}
	}
	return false
}

// canonizeInterfaceStatus orders "guest-agent" only records in predictable order
func canonizeInterfaceStatus(vmi *kvirtv1.VirtualMachineInstance) {
	length := len(vmi.Status.Interfaces)
	if vmi.Status.Interfaces[length-1].InterfaceName != "dummy0" {
		vmi.Status.Interfaces[length-1], vmi.Status.Interfaces[length-2] =
			vmi.Status.Interfaces[length-2], vmi.Status.Interfaces[length-1]
	}
}
