package handlers

import "testing"

func TestRegistrationMode(t *testing.T) {
	cases := map[string]string{
		"": RegistrationModeApproval, "approval": RegistrationModeApproval, " OPEN ": RegistrationModeOpen,
		"open": RegistrationModeOpen, "disabled": RegistrationModeDisabled,
		"bogus": RegistrationModeApproval, // unknown fails closed, never to open
	}
	for in, want := range cases {
		t.Setenv("REGISTRATION_MODE", in)
		if got := RegistrationMode(); got != want {
			t.Errorf("REGISTRATION_MODE=%q: got %q want %q", in, got, want)
		}
	}
}
