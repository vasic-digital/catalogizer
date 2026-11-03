package models

import "testing"

func TestUserRegistrationPendingMarker(t *testing.T) {
	u := &User{IsActive: false, Settings: RegistrationPendingSettings}
	if !u.IsRegistrationPending() {
		t.Fatal("inactive user with marker must be pending")
	}
	if (&User{IsActive: true, Settings: RegistrationPendingSettings}).IsRegistrationPending() {
		t.Fatal("an active user is never pending")
	}
	for _, s := range []string{"", "{}", "not json", `{"registration_status":"other"}`} {
		if (&User{IsActive: false, Settings: s}).IsRegistrationPending() {
			t.Fatalf("settings %q must not be pending", s)
		}
	}
	u = &User{IsActive: false, Settings: `{"registration_status":"pending","keep":1}`}
	u.ClearRegistrationPending()
	if u.IsRegistrationPending() || u.Settings != `{"keep":1}` {
		t.Fatalf("clear must drop only the marker, got %q", u.Settings)
	}
}
