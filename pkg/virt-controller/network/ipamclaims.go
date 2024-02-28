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
	"encoding/json"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/network/namescheme"
	"kubevirt.io/kubevirt/pkg/network/vmispec"

	"kubevirt.io/client-go/kubecli"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	ipamclaims "github.com/k8snetworkplumbingwg/ipamclaims/pkg/crd/ipamclaims/v1alpha1"
)

type NetConf struct {
	AllowPersistentIPs bool `json:"allowPersistentIPs,omitempty"`
}

func GetNetworkToIPAMClaimName(virtClient kubecli.KubevirtClient, vmi *virtv1.VirtualMachineInstance, persistentIPsEnabled bool) (map[string]string, error) {
	networkToIPAMClaimName := make(map[string]string)
	networkProcessed := make(map[string]bool)
	for _, network := range vmi.Spec.Networks {
		if !networkProcessed[network.Name] && vmispec.IsSecondaryMultusNetwork(network) {
			allowPersistentIPs, err := getAllowPersistentIPs(virtClient, vmi.Namespace, network.Multus.NetworkName)
			if err != nil {
				return map[string]string{}, fmt.Errorf("GetNetworkToIPAMClaimName failed: %w", err)
			}

			if allowPersistentIPs {
				networkToIPAMClaimName[network.Name] = fmt.Sprintf("%s.%s", vmi.Name, network.Name)
			}
		}
		networkProcessed[network.Name] = true
	}

	if !persistentIPsEnabled && len(networkToIPAMClaimName) != 0 {
		return nil, fmt.Errorf("persistenIPs is requested but PersistentIPsGate is disabled")
	}

	return networkToIPAMClaimName, nil
}

func getAllowPersistentIPs(virtClient kubecli.KubevirtClient, namespace string, fullNetworkName string) (bool, error) {
	namespace, networkName := getNamespaceAndNetworkName(namespace, fullNetworkName)
	nad, err := virtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(namespace).Get(context.Background(), networkName, v1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("Failed to locate network attachment definition %s/%s", namespace, networkName)
	}

	allowPersistentIPs, err := getAllowPersistentIPsFromNAD(nad)
	if err != nil {
		return false, fmt.Errorf("getAllowPersistentIPs failed: %w", err)
	}

	return allowPersistentIPs, nil
}

func getAllowPersistentIPsFromNAD(network *networkv1.NetworkAttachmentDefinition) (bool, error) {
	if network.Spec.Config == "" {
		return false, nil
	}

	var netConf NetConf
	err := json.Unmarshal([]byte(network.Spec.Config), &netConf)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal NAD spec.config JSON: %v", err)
	}

	return netConf.AllowPersistentIPs, nil
}

func CreateIPAMClaims(client kubecli.KubevirtClient, vmi *virtv1.VirtualMachineInstance, ownerRef *v1.OwnerReference, persistentIPsEnabled bool) error {
	networkToIPAMClaimName, err := GetNetworkToIPAMClaimName(client, vmi, persistentIPsEnabled)
	if err != nil {
		return fmt.Errorf("GetNetworkToIPAMClaimName failed: %w", err)
	}

	persistentIPnetworks := getPersistentIPnetworks(client, vmi, networkToIPAMClaimName, persistentIPsEnabled)
	if ownerRef == nil {
		ownerRef = v1.NewControllerRef(vmi, virtv1.VirtualMachineInstanceGroupVersionKind)
	}
	claims := composeIPAMClaims(vmi.Namespace, ownerRef, persistentIPnetworks, networkToIPAMClaimName)
	err = createIPAMClaims(client, vmi.Namespace, claims)
	if err != nil {
		return fmt.Errorf("createIPAMClaims failed for VMI %s: %w", vmi.Name, err)
	}

	return nil
}

func getPersistentIPnetworks(client kubecli.KubevirtClient, vmi *virtv1.VirtualMachineInstance, networkToIPAMClaimName map[string]string, persistentIPsEnabled bool) []virtv1.Network {
	nonAbsentIfaces := vmispec.FilterInterfacesSpec(vmi.Spec.Domain.Devices.Interfaces, func(iface virtv1.Interface) bool {
		return iface.State != virtv1.InterfaceStateAbsent
	})
	nonAbsentNets := vmispec.FilterNetworksByInterfaces(vmi.Spec.Networks, nonAbsentIfaces)

	persistentIPnetworks := vmispec.FilterNetworksSpec(nonAbsentNets, func(network virtv1.Network) bool {
		return vmispec.IsSecondaryMultusNetwork(network) && networkToIPAMClaimName[network.Name] != ""
	})

	return persistentIPnetworks
}

func composeIPAMClaims(namespace string, ownerRef *v1.OwnerReference, persistentIPnetworks []virtv1.Network, networkToIPAMClaimName map[string]string) []*ipamclaims.IPAMClaim {
	claims := []*ipamclaims.IPAMClaim{}
	networkNameScheme := namescheme.CreateHashedNetworkNameScheme(persistentIPnetworks)
	for _, network := range persistentIPnetworks {
		claims = append(claims, composeIPAMClaim(
			namespace,
			*ownerRef,
			networkToIPAMClaimName[network.Name],
			network,
			networkNameScheme[network.Name],
		))
	}

	return claims
}

func createIPAMClaims(client kubecli.KubevirtClient, namespace string, claims []*ipamclaims.IPAMClaim) error {
	for _, claim := range claims {
		_, err := client.IPAMClaimsClient().K8sV1alpha1().IPAMClaims(namespace).Create(
			context.Background(),
			claim,
			v1.CreateOptions{},
		)

		if err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				return fmt.Errorf("IPAMClaim create failed: %w", err)
			}

			currentClaim, err := client.IPAMClaimsClient().K8sV1alpha1().IPAMClaims(namespace).Get(
				context.Background(),
				claim.Name,
				v1.GetOptions{})
			if err != nil {
				return fmt.Errorf("IPAMClaim validation failed: %w", err)
			}

			// We need to make sure that the existing IPAMClaim belong to current VMI
			// and it isn't a lefotver of freshly deleted VMI with the same namespace/name
			if len(currentClaim.OwnerReferences) != 1 || currentClaim.OwnerReferences[0].UID != claim.OwnerReferences[0].UID {
				return fmt.Errorf("wrong IPAMClaim exists")
			}
		}
	}

	return nil
}

func composeIPAMClaim(namespace string, ownerRef v1.OwnerReference, claimName string, network virtv1.Network, interfaceName string) *ipamclaims.IPAMClaim {
	return &ipamclaims.IPAMClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      claimName,
			Namespace: namespace,
			OwnerReferences: []v1.OwnerReference{
				ownerRef,
			},
		},
		Spec: ipamclaims.IPAMClaimSpec{
			Network:   network.Multus.NetworkName,
			Interface: interfaceName,
		},
	}
}
