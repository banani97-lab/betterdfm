package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"

	"github.com/betterdfm/api/src/db"
	"github.com/fogleman/gg"
	"github.com/go-pdf/fpdf"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ReportHandler serves GET /jobs/:id/report.pdf
type ReportHandler struct {
	db *gorm.DB
}

func NewReportHandler(database *gorm.DB) *ReportHandler {
	return &ReportHandler{db: database}
}

type rptBoardPoint struct {
	X, Y float64
}

type rptBoardData struct {
	Outline []rptBoardPoint `json:"outline"`
}

// severityRGBf returns violation severity as [0,1] RGB for gg.
func severityRGBf(sev string) (float64, float64, float64) {
	switch sev {
	case "ERROR":
		return 0.87, 0.20, 0.20
	case "WARNING":
		return 0.85, 0.47, 0.05
	default: // INFO
		return 0.13, 0.47, 0.71
	}
}

// severityRGBi returns violation severity as [0,255] RGB for fpdf.
func severityRGBi(sev string) (int, int, int) {
	r, g, b := severityRGBf(sev)
	return int(r * 255), int(g * 255), int(b * 255)
}

// scoreBarColor returns an RGB color for a score (green→yellow→orange→red).
func scoreBarColor(score int) (int, int, int) {
	switch {
	case score >= 90:
		return 22, 163, 74 // green-600
	case score >= 75:
		return 202, 138, 4 // yellow-600
	case score >= 60:
		return 234, 88, 12 // orange-600
	default:
		return 220, 38, 38 // red-600
	}
}

// rptRuleWeight mirrors the engine scoring weights (inlined to avoid cross-module import).
func rptRuleWeight(id string) float64 {
	switch id {
	case "clearance":
		return 3.0
	case "trace-width":
		return 2.5
	case "annular-ring":
		return 2.5
	case "drill-size":
		return 2.0
	case "aspect-ratio":
		return 1.5
	case "edge-clearance":
		return 1.5
	case "solder-mask-dam":
		return 1.0
	default:
		return 1.0
	}
}

func rptSeverityWeight(sev string) float64 {
	switch sev {
	case "ERROR":
		return 10.0
	case "WARNING":
		return 3.0
	case "INFO":
		return 0.5
	default:
		return 1.0
	}
}

func rptRuleMaxContribution(id string) float64 {
	switch id {
	case "clearance":
		return 35.0
	case "trace-width":
		return 30.0
	case "annular-ring":
		return 20.0
	case "drill-size":
		return 15.0
	case "aspect-ratio":
		return 10.0
	case "edge-clearance":
		return 10.0
	case "solder-mask-dam":
		return 10.0
	default:
		return 10.0
	}
}

func rptMarginMult(v db.Violation) float64 {
	if v.Unit == "ratio" {
		if v.LimitMM == 0 {
			return 1.5
		}
		ratio := v.MeasuredMM / v.LimitMM
		if ratio <= 0 {
			return 1.5
		}
		if ratio < 0.5 {
			return 1.25
		}
		return 1.0
	}
	if v.MeasuredMM == 0 {
		return 1.5
	}
	if v.LimitMM > 0 && v.MeasuredMM < 0.5*v.LimitMM {
		return 1.25
	}
	return 1.0
}

type reportScoreResult struct {
	score            int
	grade            string
	verdict          string
	byRule           map[string]float64
	byRuleCount      map[string]int
	areaCM2          float64
	violationDensity float64
}

