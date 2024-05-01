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
 * Copyright 2024 Red Hat, Inc.
 *
 */

package network_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/libvmi"
	"kubevirt.io/kubevirt/pkg/pointer"
	"kubevirt.io/kubevirt/pkg/virt-controller/network"

	fakenetworkclient "kubevirt.io/client-go/generated/network-attachment-definition-client/clientset/versioned/fake"

	ipamclaims "github.com/k8snetworkplumbingwg/ipamclaims/pkg/crd/ipamclaims/v1alpha1"
	fakeipamclaimclient "github.com/k8snetworkplumbingwg/ipamclaims/pkg/crd/ipamclaims/v1alpha1/apis/clientset/versioned/fake"
)

const (
	vmiName        = "testvmi"
	vmiUID         = vmiName + "UID"
	nadNetworkName = "nad_network_name"
)

var _ = Describe("CreateIPAMClaims", func() {
	var virtClient *kubecli.MockKubevirtClient
	var networkClient *fakenetworkclient.Clientset
	var vmi *virtv1.VirtualMachineInstance

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		virtClient = kubecli.NewMockKubevirtClient(ctrl)

		networkClient = fakenetworkclient.NewSimpleClientset()
		virtClient.EXPECT().NetworkClient().Return(networkClient).AnyTimes()
		ipamClaimsClient := fakeipamclaimclient.NewSimpleClientset()
		virtClient.EXPECT().IPAMClaimsClient().Return(ipamClaimsClient).AnyTimes()
	})

	BeforeEach(func() {
		vmi = libvmi.New(
			libvmi.WithName(vmiName),
			libvmi.WithNamespace(redNamespace),
			libvmi.WithNetwork(virtv1.DefaultPodNetwork()),
			libvmi.WithNetwork(libvmi.MultusNetwork(redNetworkLogicalName, redNetworkNadName)),
			libvmi.WithNetwork(libvmi.MultusNetwork(blueNetworkLogicalName, blueNetworkNadName)),
			libvmi.WithNetwork(libvmi.MultusNetwork("absent", "absent-net")),
			libvmi.WithInterface(virtv1.Interface{Name: "default"}),
			libvmi.WithInterface(virtv1.Interface{Name: redNetworkLogicalName}),
			libvmi.WithInterface(virtv1.Interface{Name: blueNetworkLogicalName}),
			libvmi.WithInterface(virtv1.Interface{Name: "absent", State: virtv1.InterfaceStateAbsent}),
		)
		vmi.UID = vmiUID
	})

	Context("With allowPersistentIPs enabled in the NADs", func() {
		var ipamClaimsManager *network.IPAMClaimsManager

		BeforeEach(func() {
			persistentIPs := map[string]struct{}{redNetworkNadName: {}, blueNetworkNadName: {}}
			Expect(createNADs(networkClient, redNamespace, vmi.Spec.Networks, persistentIPs)).To(Succeed())

			ipamClaimsManager = network.NewIPAMClaimsManager(virtClient)
		})

		It("should create the expected IPAMClaims", func() {
			ownerRef := &v1.OwnerReference{
				APIVersion:         "kubevirt.io/v1",
				Kind:               "VirtualMachineInstance",
				Name:               vmiName,
				UID:                vmiUID,
				Controller:         pointer.P(true),
				BlockOwnerDeletion: pointer.P(true),
			}
			Expect(ipamClaimsManager.CreateIPAMClaims(vmi.Namespace, vmi.Name, vmi.Spec.Domain.Devices.Interfaces, vmi.Spec.Networks, ownerRef)).To(Succeed())

			ipamClaimsList, err := virtClient.IPAMClaimsClient().K8sV1alpha1().IPAMClaims(redNamespace).List(
				context.Background(),
				v1.ListOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipamClaimsList.Items).To(HaveLen(2))
			assertIPAMClaim(ipamClaimsList.Items[0], blueNetworkLogicalName, "pod16477688c0e")
			assertIPAMClaim(ipamClaimsList.Items[1], redNetworkLogicalName, "podb1f51a511f1")
		})

		Context("When IPAMClaims already exist", func() {
			var ownerRef *v1.OwnerReference

			BeforeEach(func() {
				ownerRef = &v1.OwnerReference{
					APIVersion:         "kubevirt.io/v1",
					Kind:               "VirtualMachineInstance",
					Name:               vmiName,
					UID:                vmiUID,
					Controller:         pointer.P(true),
					BlockOwnerDeletion: pointer.P(true),
				}
				Expect(ipamClaimsManager.CreateIPAMClaims(vmi.Namespace,
					vmi.Name,
					vmi.Spec.Domain.Devices.Interfaces,
					vmi.Spec.Networks,
					ownerRef)).To(Succeed())
			})

			It("with the expected owner UID, should not fail re-creation/validation attempt", func() {
				Expect(ipamClaimsManager.CreateIPAMClaims(
					vmi.Namespace,
					vmi.Name,
					vmi.Spec.Domain.Devices.Interfaces,
					vmi.Spec.Networks,
					ownerRef)).To(Succeed())
			})

			It("with a different owner UID, should fail fail re-creation/validation attempt", func() {
				ownerRef.UID = "differentUID"
				err := ipamClaimsManager.CreateIPAMClaims(vmi.Namespace,
					vmi.Name,
					vmi.Spec.Domain.Devices.Interfaces,
					vmi.Spec.Networks,
					ownerRef)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("wrong IPAMClaim with the same name still exists"))
			})
		})
	})

	Context("With allowPersistentIPs disabled in the NADs", func() {
		BeforeEach(func() {
			Expect(createNADs(networkClient, redNamespace, vmi.Spec.Networks, map[string]struct{}{})).To(Succeed())
		})

		It("should not create IPAMClaims", func() {
			ownerRef := &v1.OwnerReference{
				APIVersion:         "kubevirt.io/v1",
				Kind:               "VirtualMachineInstance",
				Name:               vmiName,
				UID:                vmiUID,
				Controller:         pointer.P(true),
				BlockOwnerDeletion: pointer.P(true),
			}
			ipamClaimsManager := network.NewIPAMClaimsManager(virtClient)
			Expect(ipamClaimsManager.CreateIPAMClaims(vmi.Namespace, vmi.Name, vmi.Spec.Domain.Devices.Interfaces, vmi.Spec.Networks, ownerRef)).To(Succeed())

			ipamClaimsList, err := virtClient.IPAMClaimsClient().K8sV1alpha1().IPAMClaims(redNamespace).List(
				context.Background(),
				v1.ListOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipamClaimsList.Items).To(HaveLen(0))
		})
	})
})

