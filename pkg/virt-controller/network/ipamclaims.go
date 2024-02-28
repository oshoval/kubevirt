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
	"k8s.io/apimachinery/pkg/types"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/network/namescheme"
	"kubevirt.io/kubevirt/pkg/network/vmispec"

	"kubevirt.io/client-go/kubecli"

	ipamclaims "github.com/k8snetworkplumbingwg/ipamclaims/pkg/crd/ipamclaims/v1alpha1"
)

type IPAMClaimParams struct {
	claimName   string
	networkName string
}

func GetNetworkToIPAMClaimParams(virtClient kubecli.KubevirtClient, namespace string, vmiName string, networks []virtv1.Network) (map[string]IPAMClaimParams, error) {
	networkToIPAMClaimName := make(map[string]IPAMClaimParams)
	for _, network := range vmispec.FilterMultusNonDefaultNetworks(networks) {
		persistentIPsNetworkName, err := getPersistentIPsNetworkName(virtClient, namespace, network.Multus.NetworkName)
		if err != nil {
			return map[string]IPAMClaimParams{}, fmt.Errorf("failed retrieving persistentIPsNetworkName: %w", err)
		}
		if persistentIPsNetworkName != "" {
			networkToIPAMClaimName[network.Name] = IPAMClaimParams{
				claimName:   fmt.Sprintf("%s.%s", vmiName, network.Name),
				networkName: persistentIPsNetworkName,
			}
		}
	}

	return networkToIPAMClaimName, nil
}

func getPersistentIPsNetworkName(virtClient kubecli.KubevirtClient, namespace string, fullNetworkName string) (string, error) {
	namespace, networkName := getNamespaceAndNetworkName(namespace, fullNetworkName)
	nad, err := virtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(namespace).Get(context.Background(), networkName, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to locate network attachment definition %s/%s", namespace, networkName)
	}

	if nad.Spec.Config == "" {
		return "", nil
	}

	netConf := struct {
		AllowPersistentIPs bool   `json:"allowPersistentIPs,omitempty"`
		Name               string `json:"name,omitempty"`
	}{}
	err = json.Unmarshal([]byte(nad.Spec.Config), &netConf)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal NAD spec.config JSON: %v", err)
	}

	if !netConf.AllowPersistentIPs {
		return "", nil
	}

	if netConf.Name == "" {
		return "", fmt.Errorf("failed to obtain network name: cannot be empty when allowPersistentIPs is true")
	}

	return netConf.Name, nil
}

func CreateIPAMClaims(client kubecli.KubevirtClient, vmi *virtv1.VirtualMachineInstance, ownerRef *v1.OwnerReference) error {
	nonAbsentNetworks := filterNonAbsentNetworks(vmi.Spec.Domain.Devices.Interfaces, vmi.Spec.Networks)
	networkToIPAMClaimParams, err := GetNetworkToIPAMClaimParams(client, vmi.Namespace, vmi.Name, nonAbsentNetworks)
	if err != nil {
		return fmt.Errorf("failed composing networkToIPAMClaimName: %w", err)
	}

	// For a standalone VMI, the IPAMClaim owner is the VMI itself
	if ownerRef == nil {
		ownerRef = v1.NewControllerRef(vmi, virtv1.VirtualMachineInstanceGroupVersionKind)
	}

	claims := composeIPAMClaims(vmi.Namespace, ownerRef, networkToIPAMClaimParams)
	err = createIPAMClaims(client, vmi.Namespace, claims)
	if err != nil {
		return fmt.Errorf("failed IPAMClaims creation for VMI %s: %w", vmi.Name, err)
	}

	return nil
}

func composeIPAMClaims(namespace string, ownerRef *v1.OwnerReference, networkToIPAMClaimParams map[string]IPAMClaimParams) []*ipamclaims.IPAMClaim {
	claims := []*ipamclaims.IPAMClaim{}
	for netName, ipamClaimParams := range networkToIPAMClaimParams {
		claims = append(claims, composeIPAMClaim(
			namespace,
			*ownerRef,
			ipamClaimParams,
			namescheme.GenerateHashedInterfaceName(netName),
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
				return fmt.Errorf("failed to create IPAMClaim: %w", err)
			}

			err = validateIPAMClaim(client, namespace, claim.Name, claim.OwnerReferences[0].UID)
			if err != nil {
				return fmt.Errorf("failed validating IPAMClaim: %w", err)
			}
		}
	}

	return nil
}

func composeIPAMClaim(namespace string, ownerRef v1.OwnerReference, ipamClaimParams IPAMClaimParams, interfaceName string) *ipamclaims.IPAMClaim {
	return &ipamclaims.IPAMClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      ipamClaimParams.claimName,
			Namespace: namespace,
			OwnerReferences: []v1.OwnerReference{
				ownerRef,
			},
		},
		Spec: ipamclaims.IPAMClaimSpec{
			Network:   ipamClaimParams.networkName,
			Interface: interfaceName,
		},
	}
}

func filterNonAbsentNetworks(interfaces []virtv1.Interface, networks []virtv1.Network) []virtv1.Network {
	nonAbsentIfaces := vmispec.FilterInterfacesSpec(interfaces, func(iface virtv1.Interface) bool {
		return iface.State != virtv1.InterfaceStateAbsent
	})
	nonAbsentNetworks := vmispec.FilterNetworksByInterfaces(networks, nonAbsentIfaces)

	return nonAbsentNetworks
}

func validateIPAMClaim(client kubecli.KubevirtClient, namespace string, claimName string, expectedOwnerUID types.UID) error {
	currentClaim, err := client.IPAMClaimsClient().K8sV1alpha1().IPAMClaims(namespace).Get(
		context.Background(),
		claimName,
		v1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed getting IPAMClaim: %w", err)
	}

	// We need to make sure that the existing IPAMClaim belong to current VMI,
	// and it isn't a lefotver of freshly deleted VM/VMI with the same namespace/name
	if len(currentClaim.OwnerReferences) != 1 || currentClaim.OwnerReferences[0].UID != expectedOwnerUID {
		return fmt.Errorf("failed validating IPAMClaim, wrong IPAMClaim with the same name still exists")
	}

	return nil
}