// computeReportScore replicates the engine scoring formula inline.
func computeReportScore(violations []db.Violation, board rptBoardData) reportScoreResult {
	byRule := make(map[string]float64)
	byRuleCount := make(map[string]int)

	for _, v := range violations {
		p := rptRuleWeight(v.RuleID) * rptSeverityWeight(v.Severity) * rptMarginMult(v)
		byRule[v.RuleID] += p
		byRuleCount[v.RuleID]++
	}

	// Bounding box from outline
	var bboxW, bboxH float64
	if len(board.Outline) > 0 {
		minX, minY := board.Outline[0].X, board.Outline[0].Y
		maxX, maxY := board.Outline[0].X, board.Outline[0].Y
		for _, p := range board.Outline[1:] {
			if p.X < minX {
				minX = p.X
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
		bboxW = maxX - minX
		bboxH = maxY - minY
	}
	areaCM2 := bboxW * bboxH / 100.0
	areaFactor := math.Sqrt(math.Max(1.0, areaCM2))

	var pNorm float64
	for ruleID, rawPenalty := range byRule {
		rawNorm := rawPenalty / areaFactor
		cap := rptRuleMaxContribution(ruleID)
		if rawNorm > cap {
			rawNorm = cap
		}
		pNorm += rawNorm
	}

	rawScore := 100.0 - pNorm
	score := int(math.Round(math.Max(0, math.Min(100, rawScore))))

	density := 0.0
	if areaCM2 > 0 {
		density = float64(len(violations)) / areaCM2
	}

	var grade, verdict string
	switch {
	case score >= 90:
		grade, verdict = "A", "Production Ready — no significant issues"
	case score >= 75:
		grade, verdict = "B", "Minor Issues — review recommended before submission"
	case score >= 60:
		grade, verdict = "C", "Moderate Issues — rework required"
	case score >= 40:
		grade, verdict = "D", "Significant Issues — major redesign required"
	default:
		grade, verdict = "F", "Not Manufacturable — critical failures present"
	}

	return reportScoreResult{
		score:            score,
		grade:            grade,
		verdict:          verdict,
		byRule:           byRule,
		byRuleCount:      byRuleCount,
		areaCM2:          areaCM2,
		violationDensity: density,
	}
}

// renderBoardImage draws board outline + violation dots and returns a PNG blob.
func renderBoardImage(board rptBoardData, violations []db.Violation) []byte {
	const (
		W   = 800.0
		H   = 600.0
		pad = 40.0
	)

	dc := gg.NewContext(int(W), int(H))
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// Compute bounding box from outline + violation positions
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, pt := range board.Outline {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}
	for _, v := range violations {
		if v.X < minX {
			minX = v.X
		}
		if v.X > maxX {
			maxX = v.X
		}
		if v.Y < minY {
			minY = v.Y
		}
		if v.Y > maxY {
			maxY = v.Y
		}
	}
	if minX == math.MaxFloat64 {
		minX, minY, maxX, maxY = 0, 0, 100, 100
	}

	rx := maxX - minX
	ry := maxY - minY
	if rx == 0 {
		rx = 1
	}
	if ry == 0 {
		ry = 1
	}

	// Uniform scale + centering to preserve board aspect ratio
	scale := math.Min((W-2*pad)/rx, (H-2*pad)/ry)
	offX := (W - rx*scale) / 2
	offY := (H - ry*scale) / 2

	tx := func(x float64) float64 { return (x-minX)*scale + offX }
	ty := func(y float64) float64 { return (y-minY)*scale + offY }

	// Board: light green fill then dark green border
	if len(board.Outline) >= 2 {
		dc.SetRGB(0.88, 0.95, 0.88)
		dc.MoveTo(tx(board.Outline[0].X), ty(board.Outline[0].Y))
		for _, pt := range board.Outline[1:] {
			dc.LineTo(tx(pt.X), ty(pt.Y))
		}
		dc.ClosePath()
		dc.Fill()

		dc.SetRGB(0.13, 0.40, 0.13)
		dc.SetLineWidth(2)
		dc.MoveTo(tx(board.Outline[0].X), ty(board.Outline[0].Y))
		for _, pt := range board.Outline[1:] {
			dc.LineTo(tx(pt.X), ty(pt.Y))
		}
		dc.ClosePath()
		dc.Stroke()
	}

	// Violation dots: white outer ring + colored inner fill
	for _, v := range violations {
		r, g, b := severityRGBf(v.Severity)
		dc.SetRGB(1, 1, 1)
		dc.DrawCircle(tx(v.X), ty(v.Y), 8)
		dc.Fill()
		dc.SetRGB(r, g, b)
		dc.DrawCircle(tx(v.X), ty(v.Y), 5.5)
		dc.Fill()
	}

	var buf bytes.Buffer
	_ = dc.EncodePNG(&buf)
	return buf.Bytes()
}

// trunc shortens s to at most n runes, appending "..." if cut.
func trunc(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-3]) + "..."
}

