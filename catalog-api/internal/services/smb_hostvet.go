package services

import "net"

// RED-step stubs (replaced in the fix commit).

// SMBHostResolver resolves a host name; tests replace it.
var SMBHostResolver = net.LookupIP

func ValidateSMBHostSyntax(host string) error { return nil }
func VetSMBHost(host string) error            { return nil }
func SMBAllowedPorts() []int                  { return []int{445} }
func filterShareInfos(host string, names []string) []SMBShareInfo { return nil }
