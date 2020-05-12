package util

import (
	"bufio"
	"os"

	v1 "kubevirt.io/client-go/api/v1"
	//"kubevirt.io/client-go/log"
)

const ExtensionAPIServerAuthenticationConfigMap = "extension-apiserver-authentication"
const RequestHeaderClientCAFileKey = "requestheader-client-ca-file"
const VirtShareDir = "/var/run/kubevirt"
const VirtPrivateDir = "/var/run/kubevirt-private"
const VirtLibDir = "/var/lib/kubevirt"
const HostRootMount = "/proc/1/root/"
const CPUManagerOS3Path = HostRootMount + "var/lib/origin/openshift.local.volumes/cpu_manager_state"
const CPUManagerPath = HostRootMount + "var/lib/kubelet/cpu_manager_state"

func IsSRIOVVmi(vmi *v1.VirtualMachineInstance) bool {
	for _, iface := range vmi.Spec.Domain.Devices.Interfaces {
		if iface.SRIOV != nil {
			return true
		}
	}
	return false
}

// Check if a VMI spec requests GPU
func IsGPUVMI(vmi *v1.VirtualMachineInstance) bool {
	if vmi.Spec.Domain.Devices.GPUs != nil && len(vmi.Spec.Domain.Devices.GPUs) != 0 {
		return true
	}
	return false
}

/*
// IsIpv6Disabled returns if IPv6 is disabled according sysctl
func IsIpv6Disabled() bool {
	ipv6Disabled, err := exec.Command("cat", "/proc/sys/net/ipv6/conf/default/disable_ipv6").Output()
	return err != nil || string(ipv6Disabled) == "1"
}

// GetIPBindAddress returns IP bind address (either 0.0.0.0 or [::] according sysctl disable_ipv6)
func GetIPBindAddress() string {
	if IsIpv6Disabled() {
		return "0.0.0.0"
	}

	return "[::]"
}
*/

// GetIPBindAddress returns IP bind address (either 0.0.0.0 or [::] according sysctl disable_ipv6)
func GetIPBindAddress() string {
	var disableIPv6 string
	file, err := os.Open("/proc/sys/net/ipv6/conf/default/disable_ipv6")
	if err != nil {
		//log.Log.Infof("DBGDBGX1 %s", "0.0.0.0")
		return "0.0.0.0"
	}

	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		disableIPv6 = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		//log.Log.Infof("DBGDBGX2 %s", "0.0.0.0")
		return "0.0.0.0"
	}

	if disableIPv6 == "1" {
		//log.Log.Infof("DBGDBGX3 %s", "0.0.0.0")
		return "0.0.0.0"
	}

	//log.Log.Infof("DBGDBGX4 %s", "[::]")
	return "[::]"
}
