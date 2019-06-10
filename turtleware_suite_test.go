package turtleware_test

import (
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)

	// Discard output, put capture it via hook
	logrus.StandardLogger().Out = ioutil.Discard
	hooks := test.NewGlobal()

	if !RunSpecs(t, "Server Suite") {
		for _, value := range hooks.AllEntries() {
			t.Error(value.Message)
		}
	}
}
