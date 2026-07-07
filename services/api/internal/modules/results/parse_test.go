package results

import "testing"

func TestParseDurationMs(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"3600000", 3600000, false},       // raw ms
		{"0", 0, false},                   // zero ms
		{"1:23:45", 5025000, false},       // H:MM:SS
		{"1:23:45.500", 5025500, false},   // fractional seconds
		{"25:30", 1530000, false},         // MM:SS
		{"0:00:01", 1000, false},          // one second
		{"2:00:00", 7200000, false},       // two hours
		{"1:05:09", 3909000, false},       // padded fields
		{"1:23:45.5", 5025500, false},     // single-digit fraction padded
		{"1:23:45.123", 5025123, false},   // full ms fraction
		{"1:23:45.9999", 5025999, false},  // fraction truncated to 3 digits
		{"", 0, true},                     // empty
		{"-100", 0, true},                 // negative ms
		{"abc", 0, true},                  // non-numeric
		{"1:60:00", 0, true},              // minutes out of range
		{"1:00:60", 0, true},              // seconds out of range
		{"1:2:3:4", 0, true},              // too many clock fields
		{"1:aa:00", 0, true},              // non-numeric field
	}
	for _, c := range cases {
		got, err := parseDurationMs(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseDurationMs(%q) = %d, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDurationMs(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseDurationMs(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNormalizeGender(t *testing.T) {
	cases := map[string]string{
		"M": "M", "male": "M", "L": "M", "LAKI": "M", "Laki-laki": "M", "pria": "M",
		"F": "F", "female": "F", "P": "F", "perempuan": "F", "Wanita": "F",
		"X": "X", "other": "X", "lainnya": "X", "NB": "X",
		"": "", "unknown": "", "123": "",
	}
	for in, want := range cases {
		if got := normalizeGender(in); got != want {
			t.Errorf("normalizeGender(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"finished": StatusFinished, "FINISH": StatusFinished, "selesai": StatusFinished, "OK": StatusFinished, "done": StatusFinished,
		"dnf": StatusDNF, "did not finish": StatusDNF,
		"dns": StatusDNS, "did not start": StatusDNS,
		"": "", "wat": "",
	}
	for in, want := range cases {
		if got := normalizeStatus(in); got != want {
			t.Errorf("normalizeStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeHeader(t *testing.T) {
	cases := map[string]string{
		"  BIB ":         "bib",
		"Chip Time":      "chip_time",
		"jenis-kelamin":  "jenis_kelamin",
		"Age Group":      "age_group",
	}
	for in, want := range cases {
		if got := normalizeHeader(in); got != want {
			t.Errorf("normalizeHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIndexColumns(t *testing.T) {
	header := []string{"No BIB", "Nama", "Jenis Kelamin", "Waktu Chip", "unknown_col", "status"}
	cols := indexColumns(header)

	want := map[string]int{
		colBib:      0,
		colName:     1,
		colGender:   2,
		colChipTime: 3,
		colStatus:   5,
	}
	for logical, idx := range want {
		if got, ok := cols[logical]; !ok || got != idx {
			t.Errorf("indexColumns[%q] = %d (ok=%v), want %d", logical, got, ok, idx)
		}
	}
	if _, ok := cols[colGunTime]; ok {
		t.Error("expected no gun_time column mapping")
	}
}

func TestIndexColumnsFirstWins(t *testing.T) {
	// Two aliases for the same logical column: the first occurrence must win.
	header := []string{"bib", "bib_number"}
	cols := indexColumns(header)
	if cols[colBib] != 0 {
		t.Errorf("expected first bib alias to win (index 0), got %d", cols[colBib])
	}
}

func TestApplyPlaceholders(t *testing.T) {
	subs := map[string]string{"name": "Budi", "time": "1:23:45", "rank": "5"}

	if got := applyPlaceholders("Halo {{name}}, waktu {{time}}", subs); got != "Halo Budi, waktu 1:23:45" {
		t.Errorf("applyPlaceholders substitution = %q", got)
	}
	// Unknown token is left visible rather than blanked.
	if got := applyPlaceholders("{{name}} {{unknown}}", subs); got != "Budi {{unknown}}" {
		t.Errorf("applyPlaceholders unknown token = %q, want it preserved", got)
	}
	// No tokens: returned unchanged.
	if got := applyPlaceholders("plain text", subs); got != "plain text" {
		t.Errorf("applyPlaceholders no-token = %q", got)
	}
	if got := applyPlaceholders("", subs); got != "" {
		t.Errorf("applyPlaceholders empty = %q", got)
	}
}

func TestParseInt32(t *testing.T) {
	cases := []struct {
		in   string
		def  int32
		want int32
	}{
		{"", 100, 100},
		{"50", 100, 50},
		{"0", 100, 0},
		{"-5", 100, 100}, // negative clamps to default
		{"abc", 100, 100},
		{"250", 100, 250},
	}
	for _, c := range cases {
		if got := parseInt32(c.in, c.def); got != c.want {
			t.Errorf("parseInt32(%q, %d) = %d, want %d", c.in, c.def, got, c.want)
		}
	}
}
