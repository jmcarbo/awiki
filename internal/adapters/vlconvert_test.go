package adapters

import "testing"

func TestExecVLConvertImplementsInterface(t *testing.T) {
	var _ VLConvert = (*ExecVLConvert)(nil)
	t.Log("compile-time interface check ok")
}
