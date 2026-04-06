package dukcapil

// NIKVerifyRequest is the request payload for NIK verification.
type NIKVerifyRequest struct {
	NIK string `json:"nik"`
}

// NIKVerifyResponse is the response from NIK verification.
type NIKVerifyResponse struct {
	Valid    bool   `json:"valid"`
	Alive   bool   `json:"alive"`
	Name    string `json:"name"`
	Message string `json:"message,omitempty"`
}

// DemographicRequest is the request payload for demographic verification.
type DemographicRequest struct {
	NIK         string `json:"nik"`
	Name        string `json:"name"`
	BirthDate   string `json:"birth_date"`
	BirthPlace  string `json:"birth_place"`
	MotherName  string `json:"mother_name"`
}

// DemographicResponse is the response from demographic verification.
type DemographicResponse struct {
	Match      bool    `json:"match"`
	Score      float64 `json:"score"`
	Message    string  `json:"message,omitempty"`
}

// BiometricRequest is the request payload for biometric (selfie) verification.
type BiometricRequest struct {
	NIK      string `json:"nik"`
	SelfieB64 string `json:"selfie_base64"`
}

// BiometricResponse is the response from biometric verification.
type BiometricResponse struct {
	Match      bool    `json:"match"`
	Confidence float64 `json:"confidence"`
	Message    string  `json:"message,omitempty"`
}
