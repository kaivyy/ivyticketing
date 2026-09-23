# Panduan Penambahan Vendor Timing Baru (How to Add a Timing Vendor)

* Document ID: `DOC-HOWTO-TIMING-VENDOR-2026-09-17`
* Target Audience: Backend Engineers & Hardware Integration Specialists
* Framework: IvyTicketing Vendor-Agnostic Timing Engine

---

## 1. Prinsip Utama (Zero Core Modification)

Saat menambahkan vendor timing baru (misal: MyLaps, ChronoTrack, Jaguar, atau sistem UHF RFID kustom), Anda **TIDAK PERLU** mengubah:
* Service nomor dada pelari ([`tickets.bib_service.go`](file:///root/ivyticketing/services/api/internal/modules/tickets/bib_service.go))
* Domain dan skema peserta (`tickets`, `orders`, `users`)
* Scoring processor ([`timing.Processor`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go))
* Ranking engine ([`recomputeRanks`](file:///root/ivyticketing/services/api/internal/modules/results/service.go#L128-L142))
* Mesin template dan generator e-sertifikat
* Halaman antarmuka pengguna hasil ([`results.astro`](file:///root/ivyticketing/apps/web/src/pages/org/[orgId]/events/[eventId]/results.astro))

Anda hanya perlu membuat **satu file adapter baru** di package [`services/api/internal/modules/results/timing/`](file:///root/ivyticketing/services/api/internal/modules/results/timing) yang mengimplementasikan capability interface yang dibutuhkan.

---

## 2. Langkah-Langkah Penambahan Vendor Baru

```
┌────────────────────────────────────────────────────────────┐
│              LANGKAH 1: TENTUKAN CAPABILITY                │
│ (PassingParser, ParticipantExporter, FinalResultParser)    │
└─────────────────────────────┬──────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────┐
│              LANGKAH 2: BUAT FILE ADAPTER BARU             │
│    services/api/internal/modules/results/timing/<vendor>.go│
└─────────────────────────────┬──────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────┐
│              LANGKAH 3: DAFTARKAN DI REGISTRY              │
│    services/api/internal/modules/results/timing/registry.go│
└─────────────────────────────┬──────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────┐
│              LANGKAH 4: BUAT UNIT & CONTRACT TEST          │
│    services/api/internal/modules/results/timing/<vendor>_t │
└────────────────────────────────────────────────────────────┘
```

---

## 3. Contoh Praktis: Membuat Adapter "VendorX"

Katakanlah Anda ingin menambahkan integrasi dengan **VendorX** yang mengirimkan stream deteksi via HTTP Push berformat baris teks:
`READ_ID,TAG_HEX,MAT_NAME,HH:MM:SS.mmm,YYYY-MM-DD`

### Langkah 1: Buat File Adapter
Buat file baru: `services/api/internal/modules/results/timing/vendorx.go`

```go
package timing

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"
)

type VendorXAdapter struct{}

func NewVendorXAdapter() *VendorXAdapter {
	return &VendorXAdapter{}
}

func (a *VendorXAdapter) ProviderName() string {
	return "VENDOR_X"
}

// Implementasi PassingParser: Mengurai data mentah menjadi []TimingPassing
func (a *VendorXAdapter) ParsePassings(_ context.Context, payload []byte, defaultPoint string) ([]TimingPassing, error) {
	lines := strings.Split(string(bytes.TrimSpace(payload)), "\n")
	out := make([]TimingPassing, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			continue
		}

		readID := strings.TrimSpace(parts[0])
		rawChip := strings.TrimSpace(parts[1])
		matName := strings.TrimSpace(parts[2])
		timeStr := strings.TrimSpace(parts[3])
		dateStr := strings.TrimSpace(parts[4])

		if matName == "" {
			matName = defaultPoint
		}

		obsTime, err := time.Parse("2006-01-02 15:04:05.999", dateStr+" "+timeStr)
		if err != nil {
			obsTime = time.Now().UTC()
		}

		// Normalisasi chip: uppercase tanpa spasi
		normChip := strings.ToUpper(rawChip)

		out = append(out, TimingPassing{
			ExternalID:      readID,
			CheckpointCode:  strings.ToUpper(matName),
			ChipCode:        normChip,
			RawChipCode:     rawChip,
			ObservedAt:      obsTime.UTC(),
			ReceivedAt:      time.Now().UTC(),
			SourceProvider:  "VENDOR_X",
			TransportMethod: "HTTP_PUSH",
			RawPayload:      line,
		})
	}

	return out, nil
}

// Implementasi ParticipantExporter: Memformat peserta IVY ke format konsol VendorX
func (a *VendorXAdapter) ExportParticipants(_ context.Context, participants []ParticipantRecord) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("BIB,NAME,CHIP,CONTEST\n")
	for _, p := range participants {
		sb.WriteString(p.BibNumber + "," + p.ParticipantName + "," + p.TransponderCode + "," + p.CategoryName + "\n")
	}
	return []byte(sb.String()), nil
}
```

---

### Langkah 2: Daftarkan Adapter ke Registry
Pada file `services/api/internal/modules/results/timing/registry.go`:

```go
func NewRegistry() *Registry {
	r := &Registry{
		parsers:   make(map[string]PassingParser),
		exporters: make(map[string]ParticipantExporter),
	}

	// Daftarkan provider bawaan
	r.RegisterPassingParser(NewRaceResultAdapter())
	r.RegisterParticipantExporter(NewRaceResultAdapter())
	r.RegisterPassingParser(NewCSVAdapter())

	// Daftarkan vendor baru di sini:
	r.RegisterPassingParser(NewVendorXAdapter())
	r.RegisterParticipantExporter(NewVendorXAdapter())

	return r
}
```

---

### Langkah 3: Tambahkan Unit Test
Buat file `services/api/internal/modules/results/timing/vendorx_test.go`:

```go
package timing

import (
	"context"
	"testing"
)

func TestVendorXAdapter_ParsePassings(t *testing.T) {
	adapter := NewVendorXAdapter()
	payload := []byte("1001,E2801105,FINISH,06:45:10.250,2026-09-17\n")

	passings, err := adapter.ParsePassings(context.Background(), payload, "FINISH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passings) != 1 {
		t.Fatalf("expected 1 passing, got %d", len(passings))
	}
	if passings[0].ExternalID != "1001" || passings[0].ChipCode != "E2801105" {
		t.Errorf("mismatched parsing output: %+v", passings[0])
	}
}
```

Jalankan test:
```bash
go test -v ./services/api/internal/modules/results/timing/...
```

---

## 4. Konfigurasi di Antarmuka Web

Setelah adapter didaftarkan, admin dapat langsung memilih `VENDOR_X` di:
1. **Admin Event Studio:** `/admin/events/edit?id=<eventId>` > Tab 10 (Timing & Race RFID) > Pilih Provider: `VENDOR_X`.
2. Salin URL Ingestion dan Ingestion Token.
3. Masukkan URL tersebut pada perangkat keras / software VendorX di lapangan.

Data deteksi yang dikirim oleh VendorX otomatis masuk ke staging buffer `timing_passings`, dievaluasi oleh generic processor, dan langsung memutakhirkan leaderboard resmi lomba tanpa mengubah satu baris pun kode inti sistem registrasi atau tiket.
