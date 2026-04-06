package cidr

import "testing"

func TestParse(t *testing.T) {
	r, err := Parse("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.String() != "192.168.1.0/24" {
		t.Errorf("String = %q", r.String())
	}
}

func TestParse_Invalid(t *testing.T) {
	_, err := Parse("not-a-cidr")
	if err == nil {
		t.Error("should error")
	}
}

func TestContains(t *testing.T) {
	r, _ := Parse("10.0.0.0/8")

	if !r.Contains("10.0.0.1") { t.Error("10.0.0.1") }
	if !r.Contains("10.255.255.255") { t.Error("10.255.255.255") }
	if r.Contains("11.0.0.1") { t.Error("11.0.0.1 should not match") }
	if r.Contains("invalid") { t.Error("invalid IP") }
}

func TestContains_IPv6(t *testing.T) {
	r, _ := Parse("fd00::/8")
	if !r.Contains("fd00::1") { t.Error("fd00::1") }
	if r.Contains("fe80::1") { t.Error("fe80::1 should not match") }
}

func TestNetwork_Mask(t *testing.T) {
	r, _ := Parse("192.168.1.0/24")
	if r.Network() != "192.168.1.0" {
		t.Errorf("Network = %q", r.Network())
	}
	if r.Mask() != "255.255.255.0" {
		t.Errorf("Mask = %q", r.Mask())
	}
}

func TestList(t *testing.T) {
	l, err := NewList("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")
	if err != nil {
		t.Fatalf("NewList: %v", err)
	}
	if l.Len() != 3 {
		t.Errorf("Len = %d", l.Len())
	}

	if !l.Contains("10.0.0.1") { t.Error("10.0.0.1") }
	if !l.Contains("172.16.5.10") { t.Error("172.16.5.10") }
	if !l.Contains("192.168.1.1") { t.Error("192.168.1.1") }
	if l.Contains("8.8.8.8") { t.Error("8.8.8.8 should not match") }
}

func TestList_Invalid(t *testing.T) {
	_, err := NewList("invalid")
	if err == nil {
		t.Error("should error on invalid CIDR")
	}
}

func TestIsPrivate(t *testing.T) {
	privates := []string{"10.0.0.1", "172.16.0.1", "192.168.1.1"}
	for _, ip := range privates {
		if !IsPrivate(ip) {
			t.Errorf("%s should be private", ip)
		}
	}

	publics := []string{"8.8.8.8", "1.1.1.1", "203.0.113.1"}
	for _, ip := range publics {
		if IsPrivate(ip) {
			t.Errorf("%s should not be private", ip)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	if !IsLoopback("127.0.0.1") { t.Error("127.0.0.1") }
	if !IsLoopback("::1") { t.Error("::1") }
	if IsLoopback("10.0.0.1") { t.Error("10.0.0.1") }
	if IsLoopback("invalid") { t.Error("invalid") }
}

func TestNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"127.0.0.1", "127.0.0.1"},
		{"::1", "::1"},
		{"invalid", "invalid"},
	}
	for _, tt := range tests {
		if got := Normalize(tt.in); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsPrivate_Invalid(t *testing.T) {
	if IsPrivate("not-an-ip") {
		t.Error("invalid IP should not be private")
	}
}
