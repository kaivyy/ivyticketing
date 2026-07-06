package reporting

// Report type identifiers. These mirror the export_jobs.report_type CHECK
// constraint and the summary/rows query switch in service.go.
const (
	ReportParticipant = "participant"
	ReportSales       = "sales"
	ReportPayment     = "payment"
	ReportCoupon      = "coupon"
	ReportQueue       = "queue"
	ReportBallot      = "ballot"
	ReportRacepack    = "racepack"
	ReportRevenue     = "revenue"
)

// Export job status values (mirror export_jobs.status CHECK constraint).
const (
	JobStatusPending    = "PENDING"
	JobStatusProcessing = "PROCESSING"
	JobStatusReady      = "READY"
	JobStatusFailed     = "FAILED"
)

// FormatCSV is the only supported export format in Phase 16 (PDF summary is a
// frontend print-CSS concern, Excel is deferred).
const FormatCSV = "csv"

// validReportTypes gates CreateExportJob and GetSummary.
var validReportTypes = map[string]bool{
	ReportParticipant: true,
	ReportSales:       true,
	ReportPayment:     true,
	ReportCoupon:      true,
	ReportQueue:       true,
	ReportBallot:      true,
	ReportRacepack:    true,
	ReportRevenue:     true,
}

// IsValidReportType reports whether t is a recognized report type.
func IsValidReportType(t string) bool { return validReportTypes[t] }

// ExportJobResponse is the API shape for an export job.
type ExportJobResponse struct {
	ID          string  `json:"id"`
	ReportType  string  `json:"reportType"`
	Format      string  `json:"format"`
	Status      string  `json:"status"`
	RowCount    *int32  `json:"rowCount,omitempty"`
	FileURL     *string `json:"fileUrl,omitempty"`
	Error       *string `json:"error,omitempty"`
	EventID     *string `json:"eventId,omitempty"`
	RequestedBy string  `json:"requestedBy"`
	CreatedAt   string  `json:"createdAt"`
	CompletedAt *string `json:"completedAt,omitempty"`
}

// CreateExportRequest is the POST body for requesting an export.
type CreateExportRequest struct {
	ReportType string  `json:"reportType"`
	EventID    *string `json:"eventId"`
}
