package adapters

import "testing"

func TestExecVendorVegaImplementsInterface(t *testing.T) {
	var _ VendorVega = (*ExecVendorVega)(nil)
	t.Log("compile-time interface check ok")
}
