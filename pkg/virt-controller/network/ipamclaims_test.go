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

package network

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/api"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/network/namescheme"
	"kubevirt.io/kubevirt/pkg/pointer"

	fakenetworkclient "kubevirt.io/client-go/generated/network-attachment-definition-client/clientset/versioned/fake"

	ipamclaims "github.com/k8snetworkplumbingwg/ipamclaims/pkg/crd/ipamclaims/v1alpha1"
	fakeipamclaimclient "github.com/k8snetworkplumbingwg/ipamclaims/pkg/crd/ipamclaims/v1alpha1/apis/clientset/versioned/fake"
)

const (
	redNetworkName     = "red"
	redNetworkSrcName  = redNetworkName + "-net"
	blueNetworkName    = "blue"
	blueNetworkSrcName = blueNetworkName + "-net"
	namespace          = "test-ns"
	vmiName            = "testvmi"
	vmiUID             = vmiName + "UID"
	networkName        = "network_name"
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
		vmi = api.NewMinimalVMI(vmiName)
		vmi.Namespace = namespace
		vmi.Spec.Domain.Devices.Interfaces = []virtv1.Interface{
			{Name: redNetworkName},
			{Name: "absent", State: virtv1.InterfaceStateAbsent},
		}
		vmi.UID = vmiUID
		vmi.Spec.Networks = []virtv1.Network{
			{
				Name:          redNetworkName,
				NetworkSource: virtv1.NetworkSource{Multus: &virtv1.MultusNetwork{NetworkName: redNetworkSrcName}},
			},
			{
				Name:          "absent",
				NetworkSource: virtv1.NetworkSource{Multus: &virtv1.MultusNetwork{NetworkName: "absent-net"}},
			},
		}
	})

	Context("With allowPersistentIPs enabled in the NADs", func() {
		BeforeEach(func() {
			persistentIPs := map[string]bool{redNetworkSrcName: true}
			err := createNADs(networkClient, namespace, vmi.Spec.Networks, persistentIPs)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the expected IPAMClaims", func() {
			err := CreateIPAMClaims(virtClient, vmi, nil)
			Expect(err).ToNot(HaveOccurred())

			ipamClaimsList, err := virtClient.IPAMClaimsClient().K8sV1alpha1().IPAMClaims(namespace).List(
				context.Background(),
				v1.ListOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipamClaimsList.Items).To(HaveLen(1))
			Expect(ipamClaimsList.Items[0].OwnerReferences).To(HaveLen(1))

			claim := ipamClaimsList.Items[0]
			Expect(claim.OwnerReferences[0]).To(Equal(v1.OwnerReference{
				APIVersion:         "kubevirt.io/v1",
				Kind:               "VirtualMachineInstance",
				Name:               vmiName,
				UID:                vmiUID,
				Controller:         pointer.P(true),
				BlockOwnerDeletion: pointer.P(true),
			}))
			Expect(claim.Name).To(Equal(fmt.Sprintf("%s.%s", vmiName, redNetworkName)))
			Expect(claim.Namespace).To(Equal(namespace))
			Expect(claim.Spec).To(Equal(ipamclaims.IPAMClaimSpec{
				Network:   networkName,
				Interface: namescheme.GenerateHashedInterfaceName(redNetworkName),
			}))
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

	Context("With NADs", func() {
		BeforeEach(func() {
			networks = []virtv1.Network{
				{
					Name:          redNetworkName,
					NetworkSource: virtv1.NetworkSource{Multus: &virtv1.MultusNetwork{NetworkName: redNetworkSrcName}},
				},
				{
					Name:          blueNetworkName,
					NetworkSource: virtv1.NetworkSource{Multus: &virtv1.MultusNetwork{NetworkName: blueNetworkSrcName}},
				},
			}

			persistentIPs := map[string]bool{redNetworkSrcName: true}
			Expect(createNADs(networkClient, namespace, networks, persistentIPs)).ToNot(HaveOccurred())
		})

		It("should return the expected GetNetworkToIPAMClaimParams", func() {
			networkToIPAMClaimParams, err := GetNetworkToIPAMClaimParams(virtClient, namespace, vmiName, networks)
			Expect(err).ToNot(HaveOccurred())
			Expect(networkToIPAMClaimParams).To(Equal(map[string]IPAMClaimParams{
				redNetworkName: {
					claimName:   fmt.Sprintf("%s.%s", vmiName, redNetworkName),
					networkName: networkName,
				}}))
		})
	})
})

func createNADs(networkClient *fakenetworkclient.Clientset, namespace string, networks []virtv1.Network, persistentIPs map[string]bool) error {
	gvr := schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	}
	for _, network := range networks {
		nad := &networkv1.NetworkAttachmentDefinition{
			ObjectMeta: v1.ObjectMeta{
				Name:      network.NetworkSource.Multus.NetworkName,
				Namespace: namespace,
			},
		}

		if persistentIPs[network.NetworkSource.Multus.NetworkName] {
			nad.Spec.Config = fmt.Sprintf(`{"allowPersistentIPs": true, "name": "%s"}`, networkName)
		}
		err := networkClient.Tracker().Create(gvr, nad, namespace)
		if err != nil {
			return err
		}
	}

	return nil
}
