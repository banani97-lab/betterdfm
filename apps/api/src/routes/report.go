package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	dfmengine "github.com/betterdfm/dfm-engine"
	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
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

// maxInstancesPerRule is the maximum number of worst-case violation examples
// shown per rule section. All rules are always represented; only the per-rule
// example rows are capped. Users can export the full list via CSV.
const maxInstancesPerRule = 8

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
		return 22, 163, 74
	case score >= 75:
		return 202, 138, 4
	case score >= 60:
		return 234, 88, 12
	default:
		return 220, 38, 38
	}
}

// computeReportScore converts DB violations + board outline to engine types and
// delegates scoring to dfmengine.ComputeScore — the single source of truth.
func computeReportScore(violations []db.Violation, board rptBoardData) dfmengine.ScoreResult {
	evs := make([]dfmengine.Violation, len(violations))
	for i, v := range violations {
		evs[i] = dfmengine.Violation{
			RuleID:     v.RuleID,
			Severity:   v.Severity,
			MeasuredMM: v.MeasuredMM,
			LimitMM:    v.LimitMM,
			Unit:       v.Unit,
		}
	}
	outline := make([]dfmengine.Point, len(board.Outline))
	for i, p := range board.Outline {
		outline[i] = dfmengine.Point{X: p.X, Y: p.Y}
	}
	return dfmengine.ComputeScore(evs, outline)
}

