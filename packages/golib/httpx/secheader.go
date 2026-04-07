package httpx

import "net/http"

// SecurityHeaderOptions configures the SecurityHeaders middleware. Zero value
// is safe-by-default for backend JSON APIs (no UI rendering, no third-party
// frame embedding, no inline scripts).
type SecurityHeaderOptions struct {
	// HSTS controls Strict-Transport-Security. Set to true only when serving
	// over HTTPS in production — sending HSTS over plain HTTP is harmless to
	// browsers but tells operators something is wrong.
	HSTS bool
	// HSTSMaxAge in seconds. Defaults to 1 year if HSTS=true and zero.
	HSTSMaxAge int
	// HSTSIncludeSubdomains adds includeSubDomains to the HSTS header.
	HSTSIncludeSubdomains bool
	// HSTSPreload adds preload (only when includeSubdomains is also true and
	// MaxAge is at least 31536000 — caller's responsibility).
	HSTSPreload bool

	// ContentSecurityPolicy is set verbatim if non-empty. For JSON APIs the
	// safe default (used when this is empty) is "default-src 'none'".
	ContentSecurityPolicy string

	// FrameOptions defaults to "DENY". Set to "" to omit.
	FrameOptions string
}

// SecurityHeaders wraps h with a fixed set of defensive HTTP headers:
//   X-Content-Type-Options: nosniff       (always)
//   X-Frame-Options: DENY                 (default; configurable)
//   Referrer-Policy: no-referrer          (always — backend APIs leak nothing)
//   X-XSS-Protection: 0                   (always — modern browsers; legacy off)
//   Permissions-Policy: ()                (always — disables every feature)
//   Cross-Origin-Resource-Policy: same-origin
//   Strict-Transport-Security             (when opts.HSTS=true)
//   Content-Security-Policy               (default-src 'none' for JSON APIs)
//
// These are the OWASP Secure Headers Project minimum baseline for backend
// services. They cost zero per-request CPU and close several real attack
// classes (clickjacking, MIME sniffing, referer leaks, feature abuse).
func SecurityHeaders(h http.Handler, opts SecurityHeaderOptions) http.Handler {
	csp := opts.ContentSecurityPolicy
	if csp == "" {
		csp = "default-src 'none'; frame-ancestors 'none'"
	}
	frame := opts.FrameOptions
	if frame == "" {
		frame = "DENY"
	}

	hstsValue := ""
	if opts.HSTS {
		max := opts.HSTSMaxAge
		if max <= 0 {
			max = 31536000 // 1 year
		}
		hstsValue = "max-age=" + itoa(max)
		if opts.HSTSIncludeSubdomains {
			hstsValue += "; includeSubDomains"
			if opts.HSTSPreload {
				hstsValue += "; preload"
			}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("X-Content-Type-Options", "nosniff")
		if frame != "" {
			hdr.Set("X-Frame-Options", frame)
		}
		hdr.Set("Referrer-Policy", "no-referrer")
		hdr.Set("X-XSS-Protection", "0")
		hdr.Set("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")
		hdr.Set("Cross-Origin-Resource-Policy", "same-origin")
		hdr.Set("Content-Security-Policy", csp)
		if hstsValue != "" {
			hdr.Set("Strict-Transport-Security", hstsValue)
		}
		h.ServeHTTP(w, r)
	})
}

// itoa is a tiny stdlib-only int-to-string helper to avoid pulling strconv
// just for one call. Inlines well; avoids the strconv allocation cost.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
