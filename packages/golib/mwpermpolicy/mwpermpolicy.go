// Package mwpermpolicy provides Permissions-Policy header middleware.
// Disables browser features by default to prevent fingerprinting and
// unauthorized API access. Implements W3C Permissions Policy.
package mwpermpolicy

import (
	"net/http"
	"strings"
)

// Policy holds permission directives.
type Policy struct {
	Camera        string // e.g. "()", "(self)", "(*)"
	Microphone    string
	Geolocation   string
	Payment       string
	USB           string
	Bluetooth     string
	Accelerometer string
	Gyroscope     string
	Magnetometer  string
	FullScreen    string
}

// DefaultPolicy disables all sensitive permissions.
func DefaultPolicy() Policy {
	return Policy{
		Camera:        "()",
		Microphone:    "()",
		Geolocation:   "()",
		Payment:       "()",
		USB:           "()",
		Bluetooth:     "()",
		Accelerometer: "()",
		Gyroscope:     "()",
		Magnetometer:  "()",
		FullScreen:    "()",
	}
}

// String builds the Permissions-Policy header value.
func (p Policy) String() string {
	var parts []string
	add := func(name, val string) {
		if val != "" {
			parts = append(parts, name+"="+val)
		}
	}
	add("camera", p.Camera)
	add("microphone", p.Microphone)
	add("geolocation", p.Geolocation)
	add("payment", p.Payment)
	add("usb", p.USB)
	add("bluetooth", p.Bluetooth)
	add("accelerometer", p.Accelerometer)
	add("gyroscope", p.Gyroscope)
	add("magnetometer", p.Magnetometer)
	add("fullscreen", p.FullScreen)
	return strings.Join(parts, ", ")
}

// Middleware returns a middleware that sets Permissions-Policy.
func Middleware(policy Policy) func(http.Handler) http.Handler {
	header := policy.String()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Permissions-Policy", header)
			next.ServeHTTP(w, r)
		})
	}
}

// Simple returns middleware with the default disable-all policy.
func Simple(next http.Handler) http.Handler {
	return Middleware(DefaultPolicy())(next)
}
