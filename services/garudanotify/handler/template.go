package handler

import (
	"fmt"
	"strings"
)

// Template represents a notification template with Indonesian language support.
type Template struct {
	Name      string   `json:"name"`
	SubjectEN string   `json:"subject"`
	BodyEN    string   `json:"body"`
	SubjectID string   `json:"subject_id"`
	BodyID    string   `json:"body_id"`
	Variables []string `json:"variables"`
}

// TemplateStore holds predefined notification templates.
type TemplateStore struct {
	templates map[string]Template
}

// NewTemplateStore creates a TemplateStore with all predefined templates.
func NewTemplateStore() *TemplateStore {
	ts := &TemplateStore{
		templates: make(map[string]Template),
	}

	ts.templates["otp"] = Template{
		Name:      "otp",
		SubjectEN: "Your GarudaPass Verification Code",
		BodyEN:    "Your verification code is: {{code}}. This code expires in {{expiry}} minutes. Do not share it with anyone.",
		SubjectID: "Kode Verifikasi GarudaPass Anda",
		BodyID:    "Kode verifikasi Anda adalah: {{code}}. Kode ini berlaku selama {{expiry}} menit. Jangan bagikan kepada siapa pun.",
		Variables: []string{"code", "expiry"},
	}

	ts.templates["welcome"] = Template{
		Name:      "welcome",
		SubjectEN: "Welcome to GarudaPass",
		BodyEN:    "Hello {{name}}, welcome to GarudaPass! Your digital identity account has been created successfully.",
		SubjectID: "Selamat Datang di GarudaPass",
		BodyID:    "Halo {{name}}, selamat datang di GarudaPass! Akun identitas digital Anda telah berhasil dibuat.",
		Variables: []string{"name"},
	}

	ts.templates["password_reset"] = Template{
		Name:      "password_reset",
		SubjectEN: "GarudaPass Password Reset",
		BodyEN:    "Hello {{name}}, use the following link to reset your password: {{link}}. This link expires in {{expiry}} minutes.",
		SubjectID: "Reset Kata Sandi GarudaPass",
		BodyID:    "Halo {{name}}, gunakan tautan berikut untuk mereset kata sandi Anda: {{link}}. Tautan ini berlaku selama {{expiry}} menit.",
		Variables: []string{"name", "link", "expiry"},
	}

	ts.templates["security_alert"] = Template{
		Name:      "security_alert",
		SubjectEN: "GarudaPass Security Alert",
		BodyEN:    "A {{event}} was detected on your account from {{location}} at {{time}}. If this was not you, please secure your account immediately.",
		SubjectID: "Peringatan Keamanan GarudaPass",
		BodyID:    "Terdeteksi {{event}} pada akun Anda dari {{location}} pada {{time}}. Jika ini bukan Anda, segera amankan akun Anda.",
		Variables: []string{"event", "location", "time"},
	}

	ts.templates["consent_granted"] = Template{
		Name:      "consent_granted",
		SubjectEN: "GarudaPass Consent Confirmation",
		BodyEN:    "You have granted {{service}} access to your {{data_type}} data. This consent is valid until {{expiry}}.",
		SubjectID: "Konfirmasi Persetujuan GarudaPass",
		BodyID:    "Anda telah memberikan {{service}} akses ke data {{data_type}} Anda. Persetujuan ini berlaku hingga {{expiry}}.",
		Variables: []string{"service", "data_type", "expiry"},
	}

	ts.templates["data_deletion"] = Template{
		Name:      "data_deletion",
		SubjectEN: "GarudaPass Data Deletion Confirmation",
		BodyEN:    "Your {{data_type}} data has been permanently deleted as requested on {{date}}. Reference ID: {{reference_id}}.",
		SubjectID: "Konfirmasi Penghapusan Data GarudaPass",
		BodyID:    "Data {{data_type}} Anda telah dihapus secara permanen sesuai permintaan pada {{date}}. ID Referensi: {{reference_id}}.",
		Variables: []string{"data_type", "date", "reference_id"},
	}

	return ts
}

// RenderTemplate renders a template by name with the given data.
// If lang is "id", the Indonesian version is rendered; otherwise English.
func (ts *TemplateStore) RenderTemplate(name string, data map[string]string) (subject, body string, err error) {
	return ts.RenderTemplateWithLang(name, "en", data)
}

// RenderTemplateWithLang renders a template with a specific language ("en" or "id").
func (ts *TemplateStore) RenderTemplateWithLang(name, lang string, data map[string]string) (subject, body string, err error) {
	tmpl, ok := ts.templates[name]
	if !ok {
		return "", "", fmt.Errorf("template %q not found", name)
	}

	if lang == "id" {
		subject = tmpl.SubjectID
		body = tmpl.BodyID
	} else {
		subject = tmpl.SubjectEN
		body = tmpl.BodyEN
	}

	// Replace all variables
	for _, v := range tmpl.Variables {
		placeholder := "{{" + v + "}}"
		val, exists := data[v]
		if !exists {
			return "", "", fmt.Errorf("missing required variable %q for template %q", v, name)
		}
		subject = strings.ReplaceAll(subject, placeholder, val)
		body = strings.ReplaceAll(body, placeholder, val)
	}

	return subject, body, nil
}

// ListTemplates returns all available template names and metadata.
func (ts *TemplateStore) ListTemplates() []Template {
	result := make([]Template, 0, len(ts.templates))
	for _, tmpl := range ts.templates {
		result = append(result, tmpl)
	}
	return result
}
