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

package libipam

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/network/vmispec"

	k8scnicncfiov1 "kubevirt.io/client-go/generated/network-attachment-definition-client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IPAMClaimParams struct {
	ClaimName   string
	NetworkName string
}

type NetConf struct {
	AllowPersistentIPs bool   `json:"allowPersistentIPs,omitempty"`
	Name               string `json:"name,omitempty"`
}

func PersistentIPNetwork(networkClient k8scnicncfiov1.K8sCniCncfIoV1Interface, namespace string, net v1.Network) (bool, error) {
	if !vmispec.IsSecondaryMultusNetwork(net) {
		return false, nil
	}

	namespace, networkName := vmispec.GetNamespaceAndNetworkName(namespace, net.Multus.NetworkName)
	nad, err := networkClient.NetworkAttachmentDefinitions(namespace).Get(context.Background(), networkName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to locate network attachment definition %s/%s", namespace, networkName)
	}
	netConf, err := GetPersistentIPsConf(nad)
	if err != nil {
		return false, fmt.Errorf("failed retrieving persistentIPsNetworkName: %w", err)
	}

	return netConf.AllowPersistentIPs, nil
}

func GetPersistentIPsConf(nad *networkv1.NetworkAttachmentDefinition) (NetConf, error) {
	if nad.Spec.Config == "" {
		return NetConf{}, nil
	}

	var netConf NetConf
	err := json.Unmarshal([]byte(nad.Spec.Config), &netConf)
	if err != nil {
		return NetConf{}, fmt.Errorf("failed to unmarshal NAD spec.config JSON: %v", err)
	}

	if netConf.Name == "" {
		return NetConf{}, fmt.Errorf("failed to obtain network name: missing required field")
	}

	return netConf, nil
}
