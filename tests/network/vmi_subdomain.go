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

	expect "github.com/google/goexpect"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/tests"
	"kubevirt.io/kubevirt/tests/console"
	servicepkg "kubevirt.io/kubevirt/tests/libnet/service"
	"kubevirt.io/kubevirt/tests/libvmi"
	"kubevirt.io/kubevirt/tests/util"
)

var _ = SIGDescribe("Subdomain", func() {
	var virtClient kubecli.KubevirtClient

	BeforeEach(func() {
		var err error
		virtClient, err = kubecli.GetKubevirtClient()
		Expect(err).NotTo(HaveOccurred(), "Should successfully initialize an API client")
	})

	Context("masquerade binding", func() {
		BeforeEach(func() {
			tests.BeforeTestCleanup()
		})

		Context("with a subdomain and a headless service given", func() {
			const (
				subdomain          = "testsubdomain"
				selectorLabelKey   = "expose"
				selectorLabelValue = "this"
				servicePort        = 22
			)

			BeforeEach(func() {
				serviceName := subdomain
				service := servicepkg.BuildHeadlessSpec(serviceName, servicePort, servicePort, selectorLabelKey, selectorLabelValue)
				_, err := virtClient.CoreV1().Services(util.NamespaceTestDefault).Create(context.Background(), service, k8smetav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
			})

			fedoraMasqueradeVMI := func() (*v1.VirtualMachineInstance, error) {
				net := v1.DefaultPodNetwork()
				vmi := libvmi.NewFedora(
					libvmi.WithInterface(libvmi.InterfaceDeviceWithMasqueradeBinding([]v1.Port{}...)),
					libvmi.WithNetwork(net),
				)

				return vmi, nil
			}

			It("VMI at the subdomain should have the expected FQDN", func() {
				clientVMI, err := fedoraMasqueradeVMI()
				Expect(err).ToNot(HaveOccurred())
				clientVMI.Spec.Subdomain = subdomain
				clientVMI.Labels = map[string]string{selectorLabelKey: selectorLabelValue}

				clientVMI, err = virtClient.VirtualMachineInstance(util.NamespaceTestDefault).Create(clientVMI)
				Expect(err).ToNot(HaveOccurred())
				clientVMI = tests.WaitUntilVMIReady(clientVMI, console.LoginToFedora)

				Expect(console.SafeExpectBatch(clientVMI, []expect.Batcher{
					&expect.BSnd{S: "\n"},
					&expect.BExp{R: console.PromptExpression},
					&expect.BSnd{S: "hostname -f\n"},
					&expect.BExp{R: fmt.Sprintf("%s.%s.%s.svc.cluster.local", clientVMI.Name, subdomain, clientVMI.Namespace)},
				}, 10)).To(Succeed(), "failed to get expected FQDN")
			})
		})
	})
})
