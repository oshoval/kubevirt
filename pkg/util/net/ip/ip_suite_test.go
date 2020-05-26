package ip_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ip Suite")
}
