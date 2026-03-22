package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestUpdateOptionMapPasswordRegisterCodes(t *testing.T) {
	common.OptionMap = map[string]string{}
	common.PasswordRegisterCodeEnabled = false
	common.UpdatePasswordRegisterCodes("")

	if err := updateOptionMap("PasswordRegisterCodeEnabled", "true"); err != nil {
		t.Fatalf("unexpected error updating enable switch: %v", err)
	}
	if !common.PasswordRegisterCodeEnabled {
		t.Fatal("expected password register code switch to be enabled")
	}

	if err := updateOptionMap("PasswordRegisterCodes", `["alpha","beta"]`); err != nil {
		t.Fatalf("unexpected error updating codes: %v", err)
	}
	if !common.IsPasswordRegisterCodeValid("alpha") || !common.IsPasswordRegisterCodeValid("beta") {
		t.Fatal("expected configured codes to be accepted")
	}
}
