package dfmengine

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Golden-board regression harness.
//
// Each directory under testdata/golden/<board>/ holds:
//   - board.json.gz            — BoardData parsed from a real ODB++ board by the
//     production sidecar parser (regenerate with
//     sidecar/gerbonara/scripts/gen_golden.py)
//   - expected_violations.json — per-rule violation counts, score, and a set of
//     anchor violations that must match exactly
//
// The committed JSON is the contract: any engine change that shifts violation
// semantics on real boards shows up here as a per-rule diff. To accept a new
// baseline after an intentional change:
//
//	go test -run TestGoldenBoards -update ./...

var updateGolden = flag.Bool("update", false, "rewrite golden expected_violations.json files")

// goldenProfileJSON is the canonical default capability profile. Keep in sync
// with defaultProfileRulesJSON in apps/api/src/routes/defaults.go.
const goldenProfileJSON = `{"minTraceWidthMM":0.15,"minClearanceMM":0.15,"minDrillDiamMM":0.3,"maxDrillDiamMM":6.3,"minAnnularRingMM":0.15,"maxAspectRatio":10,"minSolderMaskDamMM":0.1,"minEdgeClearanceMM":0.3,"minDrillToDrillMM":0.25,"minDrillToCopperMM":0.25,"minCopperSliverMM":0.1,"smallestPackageClass":"","maxTraceImbalanceRatio":2.0,"enableSilkscreenOnPadCheck":true,"maxComponentHeightTopMM":10,"maxComponentHeightBottomMM":5,"minComponentSpacingMM":0.5,"componentSpacing":{"discreteMM":0.254,"leadedMM":1.27,"bgaMM":3.175,"throughHoleMM":3.175},"flagThroughHoleOnBottom":true,"minMountingHoleKeepoutMM":0.5,"enableFiducialPlacementCheck":true,"enableFiducialCountCheck":true,"enablePadSizeForPackageCheck":true,"enableTombstoningRiskCheck":true,"enableViaInPadCheck":true}`

// Anchor match tolerances. X/Y within 0.01 mm and MeasuredMM within 0.001 mm
// count as "the same violation" — loose enough to survive float-level noise,
// tight enough to catch a feature moving or a measurement changing.
const (
	anchorXYTolMM       = 0.01
	anchorMeasuredTolMM = 0.001
)

