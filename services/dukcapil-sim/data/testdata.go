package data

// Person represents a Dukcapil population record.
type Person struct {
	NIK      string `json:"nik"`
	Name     string `json:"name"`
	DOB      string `json:"dob"`      // YYYY-MM-DD
	Gender   string `json:"gender"`   // "M" or "F"
	Province string `json:"province"` // province name
	City     string `json:"city"`
	Address  string `json:"address"`
	Alive    bool   `json:"alive"`
	PhotoB64 string `json:"photo_b64"` // base64-encoded photo stub
}

// NIK format: PPRRDDMMYY####
// PP = province code (2 digits)
// RR = regency/city code (2 digits)
// DD = day of birth (female: day + 40)
// MM = month of birth
// YY = year of birth (last 2 digits)
// #### = sequence number

// TestPeople contains synthetic Dukcapil records for development and testing.
var TestPeople = map[string]Person{
	"3201011501900001": {
		NIK:      "3201011501900001",
		Name:     "BUDI SANTOSO",
		DOB:      "1990-01-15",
		Gender:   "M",
		Province: "JAWA BARAT",
		City:     "KAB. BOGOR",
		Address:  "JL. RAYA BOGOR NO. 10 RT 001/002",
		Alive:    true,
		PhotoB64: "aVZCT1J3MEtHZ29BQUFBTlNVaEVVZ0FBQU1BQUFBREQ=", // stub
	},
	"3174015506850002": {
		NIK:      "3174015506850002",
		Name:     "SITI NURHALIZA",
		DOB:      "1985-06-15",
		Gender:   "F", // day = 15 + 40 = 55 in NIK
		Province: "DKI JAKARTA",
		City:     "KOTA JAKARTA SELATAN",
		Address:  "JL. SUDIRMAN NO. 25 RT 005/003",
		Alive:    true,
		PhotoB64: "UE5HSUhEUkRhdGFTdHViRm9yVGVzdA==",
	},
	"3507012003950003": {
		NIK:      "3507012003950003",
		Name:     "AGUS WIJAYA",
		DOB:      "1995-03-20",
		Gender:   "M",
		Province: "JAWA TIMUR",
		City:     "KAB. MALANG",
		Address:  "JL. KAWI NO. 7 RT 003/001",
		Alive:    true,
		PhotoB64: "R0lGODlhAQABAIAAAP///wAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==",
	},
	"5171014712880004": {
		NIK:      "5171014712880004",
		Name:     "NI MADE DEWI",
		DOB:      "1988-12-07",
		Gender:   "F", // day = 07 + 40 = 47 in NIK
		Province: "BALI",
		City:     "KOTA DENPASAR",
		Address:  "JL. SUNSET ROAD NO. 99 BR. KUTA",
		Alive:    true,
		PhotoB64: "Qk1GYWN0b3J5VGVzdEltYWdl",
	},
	"1271010110750005": {
		NIK:      "1271010110750005",
		Name:     "AHMAD LUBIS",
		DOB:      "1975-10-01",
		Gender:   "M",
		Province: "SUMATERA UTARA",
		City:     "KOTA MEDAN",
		Address:  "JL. GATOT SUBROTO NO. 88 KEL. MEDAN TIMUR",
		Alive:    true,
		PhotoB64: "VElGRlRlc3RTdHViSW1hZ2VEYXRh",
	},
	"3301016502600006": {
		NIK:      "3301016502600006",
		Name:     "SUHARTO WIBOWO",
		DOB:      "1960-02-25",
		Gender:   "M",
		Province: "JAWA TENGAH",
		City:     "KAB. CILACAP",
		Address:  "DSN. KARANGANYAR RT 002/001",
		Alive:    false, // deceased
		PhotoB64: "REVDRUFTRUREZGF0YVN0dWI=",
	},
}

// Lookup returns the person for the given NIK, or nil if not found.
func Lookup(nik string) *Person {
	p, ok := TestPeople[nik]
	if !ok {
		return nil
	}
	return &p
}
