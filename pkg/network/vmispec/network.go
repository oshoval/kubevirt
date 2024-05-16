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

package vmispec

import (
	"strings"

	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/client-go/precond"
)

func LookupNetworkByName(networks []v1.Network, name string) *v1.Network {
	for i := range networks {
		if networks[i].Name == name {
			return &networks[i]
		}
	}
	return nil
}

func LookupPodNetwork(networks []v1.Network) *v1.Network {
	for _, network := range networks {
		if network.Pod != nil {
			net := network
			return &net
		}
	}
	return nil
}

func FilterMultusNonDefaultNetworks(networks []v1.Network) []v1.Network {
	return FilterNetworksSpec(networks, IsSecondaryMultusNetwork)
}

func FilterMultusNetworks(networks []v1.Network) []v1.Network {
	return FilterNetworksSpec(networks, IsMultusNetwork)
}

func FilterNetworksSpec(nets []v1.Network, predicate func(i v1.Network) bool) []v1.Network {
	var filteredNets []v1.Network
	for _, net := range nets {
		if predicate(net) {
			filteredNets = append(filteredNets, net)
		}
	}
	return filteredNets
}

func LookUpDefaultNetwork(networks []v1.Network) *v1.Network {
	for i, network := range networks {
		if !IsSecondaryMultusNetwork(network) {
			return &networks[i]
		}
	}
	return nil
}

func IsSecondaryMultusNetwork(net v1.Network) bool {
	return net.Multus != nil && !net.Multus.Default
}

func IsMultusNetwork(net v1.Network) bool {
	return net.Multus != nil
}

func IndexNetworkSpecByName(networks []v1.Network) map[string]v1.Network {
	indexedNetworks := map[string]v1.Network{}
	for _, network := range networks {
		indexedNetworks[network.Name] = network
	}
	return indexedNetworks
}

func FilterNetworksByInterfaces(networks []v1.Network, interfaces []v1.Interface) []v1.Network {
	var nets []v1.Network
	networksByName := IndexNetworkSpecByName(networks)
	for _, iface := range interfaces {
		if net, exists := networksByName[iface.Name]; exists {
			nets = append(nets, net)
		}
	}
	return nets
}

func GetNamespaceAndNetworkName(namespace string, fullNetworkName string) (string, string) {
	if strings.Contains(fullNetworkName, "/") {
		res := strings.SplitN(fullNetworkName, "/", 2)
		return res[0], res[1]
	}
	return precond.MustNotBeEmpty(namespace), fullNetworkName
}
