package docker

import (
	"net"
	"testing"
)

// ---- parsePortMappings ----

func TestParsePortMappings_Valid(t *testing.T) {
	got := parsePortMappings("test", "9000:80,9001:8080")
	if len(got) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(got))
	}
	if got[0].ListenPort != 9000 || got[0].TargetPort != 80 {
		t.Errorf("mapping 0: got %+v, want {9000 80}", got[0])
	}
	if got[1].ListenPort != 9001 || got[1].TargetPort != 8080 {
		t.Errorf("mapping 1: got %+v, want {9001 8080}", got[1])
	}
}

func TestParsePortMappings_SingleMapping(t *testing.T) {
	got := parsePortMappings("test", "5353:53")
	if len(got) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(got))
	}
	if got[0].ListenPort != 5353 || got[0].TargetPort != 53 {
		t.Errorf("got %+v, want {5353 53}", got[0])
	}
}

func TestParsePortMappings_WhitespaceAround(t *testing.T) {
	got := parsePortMappings("test", " 9000 : 80 ")
	if len(got) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(got))
	}
	if got[0].ListenPort != 9000 || got[0].TargetPort != 80 {
		t.Errorf("got %+v, want {9000 80}", got[0])
	}
}

func TestParsePortMappings_InvalidTokenSkipped(t *testing.T) {
	// One valid, one missing colon — invalid token is skipped
	got := parsePortMappings("test", "9000:80,notaport")
	if len(got) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(got))
	}
	if got[0].ListenPort != 9000 {
		t.Errorf("got %+v, want listen=9000", got[0])
	}
}

func TestParsePortMappings_NonIntegerPortsSkipped(t *testing.T) {
	got := parsePortMappings("test", "abc:xyz,9000:80")
	if len(got) != 1 {
		t.Fatalf("expected 1 mapping after skipping invalid, got %d", len(got))
	}
	if got[0].ListenPort != 9000 {
		t.Errorf("got %+v, want listen=9000", got[0])
	}
}

func TestParsePortMappings_AllInvalid(t *testing.T) {
	got := parsePortMappings("test", "abc:xyz,nocolon")
	if len(got) != 0 {
		t.Errorf("expected 0 mappings, got %d", len(got))
	}
}

// ---- parseIPList ----

func TestParseIPList_PlainIPv4(t *testing.T) {
	nets := parseIPList("test", "192.168.1.1")
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
	ip := net.ParseIP("192.168.1.1")
	if !nets[0].Contains(ip) {
		t.Errorf("expected net to contain 192.168.1.1")
	}
	// Should be a /32
	ones, bits := nets[0].Mask.Size()
	if ones != 32 || bits != 32 {
		t.Errorf("expected /32, got /%d", ones)
	}
}

func TestParseIPList_PlainIPv6(t *testing.T) {
	nets := parseIPList("test", "::1")
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
	ones, bits := nets[0].Mask.Size()
	if ones != 128 || bits != 128 {
		t.Errorf("expected /128, got /%d", ones)
	}
}

func TestParseIPList_CIDR(t *testing.T) {
	nets := parseIPList("test", "192.168.0.0/16")
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
	if !nets[0].Contains(net.ParseIP("192.168.1.100")) {
		t.Errorf("CIDR should contain 192.168.1.100")
	}
	if nets[0].Contains(net.ParseIP("10.0.0.1")) {
		t.Errorf("CIDR should not contain 10.0.0.1")
	}
}

func TestParseIPList_Multiple(t *testing.T) {
	nets := parseIPList("test", "10.0.0.1,192.168.0.0/16,::1")
	if len(nets) != 3 {
		t.Fatalf("expected 3 nets, got %d", len(nets))
	}
}

func TestParseIPList_InvalidEntrySkipped(t *testing.T) {
	nets := parseIPList("test", "notanip,10.0.0.1")
	if len(nets) != 1 {
		t.Fatalf("expected 1 net after skipping invalid, got %d", len(nets))
	}
}

func TestParseIPList_Empty(t *testing.T) {
	nets := parseIPList("test", "")
	if len(nets) != 0 {
		t.Errorf("expected 0 nets for empty string, got %d", len(nets))
	}
}

func TestParseIPList_WhitespaceOnly(t *testing.T) {
	nets := parseIPList("test", "  ,  ")
	if len(nets) != 0 {
		t.Errorf("expected 0 nets for whitespace-only, got %d", len(nets))
	}
}
