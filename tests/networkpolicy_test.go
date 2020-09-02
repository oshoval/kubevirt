package tests_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1network "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/tests"
	cd "kubevirt.io/kubevirt/tests/containerdisk"
)

var _ = Describe("[rfe_id:150][crit:high][vendor:cnv-qe@redhat.com][level:component]Networkpolicy", func() {

	var virtClient kubecli.KubevirtClient

	var vmia *v1.VirtualMachineInstance
	var vmib *v1.VirtualMachineInstance
	var vmic *v1.VirtualMachineInstance

	tests.BeforeAll(func() {
		var err error

		virtClient, err = kubecli.GetKubevirtClient()
		tests.PanicOnError(err)

		tests.SkipIfUseFlannel(virtClient)
		tests.SkipNetworkPolicyRunningOnKindInfra()
		tests.BeforeTestCleanup()

		// Create three vmis, vmia and vmib are in same namespace, vmic is in different namespace
		vmia = setupVMIWithEphemeralDiskAndUserdata(virtClient, tests.NamespaceTestDefault, map[string]string{"type": "test"})
		vmib = setupVMIWithEphemeralDiskAndUserdata(virtClient, tests.NamespaceTestDefault, map[string]string{})
		vmic = setupVMIWithEphemeralDiskAndUserdata(virtClient, tests.NamespaceTestAlternative, map[string]string{})
	})

	Context("vms limited by Default-deny networkpolicy", func() {
		var policy *v1network.NetworkPolicy

		BeforeEach(func() {
			// deny-by-default networkpolicy will deny all the traffice to the vms in the namespace
			By("Create deny-by-default networkpolicy")
			networkpolicy := &v1network.NetworkPolicy{
				ObjectMeta: v13.ObjectMeta{
					Name: "deny-by-default",
				},
				Spec: v1network.NetworkPolicySpec{
					PodSelector: v13.LabelSelector{},
					Ingress:     []v1network.NetworkPolicyIngressRule{},
				},
			}
			var err error
			policy, err = virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			waitForNetworkPolicyDeletion(policy)
		})

		It("[test_id:1511] should be failed to reach vmia from vmib", func() {
			By("Connect vmia from vmib")
			pingEventually(vmib, vmia.Status.Interfaces[0].IP).ShouldNot(Succeed())
		})

		It("[test_id:1512] should be failed to reach vmib from vmia", func() {
			By("Connect vmib from vmia")
			pingEventually(vmia, vmib.Status.Interfaces[0].IP).ShouldNot(Succeed())
		})

	})

	Context("vms limited by allow same namespace networkpolicy", func() {
		var policy *v1network.NetworkPolicy

		BeforeEach(func() {
			// allow-same-namespave networkpolicy will only allow the traffice inside the namespace
			By("Create allow-same-namespace networkpolicy")
			networkpolicy := &v1network.NetworkPolicy{
				ObjectMeta: v13.ObjectMeta{
					Name: "allow-same-namespace",
				},
				Spec: v1network.NetworkPolicySpec{
					PodSelector: v13.LabelSelector{},
					Ingress: []v1network.NetworkPolicyIngressRule{
						{
							From: []v1network.NetworkPolicyPeer{
								{
									PodSelector: &v13.LabelSelector{},
								},
							},
						},
					},
				},
			}
			var err error
			policy, err = virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			waitForNetworkPolicyDeletion(policy)
		})

		It("[test_id:1513] should be successful to reach vmia from vmib", func() {
			By("Connect vmia from vmib in same namespace")
			pingEventually(vmib, vmia.Status.Interfaces[0].IP).Should(Succeed())
		})

		It("[test_id:1514] should be failed to reach vmia from vmic", func() {
			By("Connect vmia from vmic in differnet namespace")
			pingEventually(vmic, vmia.Status.Interfaces[0].IP).ShouldNot(Succeed())
		})

	})

	Context("vms limited by deny by label networkpolicy", func() {
		var policy *v1network.NetworkPolicy

		BeforeEach(func() {
			// deny-by-label networkpolicy will deny the traffice for the vm which have the same label
			By("Create deny-by-label networkpolicy")
			networkpolicy := &v1network.NetworkPolicy{
				ObjectMeta: v13.ObjectMeta{
					Name: "deny-by-label",
				},
				Spec: v1network.NetworkPolicySpec{
					PodSelector: v13.LabelSelector{
						MatchLabels: map[string]string{
							"type": "test",
						},
					},
					Ingress: []v1network.NetworkPolicyIngressRule{},
				},
			}
			var err error
			policy, err = virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			waitForNetworkPolicyDeletion(policy)
		})

		It("[test_id:1515] should be failed to reach vmia from vmic", func() {
			By("Connect vmia from vmic")
			pingEventually(vmic, vmia.Status.Interfaces[0].IP).ShouldNot(Succeed())
		})

		It("[test_id:1516] should be failed to reach vmia from vmib", func() {
			By("Connect vmia from vmib")
			pingEventually(vmib, vmia.Status.Interfaces[0].IP).ShouldNot(Succeed())
		})

		It("[test_id:1517] should be successful to reach vmib from vmic", func() {
			By("Connect vmib from vmic")
			pingEventually(vmic, vmib.Status.Interfaces[0].IP).Should(Succeed())
		})

	})

})

func setupVMIWithEphemeralDiskAndUserdata(virtClient kubecli.KubevirtClient, namespace string, labels map[string]string) *v1.VirtualMachineInstance {
	var err error
	vmi := tests.NewRandomVMIWithEphemeralDiskAndUserdata(cd.ContainerDiskFor(cd.ContainerDiskCirros), "#!/bin/bash\necho 'hello'\n")
	vmi.Namespace = namespace
	vmi.Labels = labels
	vmi, err = virtClient.VirtualMachineInstance(vmi.Namespace).Create(vmi)
	Expect(err).ToNot(HaveOccurred())

	return tests.WaitUntilVMIReady(vmi, tests.LoggedInCirrosExpecter)
}

func pingEventually(fromVmi *v1.VirtualMachineInstance, toIp string) AsyncAssertion {
	return Eventually(func() error {
		By(fmt.Sprintf("Pinging from VMI %s/%s to Ip %s", fromVmi.Namespace, fromVmi.Name, toIp))
		return tests.PingFromVMConsole(fromVmi, toIp)
	}, 15*time.Second, time.Second)
}

func waitForNetworkPolicyDeletion(policy *v1network.NetworkPolicy) {
	if policy == nil {
		return
	}

	virtClient, err := kubecli.GetKubevirtClient()
	Expect(err).ToNot(HaveOccurred())

	ExpectWithOffset(1, virtClient.NetworkingV1().NetworkPolicies(policy.Namespace).Delete(policy.Name, &v13.DeleteOptions{})).To(Succeed())
	EventuallyWithOffset(1, func() error {
		_, err := virtClient.NetworkingV1().NetworkPolicies(policy.Namespace).Get(policy.Name, v13.GetOptions{})
		return err
	}, 10*time.Second, time.Second).Should(SatisfyAll(HaveOccurred(), WithTransform(errors.IsNotFound, BeTrue())))
}
