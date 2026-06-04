package routes

// Tenant-isolation authz tests (WS7). These prove that a user in one org cannot
// read another org's submissions, jobs, violations, or board data via the
// by-ID read endpoints. They guard against the classic multi-tenant regression
// where a handler forgets the `org_id = ?` filter. This is a deliberate,
// compliance-driven exception to the usual "no API-layer tests" convention.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/glebarez/sqlite"
	"github.com/labstack/echo/v4"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	orgA = "org-a"
	orgB = "org-b"
)

func setupIsolationDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gdb.AutoMigrate(&db.Organization{}, &db.Submission{}, &db.AnalysisJob{}, &db.Violation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Org A owns one submission, one job (with inline board data), one violation.
	if err := gdb.Create(&db.Submission{ID: "sub-a", OrgID: orgA, Filename: "a.zip", FileType: "ODB_PLUS_PLUS", Status: "DONE"}).Error; err != nil {
		t.Fatalf("seed submission: %v", err)
	}
	if err := gdb.Create(&db.AnalysisJob{ID: "job-a", OrgID: orgA, SubmissionID: "sub-a", Status: "DONE", BoardData: datatypes.JSON([]byte(`{"ok":true}`))}).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}
	if err := gdb.Create(&db.Violation{ID: "vio-a", OrgID: orgA, JobID: "job-a", RuleID: "trace-width", Severity: "ERROR"}).Error; err != nil {
		t.Fatalf("seed violation: %v", err)
	}
	return gdb
}

// ctxFor builds an echo context authenticated as a user in orgID, with a single
// :id path param.
func ctxFor(orgID, id string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	c.Set("user", &lib.UserClaims{Sub: "u", Email: "u@x", OrgID: orgID, Role: "ADMIN"})
	return c, rec
}

func httpStatus(err error) int {
	if err == nil {
		return 0
	}
	if he, ok := err.(*echo.HTTPError); ok {
		return he.Code
	}
	return -1
}

func TestTenantIsolation_CrossOrgDenied(t *testing.T) {
	gdb := setupIsolationDB(t)
	jobs := NewJobsHandler(gdb, &lib.AWSClients{})
	subs := NewSubmissionsHandler(gdb, &lib.AWSClients{}, lib.NewQuotaService(gdb))

	cases := []struct {
		name    string
		id      string
		handler func(echo.Context) error
	}{
		{"GetJob", "job-a", jobs.GetJob},
		{"GetBoardData", "job-a", jobs.GetBoardData},
		{"GetViolations", "job-a", jobs.GetViolations},
		{"GetSubmission", "sub-a", subs.GetSubmission},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Org B must NOT be able to read org A's resource: expect 404.
			c, _ := ctxFor(orgB, tc.id)
			if got := httpStatus(tc.handler(c)); got != http.StatusNotFound {
				t.Fatalf("%s: org B reading org A resource: expected 404, got %d", tc.name, got)
			}

			// Org A (the owner) must succeed: handler returns nil and writes 200.
			c, rec := ctxFor(orgA, tc.id)
			if err := tc.handler(c); err != nil {
				t.Fatalf("%s: org A reading own resource: unexpected error: %v", tc.name, err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: org A reading own resource: expected 200, got %d", tc.name, rec.Code)
			}
		})
	}
}

func TestTenantIsolation_MissingResource(t *testing.T) {
	gdb := setupIsolationDB(t)
	jobs := NewJobsHandler(gdb, &lib.AWSClients{})

	// A nonexistent id (even within the owner org) must 404, not leak.
	c, _ := ctxFor(orgA, "does-not-exist")
	if got := httpStatus(jobs.GetJob(c)); got != http.StatusNotFound {
		t.Fatalf("expected 404 for missing job, got %d", got)
	}
}
