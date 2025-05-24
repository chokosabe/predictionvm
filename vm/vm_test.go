package vm

import (
	"testing"
)

// TestVMInitialization is a placeholder test.
// The act of compiling and running tests for package 'vm'
// should trigger the init() function in vm.go, which is where
// the ActionParser registration and potential panic occurs.
func TestVMInitialization(t *testing.T) {
	t.Log("Attempting to trigger vm.init() by running a test in the vm package.")
	// If the init() function panics, the test will fail here.
	// The debug prints from GetTypeID should appear in the test output before the panic.
}
