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
 * Copyright 2021 Red Hat, Inc.
 *
 */

package network

import (
	"encoding/json"
	"fmt"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	k8sv1 "k8s.io/api/core/v1"

	v1 "kubevirt.io/api/core/v1"
	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/network/namescheme"
	"kubevirt.io/kubevirt/pkg/network/netbinding"
	"kubevirt.io/kubevirt/pkg/network/vmispec"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"

	ipct "kubevirt.io/kubevirt/pkg/virt-controller/ipamclaims/types"
)

type MultusNetworkAnnotationPool struct {
	pool []networkv1.NetworkSelectionElement
}

// OR-TODO revert unneeded
func (mnap *MultusNetworkAnnotationPool) Add(multusNetworkAnnotation networkv1.NetworkSelectionElement) {
	mnap.pool = append(mnap.pool, multusNetworkAnnotation)
}

func (mnap MultusNetworkAnnotationPool) IsEmpty() bool {
	return len(mnap.pool) == 0
}

func (mnap MultusNetworkAnnotationPool) ToString() (string, error) {
	multusNetworksAnnotation, err := json.Marshal(mnap.pool)
	if err != nil {
		return "", fmt.Errorf("failed to create JSON list from multus interface pool %v", mnap.pool)
	}
	return string(multusNetworksAnnotation), nil
}

func (mnap MultusNetworkAnnotationPool) FindMultusAnnotation(networkName string) (int, error) {
	for i, element := range mnap.pool {
		if element.InterfaceRequest != "" && element.Name == networkName {
			return i, nil
		}
	}
	return 0, fmt.Errorf("failed finding annotation for '%s'", networkName)
}

type Option func(MultusNetworkAnnotationPool, string, []v1.Network) MultusNetworkAnnotationPool

// OR-TODO move
func WithIPAMClaimRef(networkToIPAMClaimParams map[string]ipct.IPAMClaimParams) Option {
	return func(mnap MultusNetworkAnnotationPool, namespace string, networks []v1.Network) MultusNetworkAnnotationPool {
		for _, network := range networks {
			if !vmispec.IsSecondaryMultusNetwork(network) {
				continue
			}

			_, networkName := GetNamespaceAndNetworkName(namespace, network.Multus.NetworkName)
			i, err := mnap.FindMultusAnnotation(networkName)
			if err != nil {
				continue // TODO handle error if needed
			}
			mnap.pool[i].IPAMClaimReference = networkToIPAMClaimParams[network.Name].ClaimName
		}
		return mnap
	}
}

// TODO move / keep
func AmendMultusCNIAnnotation(multusAnnotation string, namespace string, interfaces []virtv1.Interface, networks []virtv1.Network, networkNameScheme map[string]string, networkToIPAMClaimParams map[string]ipct.IPAMClaimParams) (string, error) {
	if multusAnnotation == "" {
		return "", nil
	}

	multusNetworkAnnotationPool := MultusNetworkAnnotationPool{}
	if err := json.Unmarshal([]byte(multusAnnotation), &multusNetworkAnnotationPool.pool); err != nil {
		return "", err
	}

	// TODO
	// for _, network := range networks {
	// 	if vmispec.IsSecondaryMultusNetwork(network) {
	// 		//podInterfaceName := networkNameScheme[network.Name]
	// 		// find and change
	// 	}
	// }

	return multusNetworkAnnotationPool.ToString()
}

func GenerateMultusCNIAnnotation(namespace string, interfaces []v1.Interface, networks []v1.Network, config *virtconfig.ClusterConfig, options ...Option) (string, error) {
	return GenerateMultusCNIAnnotationFromNameScheme(namespace, interfaces, networks, namescheme.CreateHashedNetworkNameScheme(networks), config, options...)
}

