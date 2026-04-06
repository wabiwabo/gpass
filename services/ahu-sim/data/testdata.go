package data

import "strings"

// Company represents an AHU/SABH legal entity record.
type Company struct {
	SKNumber    string        `json:"sk_number"`
	Name        string        `json:"name"`
	EntityType  string        `json:"entity_type"` // PT, CV, YAYASAN
	Status      string        `json:"status"`       // ACTIVE, DISSOLVED
	NPWP        string        `json:"npwp"`
	Address     string        `json:"address"`
	CapitalAuth int64         `json:"capital_authorized"`
	CapitalPaid int64         `json:"capital_paid"`
	Officers    []Officer     `json:"officers"`
	Shareholders []Shareholder `json:"shareholders"`
}

// Officer represents a company officer (direksi/komisaris).
type Officer struct {
	NIK             string `json:"nik"`
	Name            string `json:"name"`
	Position        string `json:"position"` // DIREKTUR_UTAMA, DIREKTUR, KOMISARIS, KETUA
	AppointmentDate string `json:"appointment_date"`
}

// Shareholder represents a company shareholder.
type Shareholder struct {
	Name       string  `json:"name"`
	ShareType  string  `json:"share_type"`
	Shares     int64   `json:"shares"`
	Percentage float64 `json:"percentage"`
}

// TestCompanies contains synthetic AHU/SABH records for development and testing.
// Officer NIKs match the dukcapil-sim test data.
var TestCompanies = map[string]Company{
	"AHU-0001234.AH.01.01": {
		SKNumber:    "AHU-0001234.AH.01.01",
		Name:        "PT GARUDA TEKNOLOGI INDONESIA",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "01.234.567.8-012.000",
		Address:     "JL. SUDIRMAN KAV. 52-53, JAKARTA SELATAN",
		CapitalAuth: 10000000000,
		CapitalPaid: 2500000000,
		Officers: []Officer{
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "DIREKTUR_UTAMA", AppointmentDate: "2020-03-15"},
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "KOMISARIS", AppointmentDate: "2020-03-15"},
		},
		Shareholders: []Shareholder{
			{Name: "BUDI SANTOSO", ShareType: "SAHAM BIASA", Shares: 1500, Percentage: 60.0},
			{Name: "SITI NURHALIZA", ShareType: "SAHAM BIASA", Shares: 1000, Percentage: 40.0},
		},
	},
	"AHU-0002345.AH.01.01": {
		SKNumber:    "AHU-0002345.AH.01.01",
		Name:        "CV NUSANTARA DIGITAL",
		EntityType:  "CV",
		Status:      "ACTIVE",
		NPWP:        "02.345.678.9-023.000",
		Address:     "JL. KAWI NO. 7, MALANG, JAWA TIMUR",
		CapitalAuth: 500000000,
		CapitalPaid: 500000000,
		Officers: []Officer{
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "DIREKTUR_UTAMA", AppointmentDate: "2021-07-01"},
		},
		Shareholders: []Shareholder{
			{Name: "AGUS WIJAYA", ShareType: "MODAL DASAR", Shares: 500, Percentage: 100.0},
		},
	},
	"AHU-0003456.AH.01.01": {
		SKNumber:    "AHU-0003456.AH.01.01",
		Name:        "PT BALI SEJAHTERA",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "03.456.789.0-034.000",
		Address:     "JL. SUNSET ROAD NO. 88, KUTA, BALI",
		CapitalAuth: 5000000000,
		CapitalPaid: 1250000000,
		Officers: []Officer{
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "DIREKTUR_UTAMA", AppointmentDate: "2019-11-20"},
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "DIREKTUR", AppointmentDate: "2019-11-20"},
		},
		Shareholders: []Shareholder{
			{Name: "NI MADE DEWI", ShareType: "SAHAM BIASA", Shares: 3000, Percentage: 60.0},
			{Name: "AHMAD LUBIS", ShareType: "SAHAM BIASA", Shares: 2000, Percentage: 40.0},
		},
	},
	"AHU-0004567.AH.01.01": {
		SKNumber:    "AHU-0004567.AH.01.01",
		Name:        "YAYASAN PEDULI BANGSA",
		EntityType:  "YAYASAN",
		Status:      "ACTIVE",
		NPWP:        "04.567.890.1-045.000",
		Address:     "JL. GATOT SUBROTO NO. 10, MEDAN, SUMATERA UTARA",
		CapitalAuth: 1000000000,
		CapitalPaid: 1000000000,
		Officers: []Officer{
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "KETUA", AppointmentDate: "2018-05-10"},
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "SEKRETARIS", AppointmentDate: "2018-05-10"},
		},
		Shareholders: nil,
	},
	"AHU-0005678.AH.01.01": {
		SKNumber:    "AHU-0005678.AH.01.01",
		Name:        "PT MEDAN JAYA ABADI",
		EntityType:  "PT",
		Status:      "DISSOLVED",
		NPWP:        "05.678.901.2-056.000",
		Address:     "JL. IMAM BONJOL NO. 15, MEDAN, SUMATERA UTARA",
		CapitalAuth: 2000000000,
		CapitalPaid: 500000000,
		Officers: []Officer{
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "DIREKTUR_UTAMA", AppointmentDate: "2015-01-20"},
		},
		Shareholders: []Shareholder{
			{Name: "AHMAD LUBIS", ShareType: "SAHAM BIASA", Shares: 2000, Percentage: 100.0},
		},
	},
	"AHU-0006789.AH.01.01": {
		SKNumber:    "AHU-0006789.AH.01.01",
		Name:        "PT DIGITAL NUSANTARA PRIMA",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "06.789.012.3-067.000",
		Address:     "JL. THAMRIN NO. 1, JAKARTA PUSAT",
		CapitalAuth: 50000000000,
		CapitalPaid: 12500000000,
		Officers: []Officer{
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "DIREKTUR_UTAMA", AppointmentDate: "2022-01-10"},
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "DIREKTUR", AppointmentDate: "2022-01-10"},
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "KOMISARIS", AppointmentDate: "2022-01-10"},
		},
		Shareholders: []Shareholder{
			{Name: "SITI NURHALIZA", ShareType: "SAHAM BIASA", Shares: 5000, Percentage: 50.0},
			{Name: "AGUS WIJAYA", ShareType: "SAHAM BIASA", Shares: 3000, Percentage: 30.0},
			{Name: "NI MADE DEWI", ShareType: "SAHAM BIASA", Shares: 2000, Percentage: 20.0},
		},
	},
}

// LookupBySK returns the company for the given SK number, or nil if not found.
func LookupBySK(sk string) *Company {
	c, ok := TestCompanies[sk]
	if !ok {
		return nil
	}
	return &c
}

// SearchByName returns companies whose name contains the query (case-insensitive).
func SearchByName(query string) []Company {
	query = strings.ToUpper(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	var results []Company
	for _, c := range TestCompanies {
		if strings.Contains(c.Name, query) {
			results = append(results, c)
		}
	}
	return results
}
