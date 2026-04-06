package embedinfo

import (
	"encoding/json"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()
	if info.Version == "" { t.Error("Version") }
	if info.GoVersion == "" { t.Error("GoVersion") }
}

func TestJSON(t *testing.T) {
	data, err := JSON()
	if err != nil { t.Fatal(err) }

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil { t.Fatal(err) }
	if m["version"] == "" { t.Error("version") }
}

func TestIsDev(t *testing.T) {
	old := Env
	defer func() { Env = old }()

	Env = "development"
	if !IsDev() { t.Error("development") }
	Env = "dev"
	if !IsDev() { t.Error("dev") }
	Env = "production"
	if IsDev() { t.Error("production should not be dev") }
}

func TestIsProduction(t *testing.T) {
	old := Env
	defer func() { Env = old }()

	Env = "production"
	if !IsProduction() { t.Error("production") }
	Env = "prod"
	if !IsProduction() { t.Error("prod") }
	Env = "dev"
	if IsProduction() { t.Error("dev should not be prod") }
}

func TestShort(t *testing.T) {
	old := Version
	oldC := Commit
	defer func() { Version = old; Commit = oldC }()

	Version = "1.0.0"
	Commit = "abc1234567890"
	if Short() != "1.0.0-abc1234" { t.Errorf("Short = %q", Short()) }

	Commit = "unknown"
	if Short() != "1.0.0" { t.Errorf("Short unknown = %q", Short()) }

	Commit = "abc"
	if Short() != "1.0.0" { t.Errorf("Short short = %q", Short()) }
}