var _ = Describe("GetNetworkToIPAMClaimParams", func() {
	var virtClient *kubecli.MockKubevirtClient
	var networkClient *fakenetworkclient.Clientset
	var networks []virtv1.Network

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		virtClient = kubecli.NewMockKubevirtClient(ctrl)

		networkClient = fakenetworkclient.NewSimpleClientset()
		virtClient.EXPECT().NetworkClient().Return(networkClient).AnyTimes()
	})

	BeforeEach(func() {
		networks = []virtv1.Network{
			*libvmi.MultusNetwork(redNetworkLogicalName, redNetworkNadName),
			*libvmi.MultusNetwork(blueNetworkLogicalName, blueNetworkNadName),
			*virtv1.DefaultPodNetwork(),
			{Name: "default_multus", NetworkSource: virtv1.NetworkSource{Multus: &virtv1.MultusNetwork{NetworkName: "default_multus_net", Default: true}}},
		}

		persistentIPs := map[string]struct{}{redNetworkNadName: {}}
		Expect(createNADs(networkClient, redNamespace, networks, persistentIPs)).To(Succeed())
	})

	It("should return the expected IPAMClaim parameters", func() {
		ipamClaimsManager := network.NewIPAMClaimsManager(virtClient)
		networkToIPAMClaimParams, err := ipamClaimsManager.GetNetworkToIPAMClaimParams(redNamespace, vmiName, networks)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkToIPAMClaimParams).To(Equal(map[string]network.IPAMClaimParams{
			redNetworkLogicalName: {
				ClaimName:   fmt.Sprintf("%s.%s", vmiName, redNetworkLogicalName),
				NetworkName: nadNetworkName,
			}}))
	})
})

var _ = Describe("ExtractNetworkToIPAMClaimParams", func() {
	It("should successfully extract expected network to IPAM claim params", func() {
		nadMap := map[string]*networkv1.NetworkAttachmentDefinition{
			blueNetworkLogicalName: {
				Spec: networkv1.NetworkAttachmentDefinitionSpec{
					Config: fmt.Sprintf(`{"name": "%s"}`, nadNetworkName),
				},
			},
			redNetworkLogicalName: {
				Spec: networkv1.NetworkAttachmentDefinitionSpec{
					Config: fmt.Sprintf(`{"allowPersistentIPs": true, "name": "%s"}`, nadNetworkName),
				},
			},
		}

		expected := map[string]network.IPAMClaimParams{
			redNetworkLogicalName: {
				ClaimName:   fmt.Sprintf("%s.%s", vmiName, redNetworkLogicalName),
				NetworkName: "nad_network_name",
			},
		}

		networkToIPAMClaimParams, err := network.ExtractNetworkToIPAMClaimParams(nadMap, vmiName)
		Expect(err).ToNot(HaveOccurred())
		Expect(networkToIPAMClaimParams).To(Equal(expected))
	})

	It("should fail when nad is misconfigured", func() {
		nadMap := map[string]*networkv1.NetworkAttachmentDefinition{
			redNetworkLogicalName: {
				Spec: networkv1.NetworkAttachmentDefinitionSpec{
					Config: `{"allowPersistentIPs": true}`,
				},
			},
		}
		_, err := network.ExtractNetworkToIPAMClaimParams(nadMap, vmiName)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("failed retrieving persistentIPsNetworkName: failed to obtain network name: missing required field"))
	})
})

func assertIPAMClaim(claim ipamclaims.IPAMClaim, logicalName, interfaceName string) {
	ExpectWithOffset(1, claim.OwnerReferences).To(ConsistOf(v1.OwnerReference{
		APIVersion:         "kubevirt.io/v1",
		Kind:               "VirtualMachineInstance",
		Name:               vmiName,
		UID:                vmiUID,
		Controller:         pointer.P(true),
		BlockOwnerDeletion: pointer.P(true),
	}))
	ExpectWithOffset(1, claim.Name).To(Equal(fmt.Sprintf("%s.%s", vmiName, logicalName)))
	ExpectWithOffset(1, claim.Namespace).To(Equal(redNamespace))
	ExpectWithOffset(1, claim.Spec).To(Equal(ipamclaims.IPAMClaimSpec{
		Network:   "nad_network_name",
		Interface: interfaceName,
	}))
}
