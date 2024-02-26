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
	"strings"

	"kubevirt.io/client-go/precond"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

const MULTUS_RESOURCE_NAME_ANNOTATION = "k8s.v1.cni.cncf.io/resourceName"
const MULTUS_DEFAULT_NETWORK_CNI_ANNOTATION = "v1.multus-cni.io/default-network"

func GetNetworkToResourceMap(virtClient kubecli.KubevirtClient, vmi *v1.VirtualMachineInstance) (networkToResourceMap map[string]string, persistentIPNetworks map[string]bool, err error) {
	networkToResourceMap = make(map[string]string)
	persistentIPNetworks = make(map[string]bool)

	for _, network := range vmi.Spec.Networks {
		if network.Multus != nil {
			namespace, networkName := getNamespaceAndNetworkName(vmi.Namespace, network.Multus.NetworkName)
			crd, err := virtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(namespace).Get(context.Background(), networkName, metav1.GetOptions{})
			if err != nil {
				return map[string]string{}, map[string]bool{}, fmt.Errorf("Failed to locate network attachment definition %s/%s", namespace, networkName)
			}
			networkToResourceMap[network.Name] = getResourceNameForNetwork(crd)
			val, err := getAllowPersistentIPsForNetwork(crd)
			if err != nil {
				return map[string]string{}, map[string]bool{}, err
			}
			persistentIPNetworks[network.Name] = val
		}
	}
	return
}

func getResourceNameForNetwork(network *networkv1.NetworkAttachmentDefinition) string {
	resourceName, ok := network.Annotations[MULTUS_RESOURCE_NAME_ANNOTATION]
	if ok {
		return resourceName
	}
	return "" // meaning the network is not served by resources
}

func getAllowPersistentIPsForNetwork(network *networkv1.NetworkAttachmentDefinition) (bool, error) {
	var configMap map[string]interface{}

	err := json.Unmarshal([]byte(network.Spec.Config), &configMap)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal NAD spec.config JSON: %v", err)
	}

	const allowPersistentIPsKey = "allowPersistentIPs"
	val, ok := configMap[allowPersistentIPsKey]
	if !ok {
		return false, nil
	}

	boolVal, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("value for key %s is not a boolean", allowPersistentIPsKey)
	}

	return boolVal, nil
}

func getNamespaceAndNetworkName(namespace string, fullNetworkName string) (string, string) {
	if strings.Contains(fullNetworkName, "/") {
		res := strings.SplitN(fullNetworkName, "/", 2)
		return res[0], res[1]
	}
	return precond.MustNotBeEmpty(namespace), fullNetworkName
}
