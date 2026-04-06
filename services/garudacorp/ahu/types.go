package ahu

// CompanySearchRequest is the request payload for company search by SK number.
type CompanySearchRequest struct {
	SKNumber string `json:"sk_number"`
}

// CompanySearchResponse is the response from company search.
type CompanySearchResponse struct {
	Found       bool    `json:"found"`
	SKNumber    string  `json:"sk_number"`
	Name        string  `json:"name"`
	EntityType  string  `json:"entity_type"` // PT, CV, etc.
	Status      string  `json:"status"`
	NPWP        string  `json:"npwp"`
	Address     string  `json:"address"`
	CapitalAuth int64   `json:"capital_authorized"`
	CapitalPaid int64   `json:"capital_paid"`
	Message     string  `json:"message,omitempty"`
}

// Officer represents a company officer from AHU.
type Officer struct {
	NIK             string `json:"nik"`
	Name            string `json:"name"`
	Position        string `json:"position"` // DIREKTUR_UTAMA, DIREKTUR, KOMISARIS, etc.
	AppointmentDate string `json:"appointment_date"`
}

// Shareholder represents a company shareholder from AHU.
type Shareholder struct {
	Name       string  `json:"name"`
	ShareType  string  `json:"share_type"` // INDIVIDUAL, CORPORATE
	Shares     int64   `json:"shares"`
	Percentage float64 `json:"percentage"`
}

// OfficersResponse is the response from officers lookup.
type OfficersResponse struct {
	SKNumber string    `json:"sk_number"`
	Officers []Officer `json:"officers"`
}

// ShareholdersResponse is the response from shareholders lookup.
type ShareholdersResponse struct {
	SKNumber     string        `json:"sk_number"`
	Shareholders []Shareholder `json:"shareholders"`
}
