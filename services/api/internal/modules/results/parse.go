package results

import (
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// CSV column identifiers. The import header is matched case-insensitively and
// several common aliases map to the same logical column so timing-vendor
// exports work without manual re-heading.
const (
	colBib        = "bib"
	colName       = "name"
	colGender     = "gender"
	colAge        = "age"
	colAgeGroup   = "age_group"
	colStatus     = "status"
	colChipTime   = "chip_time"
	colGunTime    = "gun_time"
	colFinishedAt = "finished_at"
)

// columnAliases maps a normalized header cell to its logical column. Anything
// not present here is ignored.
var columnAliases = map[string]string{
	"bib":         colBib,
	"bib_number":  colBib,
	"bibnumber":   colBib,
	"no_bib":      colBib,
	"name":        colName,
	"nama":        colName,
	"participant": colName,
	"gender":      colGender,
	"sex":         colGender,
	"jenis_kelamin": colGender,
	"age":         colAge,
	"usia":        colAge,
	"umur":        colAge,
	"age_group":   colAgeGroup,
	"agegroup":    colAgeGroup,
	"kategori_usia": colAgeGroup,
	"status":      colStatus,
	"chip_time":   colChipTime,
	"chiptime":    colChipTime,
	"net_time":    colChipTime,
	"waktu_chip":  colChipTime,
	"gun_time":    colGunTime,
	"guntime":     colGunTime,
	"gross_time":  colGunTime,
	"waktu_gun":   colGunTime,
	"finished_at": colFinishedAt,
	"finish_time": colFinishedAt,
}

// indexColumns builds a logical-column → record-index map from the CSV header.
func indexColumns(header []string) map[string]int {
	cols := make(map[string]int, len(header))
	for i, cell := range header {
		key := normalizeHeader(cell)
		if logical, ok := columnAliases[key]; ok {
			if _, seen := cols[logical]; !seen {
				cols[logical] = i
			}
		}
	}
	return cols
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// normalizeGender maps free-form gender text to the M/F/X CHECK domain. Unknown
// values return "" (left NULL, excluded from gender ranking).
func normalizeGender(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "M", "MALE", "L", "LAKI", "LAKI-LAKI", "PRIA":
		return "M"
	case "F", "FEMALE", "P", "PEREMPUAN", "WANITA":
		return "F"
	case "X", "OTHER", "LAINNYA", "NB":
		return "X"
	default:
		return ""
	}
}

// normalizeStatus maps free-form status text to the FINISHED/DNF/DNS domain.
// Empty or unknown returns "" so the caller keeps its default (FINISHED).
func normalizeStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "FINISHED", "FINISH", "SELESAI", "OK", "DONE":
		return StatusFinished
	case "DNF", "DID NOT FINISH":
		return StatusDNF
	case "DNS", "DID NOT START":
		return StatusDNS
	default:
		return ""
	}
}

// parseDurationMs parses an elapsed time into milliseconds. Accepts either a
// raw millisecond integer or a clock string "H:MM:SS[.mmm]" / "MM:SS[.mmm]".
func parseDurationMs(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	// Raw integer milliseconds.
	if !strings.ContainsAny(s, ":") {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 0 {
			return 0, errors.New("invalid ms")
		}
		return n, nil
	}
	// Clock format: split off optional fractional seconds first.
	var fracMs int64
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		frac := s[dot+1:]
		s = s[:dot]
		// Pad/truncate to milliseconds (3 digits).
		for len(frac) < 3 {
			frac += "0"
		}
		frac = frac[:3]
		f, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, errors.New("invalid fraction")
		}
		fracMs = f
	}
	parts := strings.Split(s, ":")
	var h, m, sec int64
	var err error
	switch len(parts) {
	case 3:
		if h, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
			return 0, err
		}
		if m, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
			return 0, err
		}
		if sec, err = strconv.ParseInt(parts[2], 10, 64); err != nil {
			return 0, err
		}
	case 2:
		if m, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
			return 0, err
		}
		if sec, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
			return 0, err
		}
	default:
		return 0, errors.New("invalid clock format")
	}
	if h < 0 || m < 0 || m > 59 || sec < 0 || sec > 59 {
		return 0, errors.New("clock field out of range")
	}
	return ((h*3600+m*60+sec)*1000 + fracMs), nil
}

// --- certificate placeholders ---

// certificateSubstitutions builds the {{placeholder}} → value map for a
// finisher's certificate. Missing values render as an empty string.
func certificateSubstitutions(v ResultView) map[string]string {
	rank := ""
	if v.RankOverall != nil {
		rank = strconv.Itoa(*v.RankOverall)
	}
	category := ""
	if v.CategoryID != nil {
		category = v.AgeGroup // best available category label without a join
	}
	if v.AgeGroup != "" {
		category = v.AgeGroup
	}
	timeStr := v.ChipTime
	if timeStr == "" {
		timeStr = v.GunTime
	}
	return map[string]string{
		"name":     v.ParticipantName,
		"time":     timeStr,
		"rank":     rank,
		"category": category,
		"bib":      v.BibNumber,
	}
}

// applyPlaceholders replaces every {{key}} token in text with its substitution.
// Tokens with no matching key are left as-is so a typo is visible rather than
// silently blanked.
func applyPlaceholders(text string, subs map[string]string) string {
	if text == "" || !strings.Contains(text, "{{") {
		return text
	}
	for key, val := range subs {
		text = strings.ReplaceAll(text, "{{"+key+"}}", val)
	}
	return text
}

func pgTextValue(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

// parseInt32 parses a query-string integer, returning def on empty or invalid
// input. Negative values are clamped to def.
func parseInt32(s string, def int32) int32 {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return int32(n)
}
