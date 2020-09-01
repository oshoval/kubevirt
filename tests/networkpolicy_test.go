package tests_test

import (
	"fmt"
	"time"

	expect "github.com/google/goexpect"
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

func assertPingSucceed(ip string, vmi *v1.VirtualMachineInstance) {
	expecter, err := tests.LoggedInCirrosExpecter(vmi)
	Expect(err).ToNot(HaveOccurred())
	defer expecter.Close()

	err = tests.CheckForTextExpecter(vmi, []expect.Batcher{
		&expect.BSnd{S: fmt.Sprintf("ping -w 3 %s \n", ip)},
		&expect.BExp{R: "0% packet loss"},
	}, 60)
	Expect(err).ToNot(HaveOccurred())
}

func assertPingFail(ip string, vmi *v1.VirtualMachineInstance) {
	expecter, err := tests.LoggedInCirrosExpecter(vmi)
	Expect(err).ToNot(HaveOccurred())
	defer expecter.Close()

	err = tests.CheckForTextExpecter(vmi, []expect.Batcher{
		&expect.BSnd{S: fmt.Sprintf("ping -w 3 %s \n", ip)},
		&expect.BExp{R: "100% packet loss"},
	}, 60)
	Expect(err).ToNot(HaveOccurred())
}

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
		vmia = tests.NewRandomVMIWithEphemeralDiskAndUserdata(cd.ContainerDiskFor(cd.ContainerDiskCirros), "#!/bin/bash\necho 'hello'\n")
		vmia.Labels = map[string]string{"type": "test"}
		vmia, err = virtClient.VirtualMachineInstance(tests.NamespaceTestDefault).Create(vmia)
		Expect(err).ToNot(HaveOccurred())

		vmib = tests.NewRandomVMIWithEphemeralDiskAndUserdata(cd.ContainerDiskFor(cd.ContainerDiskCirros), "#!/bin/bash\necho 'hello'\n")
		_, err = virtClient.VirtualMachineInstance(tests.NamespaceTestDefault).Create(vmib)
		Expect(err).ToNot(HaveOccurred())

		vmic = tests.NewRandomVMIWithEphemeralDiskAndUserdata(cd.ContainerDiskFor(cd.ContainerDiskCirros), "#!/bin/bash\necho 'hello'\n")
		vmic.Namespace = tests.NamespaceTestAlternative
		_, err = virtClient.VirtualMachineInstance(tests.NamespaceTestAlternative).Create(vmic)
		Expect(err).ToNot(HaveOccurred())

		tests.WaitForSuccessfulVMIStart(vmia)
		tests.WaitForSuccessfulVMIStart(vmib)
		tests.WaitForSuccessfulVMIStart(vmic)

		vmia, err = virtClient.VirtualMachineInstance(tests.NamespaceTestDefault).Get(vmia.Name, &v13.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		vmib, err = virtClient.VirtualMachineInstance(tests.NamespaceTestDefault).Get(vmib.Name, &v13.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		vmic, err = virtClient.VirtualMachineInstance(tests.NamespaceTestAlternative).Get(vmic.Name, &v13.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

	})

	Context("vms limited by Default-deny networkpolicy", func() {
		var networkpolicy *v1network.NetworkPolicy

		BeforeEach(func() {
			// deny-by-default networkpolicy will deny all the traffice to the vms in the namespace
			By("Create deny-by-default networkpolicy")
			networkpolicy = &v1network.NetworkPolicy{
				ObjectMeta: v13.ObjectMeta{
					Name: "deny-by-default",
				},
				Spec: v1network.NetworkPolicySpec{
					PodSelector: v13.LabelSelector{},
					Ingress:     []v1network.NetworkPolicyIngressRule{},
				},
			}
			_, err := virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			waitForNetworkPolicyDeletion(vmia, networkpolicy)
		})

		It("[test_id:1511] should be failed to reach vmia from vmib", func() {
			By("Connect vmia from vmib")
			ip := vmia.Status.Interfaces[0].IP
			assertPingFail(ip, vmib)
		})

		It("[test_id:1512] should be failed to reach vmib from vmia", func() {
			By("Connect vmib from vmia")
			ip := vmib.Status.Interfaces[0].IP
			assertPingFail(ip, vmia)
		})

	})

	Context("vms limited by allow same namespace networkpolicy", func() {
		var networkpolicy *v1network.NetworkPolicy

		BeforeEach(func() {
			// allow-same-namespave networkpolicy will only allow the traffice inside the namespace
			By("Create allow-same-namespace networkpolicy")
			networkpolicy = &v1network.NetworkPolicy{
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
			_, err := virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			waitForNetworkPolicyDeletion(vmia, networkpolicy)
		})

		It("[test_id:1513] should be successful to reach vmia from vmib", func() {
			By("Connect vmia from vmib in same namespace")
			ip := vmia.Status.Interfaces[0].IP
			assertPingSucceed(ip, vmib)
		})

		It("[test_id:1514] should be failed to reach vmia from vmic", func() {
			By("Connect vmia from vmic in differnet namespace")
			ip := vmia.Status.Interfaces[0].IP
			assertPingFail(ip, vmic)
		})

	})

	Context("vms limited by deny by label networkpolicy", func() {
		var networkpolicy *v1network.NetworkPolicy

		BeforeEach(func() {
			// deny-by-label networkpolicy will deny the traffice for the vm which have the same label
			By("Create deny-by-label networkpolicy")
			networkpolicy = &v1network.NetworkPolicy{
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
			_, err := virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			waitForNetworkPolicyDeletion(vmia, networkpolicy)
		})

		It("[test_id:1515] should be failed to reach vmia from vmic", func() {
			By("Connect vmia from vmic")
			ip := vmia.Status.Interfaces[0].IP
			assertPingFail(ip, vmic)
		})

		It("[test_id:1516] should be failed to reach vmia from vmib", func() {
			By("Connect vmia from vmib")
			ip := vmia.Status.Interfaces[0].IP
			assertPingFail(ip, vmib)
		})

		It("[test_id:1517] should be successful to reach vmib from vmic", func() {
			By("Connect vmib from vmic")
			ip := vmib.Status.Interfaces[0].IP
			assertPingSucceed(ip, vmic)
		})

	})

})

func waitForNetworkPolicyDeletion(vmi *v1.VirtualMachineInstance, networkpolicy *v1network.NetworkPolicy) {
	if networkpolicy == nil {
		return
	}

	virtClient, err := kubecli.GetKubevirtClient()
	tests.PanicOnError(err)

	_, err = virtClient.NetworkingV1().NetworkPolicies(vmi.Namespace).Get(networkpolicy.Name, v13.GetOptions{})
	if err == nil {
		ExpectWithOffset(1, virtClient.NetworkingV1().NetworkPolicies(vmi.Namespace).Delete(networkpolicy.Name, &v13.DeleteOptions{})).To(Succeed())
		EventuallyWithOffset(1, func() error {
			_, err := virtClient.NetworkingV1().NetworkPolicies(vmi.Namespace).Get(networkpolicy.Name, v13.GetOptions{})
			return err
		}, 10*time.Second, time.Second).Should(SatisfyAll(HaveOccurred(), WithTransform(errors.IsNotFound, BeTrue())))
	}
}
