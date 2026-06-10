package dfmengine

import "testing"

// Benchmarks over the pour-heavy golden board (97K traces, 39K pads, 162
// pours with 7.3K holes) — the stress case for the clearance pour pass and
// the sliver boundary scan. Run before/after geometry changes:
//
//	go test -bench BenchmarkGolden -run '^$' ./...

func benchGoldenBoard(b *testing.B) (BoardData, ProfileRules) {
	b.Helper()
	bd := loadGoldenBoard(b, "pour-heavy")
	profile := goldenProfile(b)
	return bd, profile
}

func BenchmarkGoldenClearance(b *testing.B) {
	bd, profile := benchGoldenBoard(b)
	rule := &ClearanceRule{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rule.Run(bd, profile)
	}
}

func BenchmarkGoldenCopperSliver(b *testing.B) {
	bd, profile := benchGoldenBoard(b)
	rule := &CopperSliverRule{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rule.Run(bd, profile)
	}
}

func BenchmarkGoldenAllRules(b *testing.B) {
	bd, profile := benchGoldenBoard(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewRunner().Run(bd, profile)
	}
}
