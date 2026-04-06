package data

import "strings"

// Business represents an OSS/BKPM NIB record.
type Business struct {
	NIB         string `json:"nib"`
	NPWP        string `json:"npwp"`         // matches AHU company NPWP
	CompanyName string `json:"company_name"`
	KBLI        string `json:"kbli"`          // business activity code
	KBLIDesc    string `json:"kbli_desc"`
	Status      string `json:"status"`        // ACTIVE, REVOKED
	IssuedDate  string `json:"issued_date"`
	Address     string `json:"address"`
}

// TestBusinesses contains synthetic OSS/BKPM records for development and testing.
// NPWPs match the AHU simulator companies.
var TestBusinesses = []Business{
	{
		NIB:         "1234500001234",
		NPWP:        "01.234.567.8-012.000",
		CompanyName: "PT GARUDA TEKNOLOGI INDONESIA",
		KBLI:        "62011",
		KBLIDesc:    "AKTIVITAS PEMROGRAMAN KOMPUTER",
		Status:      "ACTIVE",
		IssuedDate:  "2020-04-01",
		Address:     "JL. SUDIRMAN KAV. 52-53, JAKARTA SELATAN",
	},
	{
		NIB:         "2345600002345",
		NPWP:        "02.345.678.9-023.000",
		CompanyName: "CV NUSANTARA DIGITAL",
		KBLI:        "62021",
		KBLIDesc:    "AKTIVITAS KONSULTASI KOMPUTER",
		Status:      "ACTIVE",
		IssuedDate:  "2021-08-15",
		Address:     "JL. KAWI NO. 7, MALANG, JAWA TIMUR",
	},
	{
		NIB:         "3456700003456",
		NPWP:        "03.456.789.0-034.000",
		CompanyName: "PT BALI SEJAHTERA",
		KBLI:        "55111",
		KBLIDesc:    "HOTEL BINTANG LIMA",
		Status:      "ACTIVE",
		IssuedDate:  "2019-12-10",
		Address:     "JL. SUNSET ROAD NO. 88, KUTA, BALI",
	},
	{
		NIB:         "5678900005678",
		NPWP:        "05.678.901.2-056.000",
		CompanyName: "PT MEDAN JAYA ABADI",
		KBLI:        "46100",
		KBLIDesc:    "PERDAGANGAN BESAR ATAS DASAR BALAS JASA (FEE) ATAU KONTRAK",
		Status:      "REVOKED",
		IssuedDate:  "2015-03-01",
		Address:     "JL. IMAM BONJOL NO. 15, MEDAN, SUMATERA UTARA",
	},
	{
		NIB:         "6789000006789",
		NPWP:        "06.789.012.3-067.000",
		CompanyName: "PT DIGITAL NUSANTARA PRIMA",
		KBLI:        "62011",
		KBLIDesc:    "AKTIVITAS PEMROGRAMAN KOMPUTER",
		Status:      "ACTIVE",
		IssuedDate:  "2022-02-20",
		Address:     "JL. THAMRIN NO. 1, JAKARTA PUSAT",
	},
}

// SearchByNPWP returns businesses matching the given NPWP.
func SearchByNPWP(npwp string) []Business {
	npwp = strings.TrimSpace(npwp)
	if npwp == "" {
		return nil
	}
	var results []Business
	for _, b := range TestBusinesses {
		if b.NPWP == npwp {
			results = append(results, b)
		}
	}
	return results
}

// SearchByNIB returns the business matching the given NIB, or nil if not found.
func SearchByNIB(nib string) *Business {
	nib = strings.TrimSpace(nib)
	if nib == "" {
		return nil
	}
	for _, b := range TestBusinesses {
		if b.NIB == nib {
			return &b
		}
	}
	return nil
}