func GenerateMultusCNIAnnotationFromNameScheme(namespace string, interfaces []v1.Interface, networks []v1.Network, networkNameScheme map[string]string, config *virtconfig.ClusterConfig, options ...Option) (string, error) {
	multusNetworkAnnotationPool := MultusNetworkAnnotationPool{}

	for _, network := range networks {
		if vmispec.IsSecondaryMultusNetwork(network) {
			podInterfaceName := networkNameScheme[network.Name]
			multusNetworkAnnotationPool.Add(
				newMultusAnnotationData(namespace, interfaces, network, podInterfaceName))
		}

		if config != nil && config.NetworkBindingPlugingsEnabled() {
			if iface := vmispec.LookupInterfaceByName(interfaces, network.Name); iface.Binding != nil {
				bindingPluginAnnotationData, err := newBindingPluginMultusAnnotationData(
					config.GetConfig(), iface.Binding.Name, namespace, network.Name)
				if err != nil {
					return "", err
				}
				if bindingPluginAnnotationData != nil {
					multusNetworkAnnotationPool.Add(*bindingPluginAnnotationData)
				}
			}
		}
	}

	for _, option := range options {
		multusNetworkAnnotationPool = option(multusNetworkAnnotationPool, namespace, networks)
	}

	if !multusNetworkAnnotationPool.IsEmpty() {
		return multusNetworkAnnotationPool.ToString()
	}
	return "", nil
}

func newBindingPluginMultusAnnotationData(kvConfig *v1.KubeVirtConfiguration, pluginName, namespace, networkName string) (*networkv1.NetworkSelectionElement, error) {
	plugin := netbinding.ReadNetBindingPluginConfiguration(kvConfig, pluginName)
	if plugin == nil {
		return nil, fmt.Errorf("unable to find the network binding plugin '%s' in Kubevirt configuration", pluginName)
	}

	if plugin.NetworkAttachmentDefinition == "" {
		return nil, nil
	}
	netAttachDefNamespace, netAttachDefName := GetNamespaceAndNetworkName(namespace, plugin.NetworkAttachmentDefinition)

	// cniArgNetworkName is the CNI arg name for the VM spec network logical name.
	// The binding plugin CNI should read this arg and realize which logical network it should modify.
	const cniArgNetworkName = "logicNetworkName"

	return &networkv1.NetworkSelectionElement{
		Namespace: netAttachDefNamespace,
		Name:      netAttachDefName,
		CNIArgs: &map[string]interface{}{
			cniArgNetworkName: networkName,
		},
	}, nil
}

func newMultusAnnotationData(namespace string, interfaces []v1.Interface, network v1.Network, podInterfaceName string) networkv1.NetworkSelectionElement {
	multusIface := vmispec.LookupInterfaceByName(interfaces, network.Name)
	namespace, networkName := GetNamespaceAndNetworkName(namespace, network.Multus.NetworkName)
	var multusIfaceMac string
	if multusIface != nil {
		multusIfaceMac = multusIface.MacAddress
	}
	return networkv1.NetworkSelectionElement{
		InterfaceRequest: podInterfaceName,
		MacRequest:       multusIfaceMac,
		Namespace:        namespace,
		Name:             networkName,
	}
}

func NonDefaultMultusNetworksIndexedByIfaceName(pod *k8sv1.Pod) map[string]networkv1.NetworkStatus {
	indexedNetworkStatus := map[string]networkv1.NetworkStatus{}
	podNetworkStatus, found := pod.Annotations[networkv1.NetworkStatusAnnot]

	if !found {
		return indexedNetworkStatus
	}

	var networkStatus []networkv1.NetworkStatus
	if err := json.Unmarshal([]byte(podNetworkStatus), &networkStatus); err != nil {
		log.Log.Errorf("failed to unmarshall pod network status: %v", err)
		return indexedNetworkStatus
	}

	for _, ns := range networkStatus {
		if ns.Default {
			continue
		}
		indexedNetworkStatus[ns.Interface] = ns
	}

	return indexedNetworkStatus
}
