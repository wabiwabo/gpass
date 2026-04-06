package fingerprint

import (
	"net/http/httptest"
	"testing"
)

func TestGenerate_BasicFingerprint(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "id-ID")

	fp := Generate(req, DefaultConfig())

	if fp.Hash == "" {
		t.Error("hash should not be empty")
	}
	if len(fp.Components) == 0 {
		t.Error("components should not be empty")
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	cfg := DefaultConfig()

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("User-Agent", "Test/1.0")
	fp1 := Generate(req1, cfg)

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("User-Agent", "Test/1.0")
	fp2 := Generate(req2, cfg)

	if fp1.Hash != fp2.Hash {
		t.Error("identical requests should produce identical fingerprints")
	}
}

func TestGenerate_DifferentUA(t *testing.T) {
	cfg := DefaultConfig()

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("User-Agent", "Chrome/100")

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("User-Agent", "Firefox/100")

	fp1 := Generate(req1, cfg)
	fp2 := Generate(req2, cfg)

	if fp1.Hash == fp2.Hash {
		t.Error("different User-Agents should produce different fingerprints")
	}
}

func TestGenerate_IncludesIP(t *testing.T) {
	cfg := Config{IncludeIP: true}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "103.28.12.5")

	fp := Generate(req, cfg)
	if fp.Components["ip"] != "103.28.12.5" {
		t.Errorf("ip: got %q", fp.Components["ip"])
	}
}

func TestGenerate_IncludesMethod(t *testing.T) {
	cfg := Config{IncludeMethod: true}
	req := httptest.NewRequest("POST", "/", nil)

	fp := Generate(req, cfg)
	if fp.Components["method"] != "POST" {
		t.Errorf("method: got %q", fp.Components["method"])
	}
}

func TestGenerate_IncludesPath(t *testing.T) {
	cfg := Config{IncludePath: true}
	req := httptest.NewRequest("GET", "/api/v1/users", nil)

	fp := Generate(req, cfg)
	if fp.Components["path"] != "/api/v1/users" {
		t.Errorf("path: got %q", fp.Components["path"])
	}
}

func TestFingerprint_Match(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Test")

	cfg := DefaultConfig()
	fp1 := Generate(req, cfg)
	fp2 := Generate(req, cfg)

	if !fp1.Match(fp2) {
		t.Error("identical fingerprints should match")
	}
}

func TestFingerprint_Similarity_Identical(t *testing.T) {
	fp1 := Fingerprint{Components: map[string]string{"a": "1", "b": "2"}}
	fp2 := Fingerprint{Components: map[string]string{"a": "1", "b": "2"}}

	sim := fp1.Similarity(fp2)
	if sim != 1.0 {
		t.Errorf("identical: got %f, want 1.0", sim)
	}
}

func TestFingerprint_Similarity_Partial(t *testing.T) {
	fp1 := Fingerprint{Components: map[string]string{"a": "1", "b": "2"}}
	fp2 := Fingerprint{Components: map[string]string{"a": "1", "b": "different"}}

	sim := fp1.Similarity(fp2)
	if sim != 0.5 {
		t.Errorf("partial: got %f, want 0.5", sim)
	}
}

func TestFingerprint_Similarity_NoOverlap(t *testing.T) {
	fp1 := Fingerprint{Components: map[string]string{"a": "1"}}
	fp2 := Fingerprint{Components: map[string]string{"b": "2"}}

	sim := fp1.Similarity(fp2)
	if sim != 0.0 {
		t.Errorf("no overlap: got %f, want 0.0", sim)
	}
}

func TestFingerprint_Similarity_Empty(t *testing.T) {
	fp1 := Fingerprint{Components: map[string]string{}}
	fp2 := Fingerprint{Components: map[string]string{}}

	sim := fp1.Similarity(fp2)
	if sim != 1.0 {
		t.Errorf("both empty: got %f, want 1.0", sim)
	}
}

func TestIsSuspicious_MissingUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	suspicious, reasons := IsSuspicious(req)
	if !suspicious {
		t.Error("missing UA should be suspicious")
	}
	if len(reasons) == 0 {
		t.Error("should have reasons")
	}
}

func TestIsSuspicious_BotUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Googlebot/2.1")

	suspicious, _ := IsSuspicious(req)
	if !suspicious {
		t.Error("bot UA should be suspicious")
	}
}

func TestIsSuspicious_CurlUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")

	suspicious, _ := IsSuspicious(req)
	if !suspicious {
		t.Error("curl should be suspicious")
	}
}

func TestIsSuspicious_NormalBrowser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64)")
	req.Header.Set("Accept-Language", "id-ID")
	req.Header.Set("Accept", "text/html")

	suspicious, _ := IsSuspicious(req)
	if suspicious {
		t.Error("normal browser should not be suspicious")
	}
}

func TestIsSuspicious_MissingAcceptHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	// No Accept or Accept-Language

	suspicious, reasons := IsSuspicious(req)
	if !suspicious {
		t.Error("missing Accept headers should be suspicious")
	}
	found := false
	for _, r := range reasons {
		if r == "missing Accept-Language and Accept headers" {
			found = true
		}
	}
	if !found {
		t.Errorf("reasons: %v", reasons)
	}
}

func TestExtractIP_XFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")

	ip := extractIP(req)
	if ip != "1.2.3.4" {
		t.Errorf("ip: got %q", ip)
	}
}

func TestExtractIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")

	ip := extractIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("ip: got %q", ip)
	}
}