// GetJobReport handles GET /jobs/:id/report.pdf
func (h *ReportHandler) GetJobReport(c echo.Context) error {
	jobID := c.Param("id")

	var job db.AnalysisJob
	if err := h.db.First(&job, "id = ?", jobID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	var violations []db.Violation
	h.db.Where("job_id = ? AND ignored = false", jobID).Find(&violations)

	var ignoredViolations []db.Violation
	h.db.Where("job_id = ? AND ignored = true", jobID).Find(&ignoredViolations)

	var submission db.Submission
	h.db.First(&submission, "id = ?", job.SubmissionID)

	var profile db.CapabilityProfile
	h.db.First(&profile, "id = ?", job.ProfileID)

	var rules db.ProfileRules
	_ = json.Unmarshal(profile.Rules, &rules)

	var board rptBoardData
	_ = json.Unmarshal(job.BoardData, &board)

	// ── Compute score ────────────────────────────────────────────────────────
	sr := computeReportScore(violations, board)

	// Violation counts by severity
	errCount, warnCount, infoCount := 0, 0, 0
	for _, v := range violations {
		switch v.Severity {
		case "ERROR":
			errCount++
		case "WARNING":
			warnCount++
		case "INFO":
			infoCount++
		}
	}

	// Build PDF
	f := fpdf.New("P", "mm", "A4", "")
	f.SetMargins(10, 10, 10)
	f.SetAutoPageBreak(true, 10)

	tr := f.UnicodeTranslatorFromDescriptor("")
	const pW = 190.0 // usable width (210mm − 2×10mm margins)

	// ════════════════════════════════════════════════════════════════════════
	// PAGE 1: Executive Summary
	// ════════════════════════════════════════════════════════════════════════
	f.AddPage()

	// ── Header bar ───────────────────────────────────────────────────────────
	f.SetFillColor(22, 101, 52) // green-800
	f.SetTextColor(255, 255, 255)
	f.SetFont("Arial", "B", 15)
	f.CellFormat(pW, 12, "  DFM Analysis Report", "", 0, "L", true, 0, "")
	f.SetXY(10, f.GetY())
	f.SetFont("Arial", "", 9)
	dateStr := ""
	if job.CompletedAt != nil {
		dateStr = job.CompletedAt.UTC().Format("02 Jan 2006  15:04 UTC") + "  "
	}
	f.CellFormat(pW, 12, dateStr, "", 1, "R", false, 0, "")

	// ── Subheader ─────────────────────────────────────────────────────────────
	f.SetFillColor(240, 240, 240)
	f.SetTextColor(60, 60, 60)
	f.SetFont("Arial", "", 8)
	f.CellFormat(pW/2, 6, tr("  File: "+trunc(submission.Filename, 48)), "", 0, "L", true, 0, "")
	f.CellFormat(pW/4, 6, "Type: "+submission.FileType, "", 0, "L", true, 0, "")
	f.CellFormat(pW/4, 6, tr("Profile: "+trunc(profile.Name, 22)), "", 1, "L", true, 0, "")

	f.Ln(5)

	// ── Score + Design Summary (two columns) ─────────────────────────────────
	const colL = 95.0
	const colR = 95.0
	sectionY := f.GetY()

	// Left column: score bar + grade
	f.SetXY(10, sectionY)
	f.SetFont("Arial", "B", 9)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(colL, 6, "MANUFACTURABILITY SCORE", "", 1, "L", false, 0, "")
	f.SetX(10)
	f.Ln(2)

	// Score bar (filled + empty)
	const barW = 88.0
	const barH = 10.0
	barX := 10.0
	barY := f.GetY()
	fillW := barW * float64(sr.score) / 100.0
	sr1, sg1, sb1 := scoreBarColor(sr.score)
	f.SetFillColor(sr1, sg1, sb1)
	f.Rect(barX, barY, fillW, barH, "F")
	f.SetFillColor(220, 220, 220)
	f.Rect(barX+fillW, barY, barW-fillW, barH, "F")

	// Score text below bar
	f.SetXY(10, barY+barH+2)
	f.SetFont("Arial", "B", 10)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(barW, 6, fmt.Sprintf("%d / 100", sr.score), "", 1, "L", false, 0, "")

	// Grade letter (large)
	f.SetX(10)
	f.Ln(2)
	f.SetFont("Arial", "B", 28)
	f.SetTextColor(sr1, sg1, sb1)
	f.CellFormat(20, 12, sr.grade, "", 0, "L", false, 0, "")
	f.SetFont("Arial", "", 9)
	f.SetTextColor(80, 80, 80)
	f.CellFormat(colL-20, 12, tr(sr.verdict), "", 1, "L", false, 0, "")

	// Right column: design summary
	f.SetXY(10+colL, sectionY)
	f.SetFont("Arial", "B", 9)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(colR, 6, "Design Summary", "", 1, "L", false, 0, "")
	f.SetX(10 + colL)
	f.Ln(2)

	type summaryRow struct{ label string; val string }
	summaryRows := []summaryRow{
		{"Total Violations:", fmt.Sprintf("%d", len(violations))},
		{"Errors:", fmt.Sprintf("%d", errCount)},
		{"Warnings:", fmt.Sprintf("%d", warnCount)},
		{"Info:", fmt.Sprintf("%d", infoCount)},
		{"Acknowledged/Waived:", fmt.Sprintf("%d", len(ignoredViolations))},
		{"Board Area:", fmt.Sprintf("%.1f cm\u00b2", sr.areaCM2)},
		{"Violation Density:", fmt.Sprintf("%.1f /cm\u00b2", sr.violationDensity)},
	}
	for _, row := range summaryRows {
		f.SetX(10 + colL)
		f.SetFont("Arial", "B", 8)
		f.SetTextColor(60, 60, 60)
		f.CellFormat(45, 5.5, tr(row.label), "", 0, "L", false, 0, "")
		f.SetFont("Arial", "", 8)
		f.SetTextColor(30, 30, 30)
		f.CellFormat(colR-45, 5.5, tr(row.val), "", 1, "R", false, 0, "")
	}

	// Advance below both columns
	f.Ln(6)

	// ── Score Breakdown by Rule ───────────────────────────────────────────────
	f.SetFont("Arial", "B", 9)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(pW, 6, "Score Breakdown by Rule", "", 1, "L", false, 0, "")
	f.Ln(2)

	// Sort rules by penalty descending
	type ruleEntry struct {
		id      string
		penalty float64
		count   int
	}
	var ruleEntries []ruleEntry
	for id, penalty := range sr.byRule {
		ruleEntries = append(ruleEntries, ruleEntry{id: id, penalty: penalty, count: sr.byRuleCount[id]})
	}
	sort.Slice(ruleEntries, func(i, j int) bool {
		return ruleEntries[i].penalty > ruleEntries[j].penalty
	})

	maxPenalty := 0.0
	if len(ruleEntries) > 0 {
		maxPenalty = ruleEntries[0].penalty
	}
	if maxPenalty == 0 {
		maxPenalty = 1
	}

	const breakBarMaxW = 120.0
	for _, re := range ruleEntries {
		f.SetFont("Arial", "", 8)
		f.SetTextColor(40, 40, 40)
		f.CellFormat(38, 5.5, tr(re.id), "", 0, "L", false, 0, "")

		bw := breakBarMaxW * re.penalty / maxPenalty
		f.SetFillColor(sr1, sg1, sb1)
		f.Rect(48+10, f.GetY()+1, bw, 3.5, "F")
		f.SetFillColor(220, 220, 220)
		f.Rect(48+10+bw, f.GetY()+1, breakBarMaxW-bw, 3.5, "F")
		f.SetX(48 + 10)
		f.CellFormat(breakBarMaxW, 5.5, "", "", 0, "L", false, 0, "")

		f.SetFont("Arial", "", 8)
		f.SetTextColor(80, 80, 80)
		label := "violation"
		if re.count != 1 {
			label = "violations"
		}
		f.CellFormat(pW-38-10-breakBarMaxW, 5.5, fmt.Sprintf("%d %s", re.count, label), "", 1, "R", false, 0, "")
	}

	if len(ruleEntries) == 0 {
		f.SetFont("Arial", "I", 8)
		f.SetTextColor(100, 100, 100)
		f.CellFormat(pW, 6, "No violations found.", "", 1, "C", false, 0, "")
	}

	f.Ln(5)

	// ── Verdict text ─────────────────────────────────────────────────────────
	f.SetFont("Arial", "B", 9)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(pW, 6, "Verdict", "", 1, "L", false, 0, "")
	f.Ln(1)

	gradeLabels := map[string]string{
		"A": "no significant",
		"B": "minor",
		"C": "moderate",
		"D": "significant",
		"F": "critical",
	}
	gradeLabel := gradeLabels[sr.grade]
	if gradeLabel == "" {
		gradeLabel = "some"
	}

	var verdictText string
	if sr.score >= 90 {
		verdictText = "This design meets all manufacturability requirements and is ready for fabrication submission."
	} else if errCount > 0 {
		topRule := ""
		if len(ruleEntries) > 0 {
			topRule = ruleEntries[0].id
		}
		verdictText = fmt.Sprintf(
			"This design has %s manufacturability issues. %d error(s) must be resolved before fabrication submission.",
			gradeLabel,
			errCount,
		)
		if topRule != "" {
			verdictText += " Focus on " + topRule + " violations."
		}
	} else {
		verdictText = fmt.Sprintf(
			"%d warning(s) were found. Review is recommended before fabrication submission.",
			warnCount,
		)
	}

	f.SetFont("Arial", "", 8.5)
	f.SetTextColor(50, 50, 50)
	f.MultiCell(pW, 5.5, tr(verdictText), "1", "L", false)

	// ════════════════════════════════════════════════════════════════════════
	// PAGE 2: Design Rules + Violations (existing layout, unchanged)
	// ════════════════════════════════════════════════════════════════════════
	f.AddPage()

	// ── Header bar (repeated) ────────────────────────────────────────────────
	f.SetFillColor(22, 101, 52)
	f.SetTextColor(255, 255, 255)
	f.SetFont("Arial", "B", 15)
	f.CellFormat(pW, 12, "  DFM Analysis Report", "", 0, "L", true, 0, "")
	f.SetXY(10, f.GetY())
	f.SetFont("Arial", "", 9)
	f.CellFormat(pW, 12, dateStr, "", 1, "R", false, 0, "")

	f.SetFillColor(240, 240, 240)
	f.SetTextColor(60, 60, 60)
	f.SetFont("Arial", "", 8)
	f.CellFormat(pW/2, 6, tr("  File: "+trunc(submission.Filename, 48)), "", 0, "L", true, 0, "")
	f.CellFormat(pW/4, 6, "Type: "+submission.FileType, "", 0, "L", true, 0, "")
	f.CellFormat(pW/4, 6, tr("Profile: "+trunc(profile.Name, 22)), "", 1, "L", true, 0, "")

	f.Ln(5)

	// ── Violation summary + Board image ───────────────────────────────────────
	summaryY := f.GetY()

	imgBytes := renderBoardImage(board, violations)
	f.SetXY(10, summaryY)
	f.SetFont("Arial", "B", 10)
	f.SetTextColor(30, 30, 30)
	f.CellFormat(90, 7, "Violation Summary", "", 1, "L", false, 0, "")
	f.Ln(2)

	type sevRow struct {
		label, sev string
		count      int
	}
	for _, row := range []sevRow{
		{"Errors", "ERROR", errCount},
		{"Warnings", "WARNING", warnCount},
		{"Info", "INFO", infoCount},
	} {
		ri, gi, bi := severityRGBi(row.sev)
		f.SetX(10)
		f.SetFillColor(ri, gi, bi)
		f.SetTextColor(255, 255, 255)
		f.SetFont("Arial", "B", 9)
		f.CellFormat(28, 6, "  "+row.label, "0", 0, "L", true, 0, "")
		f.SetFillColor(240, 240, 240)
		f.SetTextColor(30, 30, 30)
		f.SetFont("Arial", "", 9)
		f.CellFormat(18, 6, fmt.Sprintf("%d", row.count), "0", 1, "R", true, 0, "")
	}

	imgOpts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
	f.RegisterImageOptionsReader("board_img", imgOpts, bytes.NewReader(imgBytes))
	f.ImageOptions("board_img", 105, summaryY, 95, 0, false, imgOpts, 0, "")
	f.SetY(summaryY + 74)
	f.Ln(4)

	// ── Design Rules table ────────────────────────────────────────────────────
	f.SetFont("Arial", "B", 10)
	f.SetTextColor(30, 30, 30)
	f.CellFormat(pW, 7, "Design Rules Applied", "", 1, "L", false, 0, "")
	f.Ln(1)

	f.SetFillColor(22, 101, 52)
	f.SetTextColor(255, 255, 255)
	f.SetFont("Arial", "B", 8)
	f.CellFormat(pW/2, 6, "Rule", "1", 0, "C", true, 0, "")
	f.CellFormat(pW/2, 6, "Value", "1", 1, "C", true, 0, "")

	type ruleRow struct{ name, val string }
	ruleRows := []ruleRow{
		{"Min Trace Width", fmt.Sprintf("%.3f mm", rules.MinTraceWidthMM)},
		{"Min Clearance", fmt.Sprintf("%.3f mm", rules.MinClearanceMM)},
		{"Min Drill Diameter", fmt.Sprintf("%.3f mm", rules.MinDrillDiamMM)},
		{"Max Drill Diameter", fmt.Sprintf("%.3f mm", rules.MaxDrillDiamMM)},
		{"Min Annular Ring", fmt.Sprintf("%.3f mm", rules.MinAnnularRingMM)},
		{"Max Aspect Ratio", fmt.Sprintf("%.1f : 1", rules.MaxAspectRatio)},
		{"Min Solder Mask Dam", fmt.Sprintf("%.3f mm", rules.MinSolderMaskDamMM)},
		{"Min Edge Clearance", fmt.Sprintf("%.3f mm", rules.MinEdgeClearanceMM)},
	}
	for i, row := range ruleRows {
		if i%2 == 0 {
			f.SetFillColor(248, 248, 248)
		} else {
			f.SetFillColor(255, 255, 255)
		}
		f.SetTextColor(40, 40, 40)
		f.SetFont("Arial", "", 8)
		f.CellFormat(pW/2, 5.5, "  "+row.name, "1", 0, "L", true, 0, "")
		f.CellFormat(pW/2, 5.5, row.val, "1", 1, "C", true, 0, "")
	}

	f.Ln(6)

	// ── Violations table ──────────────────────────────────────────────────────
	const maxViolRows = 200
	shownViolations := violations
	if len(shownViolations) > maxViolRows {
		shownViolations = shownViolations[:maxViolRows]
	}

	violTitle := fmt.Sprintf("Violations  (%d total)", len(violations))
	if len(violations) > maxViolRows {
		violTitle = fmt.Sprintf("Violations  (showing first %d of %d — see full export for complete list)", maxViolRows, len(violations))
	}
	f.SetFont("Arial", "B", 10)
	f.SetTextColor(30, 30, 30)
	f.CellFormat(pW, 7, violTitle, "", 1, "L", false, 0, "")
	f.Ln(1)

	const (
		colSev  = 22.0
		colRule = 33.0
		colLyr  = 28.0
		colX    = 12.0
		colY    = 12.0
		colMsg  = 83.0
	)

	f.SetFillColor(22, 101, 52)
	f.SetTextColor(255, 255, 255)
	f.SetFont("Arial", "B", 8)
	f.CellFormat(colSev, 6, "Severity", "1", 0, "C", true, 0, "")
	f.CellFormat(colRule, 6, "Rule", "1", 0, "C", true, 0, "")
	f.CellFormat(colLyr, 6, "Layer", "1", 0, "C", true, 0, "")
	f.CellFormat(colX, 6, "X (mm)", "1", 0, "C", true, 0, "")
	f.CellFormat(colY, 6, "Y (mm)", "1", 0, "C", true, 0, "")
	f.CellFormat(colMsg, 6, "Description", "1", 1, "C", true, 0, "")

	if len(violations) == 0 {
		f.SetFillColor(255, 255, 255)
		f.SetTextColor(100, 100, 100)
		f.SetFont("Arial", "I", 9)
		f.CellFormat(pW, 7, "No violations found - board passes all checks.", "1", 1, "C", false, 0, "")
	}

	const lineH = 5.0
	f.SetFont("Arial", "", 7.5)
	for i, v := range shownViolations {
		fill := i%2 == 0
		if fill {
			f.SetFillColor(250, 250, 250)
		} else {
			f.SetFillColor(255, 255, 255)
		}

		msgText := tr(v.Message)
		nLines := len(f.SplitLines([]byte(msgText), colMsg-1))
		if nLines < 1 {
			nLines = 1
		}
		rowH := float64(nLines) * lineH

		ri, gi, bi := severityRGBi(v.Severity)
		f.SetTextColor(ri, gi, bi)
		f.CellFormat(colSev, rowH, v.Severity, "1", 0, "C", true, 0, "")

		f.SetTextColor(40, 40, 40)
		f.CellFormat(colRule, rowH, tr(trunc(v.RuleID, 22)), "1", 0, "L", true, 0, "")
		f.CellFormat(colLyr, rowH, tr(trunc(v.Layer, 18)), "1", 0, "L", true, 0, "")
		f.CellFormat(colX, rowH, fmt.Sprintf("%.2f", v.X), "1", 0, "R", true, 0, "")
		f.CellFormat(colY, rowH, fmt.Sprintf("%.2f", v.Y), "1", 0, "R", true, 0, "")
		f.MultiCell(colMsg, lineH, msgText, "1", "L", fill)
	}

	// ── Acknowledged / Waived section ────────────────────────────────────────
	if len(ignoredViolations) > 0 {
		f.Ln(6)
		f.SetFont("Arial", "B", 10)
		f.SetTextColor(30, 30, 30)
		f.CellFormat(pW, 7, fmt.Sprintf("Acknowledged / Waived  (%d)", len(ignoredViolations)), "", 1, "L", false, 0, "")
		f.Ln(1)

		f.SetFont("Arial", "I", 8)
		f.SetTextColor(100, 100, 100)
		f.MultiCell(pW, 5, "The following violations were reviewed and acknowledged by the design team. They are excluded from the manufacturability score.", "0", "L", false)
		f.Ln(2)

		f.SetFillColor(136, 136, 136)
		f.SetTextColor(255, 255, 255)
		f.SetFont("Arial", "B", 8)
		f.CellFormat(colSev, 6, "Severity", "1", 0, "C", true, 0, "")
		f.CellFormat(colRule, 6, "Rule", "1", 0, "C", true, 0, "")
		f.CellFormat(colLyr, 6, "Layer", "1", 0, "C", true, 0, "")
		f.CellFormat(colX, 6, "X (mm)", "1", 0, "C", true, 0, "")
		f.CellFormat(colY, 6, "Y (mm)", "1", 0, "C", true, 0, "")
		f.CellFormat(colMsg, 6, "Description", "1", 1, "C", true, 0, "")

		f.SetFont("Arial", "", 7.5)
		for i, v := range ignoredViolations {
			if i%2 == 0 {
				f.SetFillColor(248, 248, 248)
			} else {
				f.SetFillColor(255, 255, 255)
			}
			msgText := tr(v.Message)
			nLines := len(f.SplitLines([]byte(msgText), colMsg-1))
			if nLines < 1 {
				nLines = 1
			}
			rowH := float64(nLines) * lineH

			ri, gi, bi := severityRGBi(v.Severity)
			f.SetTextColor(ri, gi, bi)
			f.CellFormat(colSev, rowH, v.Severity, "1", 0, "C", true, 0, "")

			f.SetTextColor(100, 100, 100) // dimmed for waived rows
			f.CellFormat(colRule, rowH, tr(trunc(v.RuleID, 22)), "1", 0, "L", true, 0, "")
			f.CellFormat(colLyr, rowH, tr(trunc(v.Layer, 18)), "1", 0, "L", true, 0, "")
			f.CellFormat(colX, rowH, fmt.Sprintf("%.2f", v.X), "1", 0, "R", true, 0, "")
			f.CellFormat(colY, rowH, fmt.Sprintf("%.2f", v.Y), "1", 0, "R", true, 0, "")
			f.MultiCell(colMsg, lineH, msgText, "1", "L", i%2 == 0)
		}
	}

	// ── Stream response ───────────────────────────────────────────────────────
	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="dfm-report-`+jobID+`.pdf"`)
	return f.Output(c.Response().Writer)
}
