package tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1network "k8s.io/api/networking/v1"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/tests"
	cd "kubevirt.io/kubevirt/tests/containerdisk"
)

func assertPingSucceedBetweenVMs(vmisrc, vmidst *v1.VirtualMachineInstance) {
	for _, ip := range vmidst.Status.Interfaces[0].IPs {
		assertPingSucceed(ip, vmisrc)
	}
}

func assertPingSucceed(ip string, vmi *v1.VirtualMachineInstance) {
	expecter, err := tests.LoggedInCirrosExpecter(vmi)
	Expect(err).ToNot(HaveOccurred())
	defer expecter.Close()

	Expect(tests.PingFromVMConsole(vmi, ip, "-w 3")).To(Succeed())
}

func assertPingFailBetweenVMs(vmisrc, vmidst *v1.VirtualMachineInstance) {
	for _, ip := range vmidst.Status.Interfaces[0].IPs {
		assertPingFail(ip, vmisrc)
	}
}

func assertPingFail(ip string, vmi *v1.VirtualMachineInstance) {
	expecter, err := tests.LoggedInCirrosExpecter(vmi)
	Expect(err).ToNot(HaveOccurred())
	defer expecter.Close()

	err = tests.PingFromVMConsole(vmi, ip, "-w 3")
	Expect(err).To(HaveOccurred())
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
		tests.BeforeTestCleanup()
		// Create three vmis, vmia and vmib are in same namespace, vmic is in different namespace
		vmia = tests.NewRandomVMIWithEphemeralDiskAndUserdata(cd.ContainerDiskFor(cd.ContainerDiskCirros), "#!/bin/bash\necho 'hello'\n")
		vmia.Labels = map[string]string{"type": "test"}
		vmia, err = virtClient.VirtualMachineInstance(tests.NamespaceTestDefault).Create(vmia)
		Expect(err).ToNot(HaveOccurred())

		vmib = tests.NewRandomVMIWithEphemeralDiskAndUserdata(cd.ContainerDiskFor(cd.ContainerDiskCirros), "#!/bin/bash\necho 'hello'\n")
		vmib, err = virtClient.VirtualMachineInstance(tests.NamespaceTestDefault).Create(vmib)
		Expect(err).ToNot(HaveOccurred())

		vmic = tests.NewRandomVMIWithEphemeralDiskAndUserdata(cd.ContainerDiskFor(cd.ContainerDiskCirros), "#!/bin/bash\necho 'hello'\n")
		vmic.Namespace = tests.NamespaceTestAlternative
		vmic, err = virtClient.VirtualMachineInstance(tests.NamespaceTestAlternative).Create(vmic)
		Expect(err).ToNot(HaveOccurred())

		vmia = tests.WaitUntilVMIReady(vmia, tests.LoggedInCirrosExpecter)
		vmib = tests.WaitUntilVMIReady(vmib, tests.LoggedInCirrosExpecter)
		vmic = tests.WaitUntilVMIReadyNamespace(vmic, vmic.Namespace, tests.LoggedInCirrosExpecter)
	})

	Context("vms limited by Default-deny networkpolicy", func() {

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
			_, err := virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:1511] should be failed to reach vmia from vmib", func() {
			By("Connect vmia from vmib")
			Expect(vmia.Status.Interfaces[0].IPs).NotTo(BeEmpty())
			assertPingFailBetweenVMs(vmib, vmia)
		})

		It("[test_id:1512] should be failed to reach vmib from vmia", func() {
			By("Connect vmib from vmia")
			Expect(vmib.Status.Interfaces[0].IPs).NotTo(BeEmpty())
			assertPingFailBetweenVMs(vmia, vmib)
		})

		AfterEach(func() {
			Expect(virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Delete("deny-by-default", &v13.DeleteOptions{})).To(Succeed())
		})

	})

	Context("vms limited by allow same namespace networkpolicy", func() {
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
			_, err := virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:1513] should be successful to reach vmia from vmib", func() {
			By("Connect vmia from vmib in same namespace")
			Expect(vmia.Status.Interfaces[0].IPs).NotTo(BeEmpty())
			assertPingSucceedBetweenVMs(vmib, vmia)
		})

		It("[test_id:1514] should be failed to reach vmia from vmic", func() {
			By("Connect vmia from vmic in differnet namespace")
			Expect(vmia.Status.Interfaces[0].IPs).NotTo(BeEmpty())
			assertPingFailBetweenVMs(vmic, vmia)
		})

		AfterEach(func() {
			Expect(virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Delete("allow-same-namespace", &v13.DeleteOptions{})).To(Succeed())
		})

	})

	Context("vms limited by deny by label networkpolicy", func() {
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
			_, err := virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Create(networkpolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:1515] should be failed to reach vmia from vmic", func() {
			By("Connect vmia from vmic")
			Expect(vmia.Status.Interfaces[0].IPs).NotTo(BeEmpty())
			assertPingFailBetweenVMs(vmic, vmia)
		})

		It("[test_id:1516] should be failed to reach vmia from vmib", func() {
			By("Connect vmia from vmib")
			Expect(vmia.Status.Interfaces[0].IPs).NotTo(BeEmpty())
			assertPingFailBetweenVMs(vmib, vmia)
		})

		It("[test_id:1517] should be successful to reach vmib from vmic", func() {
			By("Connect vmib from vmic")
			Expect(vmib.Status.Interfaces[0].IPs).NotTo(BeEmpty())
			assertPingSucceedBetweenVMs(vmic, vmib)
		})

		AfterEach(func() {
			Expect(virtClient.NetworkingV1().NetworkPolicies(vmia.Namespace).Delete("deny-by-label", &v13.DeleteOptions{})).To(Succeed())
		})

	})

})