type goldenAnchor struct {
	RuleID     string  `json:"ruleId"`
	Severity   string  `json:"severity"`
	Layer      string  `json:"layer"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	MeasuredMM float64 `json:"measuredMM"`
	LimitMM    float64 `json:"limitMM"`
}

type goldenExpectation struct {
	RuleCounts map[string]int `json:"ruleCounts"`
	Total      int            `json:"total"`
	Score      int            `json:"score"`
	Grade      string         `json:"grade"`
	Anchors    []goldenAnchor `json:"anchors"`
}

func goldenBoardDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("testdata", "golden"))
	if err != nil {
		t.Fatalf("reading testdata/golden: %v", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		t.Fatal("no golden boards committed under testdata/golden")
	}
	return dirs
}

func loadGoldenBoard(t *testing.T, board string) BoardData {
	t.Helper()
	dir := filepath.Join("testdata", "golden", board)
	var raw []byte
	if f, err := os.Open(filepath.Join(dir, "board.json.gz")); err == nil {
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("%s: opening board.json.gz: %v", board, err)
		}
		defer gz.Close()
		raw, err = io.ReadAll(gz)
		if err != nil {
			t.Fatalf("%s: reading board.json.gz: %v", board, err)
		}
	} else {
		raw, err = os.ReadFile(filepath.Join(dir, "board.json"))
		if err != nil {
			t.Fatalf("%s: no board.json(.gz) fixture: %v", board, err)
		}
	}
	var bd BoardData
	if err := json.Unmarshal(raw, &bd); err != nil {
		t.Fatalf("%s: unmarshaling board fixture: %v", board, err)
	}
	return bd
}

func goldenProfile(t *testing.T) ProfileRules {
	t.Helper()
	var p ProfileRules
	if err := json.Unmarshal([]byte(goldenProfileJSON), &p); err != nil {
		t.Fatalf("unmarshaling golden profile: %v", err)
	}
	return p
}

// buildExpectation derives the golden expectation from an actual violation set.
func buildExpectation(violations []Violation, outline []Point) goldenExpectation {
	exp := goldenExpectation{RuleCounts: map[string]int{}, Total: len(violations)}
	byRule := map[string][]Violation{}
	for _, v := range violations {
		exp.RuleCounts[v.RuleID]++
		byRule[v.RuleID] = append(byRule[v.RuleID], v)
	}
	score := ComputeScore(violations, outline)
	exp.Score = score.Score
	exp.Grade = score.Grade

	// Anchors: up to 3 violations per rule (first, middle, last after a
	// deterministic sort). These must match exactly on rerun — they pin
	// concrete findings, not just counts.
	ruleIDs := make([]string, 0, len(byRule))
	for id := range byRule {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		vs := byRule[id]
		sort.Slice(vs, func(i, j int) bool {
			if vs[i].Layer != vs[j].Layer {
				return vs[i].Layer < vs[j].Layer
			}
			if vs[i].X != vs[j].X {
				return vs[i].X < vs[j].X
			}
			if vs[i].Y != vs[j].Y {
				return vs[i].Y < vs[j].Y
			}
			return vs[i].MeasuredMM < vs[j].MeasuredMM
		})
		picks := []int{0}
		if len(vs) > 2 {
			picks = append(picks, len(vs)/2)
		}
		if len(vs) > 1 {
			picks = append(picks, len(vs)-1)
		}
		for _, k := range picks {
			v := vs[k]
			exp.Anchors = append(exp.Anchors, goldenAnchor{
				RuleID:     v.RuleID,
				Severity:   v.Severity,
				Layer:      v.Layer,
				X:          round3(v.X),
				Y:          round3(v.Y),
				MeasuredMM: round4(v.MeasuredMM),
				LimitMM:    round4(v.LimitMM),
			})
		}
	}
	return exp
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func anchorMatches(a goldenAnchor, v Violation) bool {
	return a.RuleID == v.RuleID &&
		a.Severity == v.Severity &&
		a.Layer == v.Layer &&
		math.Abs(a.X-v.X) <= anchorXYTolMM &&
		math.Abs(a.Y-v.Y) <= anchorXYTolMM &&
		math.Abs(a.MeasuredMM-v.MeasuredMM) <= anchorMeasuredTolMM
}

// diffTable renders a per-rule expected-vs-actual count comparison.
func diffTable(expected, actual map[string]int) string {
	ids := map[string]bool{}
	for id := range expected {
		ids[id] = true
	}
	for id := range actual {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	var b strings.Builder
	fmt.Fprintf(&b, "%-26s %9s %9s\n", "rule", "expected", "actual")
	for _, id := range sorted {
		marker := ""
		if expected[id] != actual[id] {
			marker = "  <-- MISMATCH"
		}
		fmt.Fprintf(&b, "%-26s %9d %9d%s\n", id, expected[id], actual[id], marker)
	}
	return b.String()
}

func TestGoldenBoards(t *testing.T) {
	if testing.Short() {
		t.Skip("golden boards are slow; skipped with -short")
	}
	profile := goldenProfile(t)
	for _, board := range goldenBoardDirs(t) {
		board := board
		t.Run(board, func(t *testing.T) {
			t.Parallel()
			bd := loadGoldenBoard(t, board)
			violations := NewRunner().Run(bd, profile)
			actual := buildExpectation(violations, bd.Outline)

			expPath := filepath.Join("testdata", "golden", board, "expected_violations.json")
			if *updateGolden {
				data, err := json.MarshalIndent(actual, "", "  ")
				if err != nil {
					t.Fatalf("marshaling expectation: %v", err)
				}
				if err := os.WriteFile(expPath, append(data, '\n'), 0o644); err != nil {
					t.Fatalf("writing %s: %v", expPath, err)
				}
				t.Logf("updated %s (%d violations, score %d %s)",
					expPath, actual.Total, actual.Score, actual.Grade)
				return
			}

			raw, err := os.ReadFile(expPath)
			if err != nil {
				t.Fatalf("missing %s — run `go test -run TestGoldenBoards -update ./...` to create it", expPath)
			}
			var expected goldenExpectation
			if err := json.Unmarshal(raw, &expected); err != nil {
				t.Fatalf("unmarshaling %s: %v", expPath, err)
			}

			countsMatch := len(expected.RuleCounts) == len(actual.RuleCounts)
			if countsMatch {
				for id, n := range expected.RuleCounts {
					if actual.RuleCounts[id] != n {
						countsMatch = false
						break
					}
				}
			}
			if !countsMatch {
				t.Errorf("per-rule violation counts drifted:\n%s",
					diffTable(expected.RuleCounts, actual.RuleCounts))
			}
			if expected.Total != actual.Total {
				t.Errorf("total violations: expected %d, got %d", expected.Total, actual.Total)
			}
			if expected.Score != actual.Score || expected.Grade != actual.Grade {
				t.Errorf("score drifted: expected %d (%s), got %d (%s)",
					expected.Score, expected.Grade, actual.Score, actual.Grade)
			}
			for _, a := range expected.Anchors {
				found := false
				for _, v := range violations {
					if anchorMatches(a, v) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("anchor violation not found: %s %s on %s at (%.3f, %.3f) measured %.4f",
						a.RuleID, a.Severity, a.Layer, a.X, a.Y, a.MeasuredMM)
				}
			}
			if t.Failed() {
				t.Logf("if this change is intentional, rebaseline with: go test -run TestGoldenBoards -update ./...")
			}
		})
	}
}

// TestGoldenDeterminism runs the full rule set twice on each golden board and
// requires byte-identical violation output. The v1↔v2 compare feature and the
// golden expectations above both rely on this property.
func TestGoldenDeterminism(t *testing.T) {
	if testing.Short() {
		t.Skip("golden boards are slow; skipped with -short")
	}
	profile := goldenProfile(t)
	for _, board := range goldenBoardDirs(t) {
		board := board
		t.Run(board, func(t *testing.T) {
			t.Parallel()
			bd := loadGoldenBoard(t, board)
			v1 := NewRunner().Run(bd, profile)
			v2 := NewRunner().Run(bd, profile)
			j1, err := json.Marshal(v1)
			if err != nil {
				t.Fatal(err)
			}
			j2, err := json.Marshal(v2)
			if err != nil {
				t.Fatal(err)
			}
			if string(j1) != string(j2) {
				t.Errorf("violation output is not deterministic across runs (%d vs %d violations)",
					len(v1), len(v2))
			}
		})
	}
}