// renderBoardImage draws board outline + violation dots and returns a PNG blob.
func renderBoardImage(board rptBoardData, violations []db.Violation) []byte {
	const (
		W   = 900.0
		H   = 650.0
		pad = 48.0
	)

	dc := gg.NewContext(int(W), int(H))
	dc.SetRGB(0.97, 0.97, 0.97)
	dc.Clear()

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

	scale := math.Min((W-2*pad)/rx, (H-2*pad)/ry)
	offX := (W - rx*scale) / 2
	offY := (H - ry*scale) / 2

	tx := func(x float64) float64 { return (x-minX)*scale + offX }
	ty := func(y float64) float64 { return (y-minY)*scale + offY }

	// Board fill
	if len(board.Outline) >= 2 {
		dc.SetRGB(0.85, 0.93, 0.85)
		dc.MoveTo(tx(board.Outline[0].X), ty(board.Outline[0].Y))
		for _, pt := range board.Outline[1:] {
			dc.LineTo(tx(pt.X), ty(pt.Y))
		}
		dc.ClosePath()
		dc.Fill()

		dc.SetRGB(0.10, 0.35, 0.10)
		dc.SetLineWidth(2.5)
		dc.MoveTo(tx(board.Outline[0].X), ty(board.Outline[0].Y))
		for _, pt := range board.Outline[1:] {
			dc.LineTo(tx(pt.X), ty(pt.Y))
		}
		dc.ClosePath()
		dc.Stroke()
	}

	// Violation dots
	for _, v := range violations {
		r, g, b := severityRGBf(v.Severity)
		dc.SetRGB(1, 1, 1)
		dc.DrawCircle(tx(v.X), ty(v.Y), 9)
		dc.Fill()
		dc.SetRGB(r, g, b)
		dc.DrawCircle(tx(v.X), ty(v.Y), 6)
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

// ruleDisplayName returns a human-readable name for a rule ID.
func ruleDisplayName(id string) string {
	names := map[string]string{
		"trace-width":       "Trace Width",
		"clearance":         "Copper Clearance",
		"drill-size":        "Drill Size",
		"annular-ring":      "Annular Ring",
		"aspect-ratio":      "Drill Aspect Ratio",
		"solder-mask-dam":   "Solder Mask Dam",
		"edge-clearance":    "Edge Clearance",
		"drill-to-drill":    "Drill-to-Drill Spacing",
		"drill-to-copper":   "Drill-to-Copper Spacing",
		"copper-sliver":     "Copper Sliver",
		"silkscreen-on-pad": "Silkscreen on Pad",
		"fiducial-count":    "Fiducial Count",
	}
	if n, ok := names[id]; ok {
		return n
	}
	return strings.Title(strings.ReplaceAll(id, "-", " "))
}

// drawPageHeader draws the green title bar + grey subheader used on every page.
func drawPageHeader(f *fpdf.Fpdf, pW float64, tr func(string) string,
	filename, fileType, profileName, dateStr string) {

	f.SetFillColor(22, 101, 52)
	f.SetTextColor(255, 255, 255)
	f.SetFont("Arial", "B", 14)
	f.CellFormat(pW, 11, "  DFM Analysis Report", "", 0, "L", true, 0, "")
	f.SetXY(15, f.GetY())
	f.SetFont("Arial", "", 8)
	f.CellFormat(pW, 11, dateStr, "", 1, "R", false, 0, "")

	f.SetFillColor(242, 242, 242)
	f.SetTextColor(70, 70, 70)
	f.SetFont("Arial", "", 7.5)
	f.CellFormat(pW*0.5, 5.5, tr("  File: "+trunc(filename, 52)), "", 0, "L", true, 0, "")
	f.CellFormat(pW*0.25, 5.5, "Type: "+fileType, "", 0, "L", true, 0, "")
	f.CellFormat(pW*0.25, 5.5, tr("Profile: "+trunc(profileName, 24)), "", 1, "L", true, 0, "")
}

// violationDeviation returns how far a violation is from its limit, normalised
// to [0, ∞). Used to rank worst instances within a rule group.
func violationDeviation(v db.Violation) float64 {
	if v.LimitMM == 0 {
		return 0
	}
	return math.Abs(v.MeasuredMM-v.LimitMM) / v.LimitMM
}

// GetJobReport handles GET /jobs/:id/report.pdf
func (h *ReportHandler) GetJobReport(c echo.Context) error {
	user := lib.GetUser(c)
	jobID := c.Param("id")

	var job db.AnalysisJob
	if err := h.db.First(&job, "id = ? AND org_id = ?", jobID, user.OrgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	var violations []db.Violation
	h.db.Where("job_id = ? AND ignored = false", jobID).
		Order("severity, rule_id").
		Find(&violations)

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

	sr := computeReportScore(violations, board)

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

	// Group violations by rule, sorted by penalty descending
	type ruleGroup struct {
		id         string
		violations []db.Violation
		penalty    float64
	}
	groupMap := map[string]*ruleGroup{}
	for i := range violations {
		v := &violations[i]
		g, ok := groupMap[v.RuleID]
		if !ok {
			g = &ruleGroup{id: v.RuleID}
			groupMap[v.RuleID] = g
		}
		g.violations = append(g.violations, *v)
		g.penalty += sr.ByRule[v.RuleID]
	}
	var groups []*ruleGroup
	for _, g := range groupMap {
		// Sort instances within each group: worst first
		sort.Slice(g.violations, func(i, j int) bool {
			return violationDeviation(g.violations[i]) > violationDeviation(g.violations[j])
		})
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].penalty > groups[j].penalty
	})

	// ── Build PDF ────────────────────────────────────────────────────────────
	f := fpdf.New("P", "mm", "A4", "")
	f.SetMargins(15, 15, 15)
	f.SetAutoPageBreak(true, 15)
	tr := f.UnicodeTranslatorFromDescriptor("")
	const pW = 180.0 // 210 − 2×15mm margins

	dateStr := ""
	if job.CompletedAt != nil {
		dateStr = job.CompletedAt.UTC().Format("02 Jan 2006  15:04 UTC") + "  "
	}

	// ════════════════════════════════════════════════════════════════════════
	// PAGE 1: Executive Summary
	// ════════════════════════════════════════════════════════════════════════
	f.AddPage()
	drawPageHeader(f, pW, tr, submission.Filename, submission.FileType, profile.Name, dateStr)
	f.Ln(6)

	// ── Score + Design Summary (two columns) ─────────────────────────────────
	const colL, colR = 88.0, 88.0
	sectionY := f.GetY()
	sr1, sg1, sb1 := scoreBarColor(sr.Score)

	// Left: score bar + grade
	f.SetXY(15, sectionY)
	f.SetFont("Arial", "B", 8)
	f.SetTextColor(60, 60, 60)
	f.CellFormat(colL, 5.5, "MANUFACTURABILITY SCORE", "", 1, "L", false, 0, "")
	f.SetX(15)
	f.Ln(2)

	const barW, barH = 84.0, 10.0
	barX, barY := 15.0, f.GetY()
	fillW := barW * float64(sr.Score) / 100.0
	f.SetFillColor(sr1, sg1, sb1)
	f.Rect(barX, barY, fillW, barH, "F")
	f.SetFillColor(225, 225, 225)
	f.Rect(barX+fillW, barY, barW-fillW, barH, "F")

	f.SetXY(15, barY+barH+2)
	f.SetFont("Arial", "B", 24)
	f.SetTextColor(sr1, sg1, sb1)
	f.CellFormat(18, 11, sr.Grade, "", 0, "L", false, 0, "")
	f.SetFont("Arial", "B", 10)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(20, 11, fmt.Sprintf("%d/100", sr.Score), "", 0, "L", false, 0, "")
	f.SetFont("Arial", "", 8)
	f.SetTextColor(90, 90, 90)
	f.CellFormat(colL-38, 11, tr(sr.Verdict), "", 1, "L", false, 0, "")

	// Right: design summary
	f.SetXY(15+colL+4, sectionY)
	f.SetFont("Arial", "B", 8)
	f.SetTextColor(60, 60, 60)
	f.CellFormat(colR, 5.5, "DESIGN SUMMARY", "", 1, "L", false, 0, "")
	f.SetX(15 + colL + 4)
	f.Ln(2)

	type summaryRow struct{ label, val string }
	summaryRows := []summaryRow{
		{"Total violations:", fmt.Sprintf("%d", len(violations))},
		{"Errors:", fmt.Sprintf("%d", errCount)},
		{"Warnings:", fmt.Sprintf("%d", warnCount)},
		{"Info:", fmt.Sprintf("%d", infoCount)},
		{"Waived:", fmt.Sprintf("%d", len(ignoredViolations))},
		{"Rules fired:", fmt.Sprintf("%d", len(groups))},
		{"Board area:", fmt.Sprintf("%.1f cm²", sr.AreaCM2)},
		{"Violation density:", fmt.Sprintf("%.1f /cm²", sr.ViolationDensity)},
	}
	for _, row := range summaryRows {
		f.SetX(15 + colL + 4)
		f.SetFont("Arial", "", 8)
		f.SetTextColor(70, 70, 70)
		f.CellFormat(52, 5, tr(row.label), "", 0, "L", false, 0, "")
		f.SetFont("Arial", "B", 8)
		f.SetTextColor(30, 30, 30)
		f.CellFormat(colR-52, 5, tr(row.val), "", 1, "R", false, 0, "")
	}

	afterColumnsY := f.GetY()
	if barY+barH+13 > afterColumnsY {
		afterColumnsY = barY + barH + 13
	}
	f.SetY(afterColumnsY + 6)

	// ── Score Breakdown by Rule ───────────────────────────────────────────────
	f.SetFont("Arial", "B", 9)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(pW, 6, "Score Impact by Rule", "", 1, "L", false, 0, "")
	f.Ln(1)

	type ruleEntry struct {
		id      string
		penalty float64
		count   int
	}
	var ruleEntries []ruleEntry
	for id, penalty := range sr.ByRule {
		ruleEntries = append(ruleEntries, ruleEntry{id: id, penalty: penalty, count: sr.ByRuleCount[id]})
	}
	sort.Slice(ruleEntries, func(i, j int) bool { return ruleEntries[i].penalty > ruleEntries[j].penalty })

	maxPenalty := 1.0
	if len(ruleEntries) > 0 && ruleEntries[0].penalty > 0 {
		maxPenalty = ruleEntries[0].penalty
	}

	const breakBarMaxW = 98.0
	for _, re := range ruleEntries {
		bw := breakBarMaxW * re.penalty / maxPenalty
		f.SetFont("Arial", "", 7.5)
		f.SetTextColor(40, 40, 40)
		f.CellFormat(46, 5, tr(ruleDisplayName(re.id)), "", 0, "L", false, 0, "")
		f.SetFillColor(sr1, sg1, sb1)
		f.Rect(15+46+2, f.GetY()+0.8, bw, 3.5, "F")
		f.SetFillColor(225, 225, 225)
		f.Rect(15+46+2+bw, f.GetY()+0.8, breakBarMaxW-bw, 3.5, "F")
		f.SetX(15 + 46 + 2)
		f.CellFormat(breakBarMaxW, 5, "", "", 0, "L", false, 0, "")
		f.SetTextColor(100, 100, 100)
		label := "violation"
		if re.count != 1 {
			label = "violations"
		}
		f.CellFormat(pW-46-2-breakBarMaxW, 5, fmt.Sprintf("%d %s", re.count, label), "", 1, "R", false, 0, "")
	}
	if len(ruleEntries) == 0 {
		f.SetFont("Arial", "I", 8)
		f.SetTextColor(100, 100, 100)
		f.CellFormat(pW, 6, "No violations — board passes all checks.", "", 1, "C", false, 0, "")
	}

	f.Ln(5)

	// ── Board overview image (two columns: image left, verdict right) ─────────
	imgBytes := renderBoardImage(board, violations)
	f.RegisterImageOptionsReader("board_img", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(imgBytes))
	imgY := f.GetY()
	const imgW = 88.0
	// Compute height explicitly (rendered canvas is 900×650) so fpdf knows
	// how tall the image is and we can advance Y past it afterward.
	const imgH = imgW * 650.0 / 900.0
	f.ImageOptions("board_img", 15, imgY, imgW, imgH, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Verdict alongside the image
	f.SetXY(15+imgW+4, imgY)
	f.SetFont("Arial", "B", 9)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(pW-imgW-4, 6, "Verdict", "", 1, "L", false, 0, "")
	f.Ln(1)
	f.SetX(15 + imgW + 4)

	gradeLabels := map[string]string{
		"A": "no significant", "B": "minor", "C": "moderate", "D": "significant", "F": "critical",
	}
	gradeLabel := gradeLabels[sr.Grade]
	if gradeLabel == "" {
		gradeLabel = "some"
	}
	var verdictText string
	if sr.Score >= 90 {
		verdictText = "This design meets all manufacturability requirements and is ready for fabrication submission."
	} else if errCount > 0 {
		topRule := ""
		if len(ruleEntries) > 0 {
			topRule = ruleDisplayName(ruleEntries[0].id)
		}
		verdictText = fmt.Sprintf(
			"This design has %s manufacturability issues. %d error(s) must be resolved before fabrication submission.",
			gradeLabel, errCount,
		)
		if topRule != "" {
			verdictText += " The most impactful rule is " + topRule + "."
		}
	} else {
		verdictText = fmt.Sprintf(
			"%d warning(s) were found. Review is recommended before fabrication submission.", warnCount,
		)
	}
	f.SetFont("Arial", "", 8.5)
	f.SetTextColor(50, 50, 50)
	f.SetX(15 + imgW + 4)
	f.MultiCell(pW-imgW-4, 5.5, tr(verdictText), "1", "L", false)

	if len(groups) > 0 {
		f.Ln(4)
		f.SetX(15 + imgW + 4)
		f.SetFont("Arial", "I", 7.5)
		f.SetTextColor(110, 110, 110)
		f.MultiCell(pW-imgW-4, 4.5,
			"See following pages for per-rule breakdown. Export CSV for the complete violation list.",
			"0", "L", false)
	}

	// Advance past whichever column is taller (image or verdict text).
	afterImageY := imgY + imgH + 6
	if f.GetY() > afterImageY {
		afterImageY = f.GetY()
	}
	f.SetY(afterImageY)

	// ════════════════════════════════════════════════════════════════════════
	// PAGES 2+: Issues by Rule
	// ════════════════════════════════════════════════════════════════════════
	if len(groups) > 0 {
		f.AddPage()
		drawPageHeader(f, pW, tr, submission.Filename, submission.FileType, profile.Name, dateStr)
		f.Ln(5)

		// Column widths for the per-rule instance table
		const (
			iColSev  = 18.0
			iColLyr  = 42.0
			iColX    = 18.0
			iColY    = 18.0
			iColMeas = 42.0
			iColLim  = 42.0
		)

		for gi, g := range groups {
			sev := "ERROR"
			if len(g.violations) > 0 {
				sev = g.violations[0].Severity
			}
			ri, gi2, bi := severityRGBi(sev)

			// Keep rule section together: estimate min height needed
			// (header ~10 + suggestion ~6 + table header ~6 + min 1 row ~5 = ~27mm)
			if f.GetY() > 240 {
				f.AddPage()
				drawPageHeader(f, pW, tr, submission.Filename, submission.FileType, profile.Name, dateStr)
				f.Ln(5)
			}

			_ = gi // suppress unused warning

			// Section header bar
			f.SetFillColor(ri, gi2, bi)
			f.SetTextColor(255, 255, 255)
			f.SetFont("Arial", "B", 9)
			displayName := ruleDisplayName(g.id)
			countLabel := "violation"
			if len(g.violations) != 1 {
				countLabel = "violations"
			}
			headerText := fmt.Sprintf("  %s  —  %d %s", displayName, len(g.violations), countLabel)
			f.CellFormat(pW, 8, tr(headerText), "", 0, "L", true, 0, "")
			f.SetFont("Arial", "", 8)
			f.CellFormat(0, 8, sev+"  ", "", 1, "R", false, 0, "")

			// Suggestion line
			suggestion := ""
			if len(g.violations) > 0 && g.violations[0].Suggestion != "" {
				suggestion = g.violations[0].Suggestion
			}
			if suggestion != "" {
				f.SetFillColor(252, 249, 240)
				f.SetTextColor(100, 70, 10)
				f.SetFont("Arial", "I", 7.5)
				f.MultiCell(pW, 4.5, tr("Fix: "+suggestion), "LRB", "L", true)
			}
			f.Ln(1)

			// Table header
			f.SetFillColor(55, 65, 81)
			f.SetTextColor(255, 255, 255)
			f.SetFont("Arial", "B", 7.5)
			f.CellFormat(iColSev, 5.5, "Severity", "1", 0, "C", true, 0, "")
			f.CellFormat(iColLyr, 5.5, "Layer", "1", 0, "C", true, 0, "")
			f.CellFormat(iColX, 5.5, "X (mm)", "1", 0, "C", true, 0, "")
			f.CellFormat(iColY, 5.5, "Y (mm)", "1", 0, "C", true, 0, "")
			f.CellFormat(iColMeas, 5.5, "Measured", "1", 0, "C", true, 0, "")
			f.CellFormat(iColLim, 5.5, "Limit", "1", 1, "C", true, 0, "")

			// Instance rows (worst maxInstancesPerRule)
			shown := g.violations
			if len(shown) > maxInstancesPerRule {
				shown = shown[:maxInstancesPerRule]
			}
			f.SetFont("Arial", "", 7.5)
			for i, v := range shown {
				if i%2 == 0 {
					f.SetFillColor(250, 250, 250)
				} else {
					f.SetFillColor(255, 255, 255)
				}
				vri, vgi, vbi := severityRGBi(v.Severity)
				f.SetTextColor(vri, vgi, vbi)
				f.CellFormat(iColSev, 5, v.Severity, "1", 0, "C", true, 0, "")
				f.SetTextColor(40, 40, 40)
				f.CellFormat(iColLyr, 5, tr(trunc(v.Layer, 24)), "1", 0, "L", true, 0, "")
				f.CellFormat(iColX, 5, fmt.Sprintf("%.2f", v.X), "1", 0, "R", true, 0, "")
				f.CellFormat(iColY, 5, fmt.Sprintf("%.2f", v.Y), "1", 0, "R", true, 0, "")

				measStr := fmt.Sprintf("%.4f %s", v.MeasuredMM, v.Unit)
				limitStr := fmt.Sprintf("%.4f %s", v.LimitMM, v.Unit)
				if v.Unit == "ratio" {
					measStr = fmt.Sprintf("%.1f:1", v.MeasuredMM)
					limitStr = fmt.Sprintf("%.1f:1", v.LimitMM)
				}
				if v.MeasuredMM == 0 && v.LimitMM == 0 {
					measStr = tr("—")
					limitStr = tr("—")
				}
				f.CellFormat(iColMeas, 5, measStr, "1", 0, "C", true, 0, "")
				f.CellFormat(iColLim, 5, limitStr, "1", 1, "C", true, 0, "")
			}

			// Overflow note
			if len(g.violations) > maxInstancesPerRule {
				remaining := len(g.violations) - maxInstancesPerRule
				f.SetFillColor(245, 245, 245)
				f.SetTextColor(110, 110, 110)
				f.SetFont("Arial", "I", 7)
				f.CellFormat(pW, 4.5,
					tr(fmt.Sprintf("  ... and %d more instance(s) - export CSV for the complete list.", remaining)),
					"LRB", 1, "L", true, 0, "")
			}

			f.Ln(5)
		}
	}

	// ════════════════════════════════════════════════════════════════════════
	// FINAL SECTION: Capability Profile + Waived Violations
	// ════════════════════════════════════════════════════════════════════════
	f.AddPage()
	drawPageHeader(f, pW, tr, submission.Filename, submission.FileType, profile.Name, dateStr)
	f.Ln(5)

	// Design rules table
	f.SetFont("Arial", "B", 9)
	f.SetTextColor(40, 40, 40)
	f.CellFormat(pW, 6, tr("Capability Profile - "+profile.Name), "", 1, "L", false, 0, "")
	f.Ln(1)

	f.SetFillColor(22, 101, 52)
	f.SetTextColor(255, 255, 255)
	f.SetFont("Arial", "B", 8)
	f.CellFormat(pW/2, 6, "Rule", "1", 0, "C", true, 0, "")
	f.CellFormat(pW/2, 6, "Value", "1", 1, "C", true, 0, "")

	type ruleRow struct{ name, val string }
	profileRows := []ruleRow{
		{"Min Trace Width", fmt.Sprintf("%.3f mm", rules.MinTraceWidthMM)},
		{"Min Clearance", fmt.Sprintf("%.3f mm", rules.MinClearanceMM)},
		{"Min Drill Diameter", fmt.Sprintf("%.3f mm", rules.MinDrillDiamMM)},
		{"Max Drill Diameter", fmt.Sprintf("%.3f mm", rules.MaxDrillDiamMM)},
		{"Min Annular Ring", fmt.Sprintf("%.3f mm", rules.MinAnnularRingMM)},
		{"Max Aspect Ratio", fmt.Sprintf("%.1f : 1", rules.MaxAspectRatio)},
		{"Min Solder Mask Dam", fmt.Sprintf("%.3f mm", rules.MinSolderMaskDamMM)},
		{"Min Edge Clearance", fmt.Sprintf("%.3f mm", rules.MinEdgeClearanceMM)},
		{"Min Drill-to-Drill", fmt.Sprintf("%.3f mm", rules.MinDrillToDrillMM)},
		{"Min Drill-to-Copper", fmt.Sprintf("%.3f mm", rules.MinDrillToCopperMM)},
		{"Min Copper Sliver", fmt.Sprintf("%.3f mm", rules.MinCopperSliverMM)},
	}
	for i, row := range profileRows {
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

	// Waived violations
	if len(ignoredViolations) > 0 {
		f.Ln(7)
		f.SetFont("Arial", "B", 9)
		f.SetTextColor(40, 40, 40)
		f.CellFormat(pW, 6, fmt.Sprintf("Waived / Acknowledged  (%d)", len(ignoredViolations)), "", 1, "L", false, 0, "")
		f.Ln(1)

		f.SetFont("Arial", "I", 7.5)
		f.SetTextColor(110, 110, 110)
		f.MultiCell(pW, 4.5,
			"The following violations were reviewed and acknowledged. They are excluded from the manufacturability score.",
			"0", "L", false)
		f.Ln(2)

		const (
			wColSev  = 18.0
			wColRule = 36.0
			wColLyr  = 38.0
			wColX    = 18.0
			wColY    = 18.0
			wColMsg  = 52.0
		)

		f.SetFillColor(136, 136, 136)
		f.SetTextColor(255, 255, 255)
		f.SetFont("Arial", "B", 7.5)
		f.CellFormat(wColSev, 5.5, "Sev", "1", 0, "C", true, 0, "")
		f.CellFormat(wColRule, 5.5, "Rule", "1", 0, "C", true, 0, "")
		f.CellFormat(wColLyr, 5.5, "Layer", "1", 0, "C", true, 0, "")
		f.CellFormat(wColX, 5.5, "X (mm)", "1", 0, "C", true, 0, "")
		f.CellFormat(wColY, 5.5, "Y (mm)", "1", 0, "C", true, 0, "")
		f.CellFormat(wColMsg, 5.5, "Description", "1", 1, "C", true, 0, "")

		f.SetFont("Arial", "", 7.5)
		for i, v := range ignoredViolations {
			if i%2 == 0 {
				f.SetFillColor(248, 248, 248)
			} else {
				f.SetFillColor(255, 255, 255)
			}
			msgText := tr(trunc(v.Message, 60))
			ri, gi2, bi := severityRGBi(v.Severity)
			f.SetTextColor(ri, gi2, bi)
			f.CellFormat(wColSev, 5, v.Severity, "1", 0, "C", true, 0, "")
			f.SetTextColor(130, 130, 130)
			f.CellFormat(wColRule, 5, tr(trunc(ruleDisplayName(v.RuleID), 20)), "1", 0, "L", true, 0, "")
			f.CellFormat(wColLyr, 5, tr(trunc(v.Layer, 22)), "1", 0, "L", true, 0, "")
			f.CellFormat(wColX, 5, fmt.Sprintf("%.2f", v.X), "1", 0, "R", true, 0, "")
			f.CellFormat(wColY, 5, fmt.Sprintf("%.2f", v.Y), "1", 0, "R", true, 0, "")
			f.CellFormat(wColMsg, 5, msgText, "1", 1, "L", true, 0, "")
		}
	}

	// ── Footer note ──────────────────────────────────────────────────────────
	f.Ln(8)
	f.SetFont("Arial", "I", 7)
	f.SetTextColor(150, 150, 150)
	f.CellFormat(pW, 5, tr(fmt.Sprintf(
		"Generated by BetterDFM  ·  Job %s  ·  %s",
		jobID, dateStr,
	)), "T", 1, "C", false, 0, "")

	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="dfm-report-`+jobID+`.pdf"`)
	return f.Output(c.Response().Writer)
}
