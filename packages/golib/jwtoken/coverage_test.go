package jwtoken

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// TestParseClaims_FormatAndDecodeErrors pins the two error branches in
// ParseClaims: wrong-part-count and bad base64/json segment.
func TestParseClaims_FormatAndDecodeErrors(t *testing.T) {
	if _, err := ParseClaims("only.two"); err == nil || !strings.Contains(err.Error(), "invalid token format") {
		t.Errorf("two parts: %v", err)
	}
	bad := b64(`{"alg":"ES256"}`) + ".!!!." + b64("sig")
	if _, err := ParseClaims(bad); err == nil || !strings.Contains(err.Error(), "invalid claims") {
		t.Errorf("bad claims b64: %v", err)
	}
}

// TestParseClaimsMap_FormatAndDecodeErrors pins the same two branches
// for ParseClaimsMap.
func TestParseClaimsMap_FormatAndDecodeErrors(t *testing.T) {
	if _, err := ParseClaimsMap("nope"); err == nil || !strings.Contains(err.Error(), "invalid token format") {
		t.Errorf("one part: %v", err)
	}
	bad := b64(`{}`) + "." + b64("not json") + "." + b64("sig")
	if _, err := ParseClaimsMap(bad); err == nil || !strings.Contains(err.Error(), "invalid claims") {
		t.Errorf("bad claims json: %v", err)
	}
}

// TestParseClaims_HappyPathSingleAndMultiAudience pins the dual-format
// Audience UnmarshalJSON: a single string ("aud":"a") and a JSON array
// ("aud":["a","b"]) must both produce a valid Audience slice.
func TestParseClaims_HappyPathSingleAndMultiAudience(t *testing.T) {
	mk := func(claims string) string {
		return b64(`{"alg":"ES256"}`) + "." + b64(claims) + "." + b64("sig")
	}

	c1, err := ParseClaims(mk(`{"sub":"alice","aud":"single-aud","exp":9999999999}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c1.Audience) != 1 || c1.Audience[0] != "single-aud" {
		t.Errorf("single audience: %v", c1.Audience)
	}

	c2, err := ParseClaims(mk(`{"sub":"alice","aud":["a","b","c"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Audience) != 3 || c2.Audience[2] != "c" {
		t.Errorf("multi audience: %v", c2.Audience)
	}
}

// TestAudience_UnmarshalJSON_Garbage pins the failure branch when neither
// string nor []string fits — must surface the second Unmarshal error.
func TestAudience_UnmarshalJSON_Garbage(t *testing.T) {
	var a Audience
	if err := a.UnmarshalJSON([]byte(`{"x":1}`)); err == nil {
		t.Error("object accepted for Audience")
	}
	if err := a.UnmarshalJSON([]byte(`123`)); err == nil {
		t.Error("number accepted for Audience")
	}
}

// TestParseClaimsMap_HappyPath pins the success branch end-to-end.
func TestParseClaimsMap_HappyPath(t *testing.T) {
	tok := b64(`{"alg":"ES256"}`) + "." + b64(`{"sub":"x","custom":"v"}`) + "." + b64("sig")
	m, err := ParseClaimsMap(tok)
	if err != nil {
		t.Fatal(err)
	}
	if m["sub"] != "x" || m["custom"] != "v" {
		t.Errorf("map = %v", m)
	}
	// Verify it actually went through json (not magic).
	if _, err := json.Marshal(m); err != nil {
		t.Errorf("map not JSON-marshalable: %v", err)
	}
}
