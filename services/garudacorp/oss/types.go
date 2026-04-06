package oss

// NIBSearchRequest is the request payload for NIB search.
type NIBSearchRequest struct {
	NPWP string `json:"npwp,omitempty"`
	NIB  string `json:"nib,omitempty"`
}

// NIBSearchResponse is the response from NIB search.
type NIBSearchResponse struct {
	Found      bool       `json:"found"`
	NIB        string     `json:"nib"`
	NPWP       string     `json:"npwp"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Businesses []Business `json:"businesses"`
	Message    string     `json:"message,omitempty"`
}

// Business represents a business activity registered in OSS.
type Business struct {
	KBLI        string `json:"kbli"`
	Description string `json:"description"`
	Status      string `json:"status"`
	RiskLevel   string `json:"risk_level"` // LOW, MEDIUM, HIGH
}
