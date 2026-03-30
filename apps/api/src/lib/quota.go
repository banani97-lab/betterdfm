package lib

import (
	"fmt"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// QuotaCheckResult is returned by quota check methods.
type QuotaCheckResult struct {
	Allowed   bool   `json:"allowed"`
	IsOverage bool   `json:"isOverage"`
	Used      int    `json:"used"`
	Limit     int    `json:"limit"` // -1 = unlimited
	Message   string `json:"message"`
}

// UsageSummary is returned by GetUsageSummary.
type UsageSummary struct {
	Tier        string        `json:"tier"`
	PeriodStart time.Time     `json:"billingPeriodStart"`
	PeriodEnd   time.Time     `json:"billingPeriodEnd"`
	Analyses    ResourceUsage `json:"analyses"`
	Users       ResourceUsage `json:"users"`
	Profiles    ResourceUsage `json:"profiles"`
	Projects    ResourceUsage `json:"projects"`
	ShareLinks  ResourceUsage `json:"shareLinks"`
	Features    FeatureFlags  `json:"features"`
}

// ResourceUsage tracks usage against limits.
type ResourceUsage struct {
	Used    int `json:"used"`
	Limit   int `json:"limit"`
	Overage int `json:"overage,omitempty"`
}

// FeatureFlags indicates which features are enabled for the tier.
type FeatureFlags struct {
	BatchUpload    bool `json:"batchUpload"`
	MaxBatchFiles  int  `json:"maxBatchFiles"`
	Compare        bool `json:"compare"`
	AdminDashboard bool `json:"adminDashboard"`
	CustomerPortal bool `json:"customerPortal"`
}

// QuotaService provides subscription tier enforcement.
type QuotaService struct {
	db *gorm.DB
}

// NewQuotaService creates a new QuotaService.
func NewQuotaService(database *gorm.DB) *QuotaService {
	return &QuotaService{db: database}
}

// loadOrg fetches the organization by primary key.
func (q *QuotaService) loadOrg(orgID string) (db.Organization, TierLimits, error) {
	var org db.Organization
	if err := q.db.First(&org, "id = ?", orgID).Error; err != nil {
		return org, TierLimits{}, err
	}
	return org, GetTierLimits(org.SubscriptionTier), nil
}

// billingPeriod computes the current billing period start and end from the anchor.
func billingPeriod(anchor time.Time, now time.Time) (start time.Time, end time.Time) {
	if anchor.IsZero() {
		// Default to first of current month
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
		return
	}

	day := anchor.Day()

	// Start from anchor's year/month, advance until we pass now
	candidate := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, time.UTC)

	// Handle months where anchor day doesn't exist (e.g., day 31 in a 30-day month)
	// Go normalizes this automatically (e.g., Feb 31 -> Mar 3), so clamp to last day
	for candidate.Month() != now.Month() && day > 28 {
		day--
		candidate = time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, time.UTC)
	}

	if candidate.After(now) {
		// Current period started last month
		start = candidate.AddDate(0, -1, 0)
		end = candidate
	} else {
		start = candidate
		end = candidate.AddDate(0, 1, 0)
	}
	return
}

// CheckAnalysisQuota checks if the org can run another analysis.
// Always allowed (overage model), but sets IsOverage when over limit.
func (q *QuotaService) CheckAnalysisQuota(orgID string) QuotaCheckResult {
	org, limits, err := q.loadOrg(orgID)
	if err != nil {
		return QuotaCheckResult{Allowed: true, Limit: -1}
	}

	if limits.AnalysesPerMonth == -1 {
		return QuotaCheckResult{Allowed: true, Limit: -1}
	}

	now := time.Now()
	periodStart, _ := billingPeriod(org.BillingCycleAnchor, now)

	var count int64
	q.db.Model(&db.UsageEvent{}).
		Where("org_id = ? AND event_type = ? AND created_at >= ?", orgID, "ANALYSIS", periodStart).
		Count(&count)

	used := int(count)
	isOverage := used >= limits.AnalysesPerMonth

	result := QuotaCheckResult{
		Allowed: true, // Always allowed (overage model)
		Used:    used,
		Limit:   limits.AnalysesPerMonth,
	}

	if isOverage {
		result.IsOverage = true
		overageCount := used - limits.AnalysesPerMonth + 1
		rateDollars := float64(limits.OverageRateCents) / 100.0
		result.Message = fmt.Sprintf("Additional analyses are $%.0f each. This is overage analysis #%d.", rateDollars, overageCount)
	}

	return result
}

// CheckResourceLimit checks if the org has room for another resource of the given type.
// Returns Allowed=false when at the limit.
func (q *QuotaService) CheckResourceLimit(orgID string, resource string) QuotaCheckResult {
	_, limits, err := q.loadOrg(orgID)
	if err != nil {
		return QuotaCheckResult{Allowed: true, Limit: -1}
	}

	var limit int
	var count int64

	switch resource {
	case "users":
		limit = limits.MaxUsers
		q.db.Model(&db.User{}).Where("org_id = ?", orgID).Count(&count)
	case "profiles":
		limit = limits.MaxProfiles
		q.db.Model(&db.CapabilityProfile{}).Where("org_id = ?", orgID).Count(&count)
	case "projects":
		limit = limits.MaxProjects
		q.db.Model(&db.Project{}).Where("org_id = ? AND archived = ?", orgID, false).Count(&count)
	case "share_links":
		limit = limits.MaxActiveShareLinks
		q.db.Model(&db.ShareLink{}).Where("org_id = ? AND active = ?", orgID, true).Count(&count)
	default:
		return QuotaCheckResult{Allowed: true, Limit: -1}
	}

	if limit == -1 {
		return QuotaCheckResult{Allowed: true, Used: int(count), Limit: -1}
	}

	used := int(count)
	if used >= limit {
		return QuotaCheckResult{
			Allowed: false,
			Used:    used,
			Limit:   limit,
			Message: fmt.Sprintf("Upgrade required: your %s plan allows up to %d %s.", "", limit, resource),
		}
	}

	return QuotaCheckResult{Allowed: true, Used: used, Limit: limit}
}

// CheckResourceLimitWithTier is like CheckResourceLimit but includes the tier name in the message.
func (q *QuotaService) CheckResourceLimitWithTier(orgID string, resource string) QuotaCheckResult {
	org, limits, err := q.loadOrg(orgID)
	if err != nil {
		return QuotaCheckResult{Allowed: true, Limit: -1}
	}

	var limit int
	var count int64
	var resourceLabel string

	switch resource {
	case "users":
		limit = limits.MaxUsers
		resourceLabel = "user"
		q.db.Model(&db.User{}).Where("org_id = ?", orgID).Count(&count)
	case "profiles":
		limit = limits.MaxProfiles
		resourceLabel = "capability profile"
		q.db.Model(&db.CapabilityProfile{}).Where("org_id = ?", orgID).Count(&count)
	case "projects":
		limit = limits.MaxProjects
		resourceLabel = "project"
		q.db.Model(&db.Project{}).Where("org_id = ? AND archived = ?", orgID, false).Count(&count)
	case "share_links":
		limit = limits.MaxActiveShareLinks
		resourceLabel = "active share link"
		q.db.Model(&db.ShareLink{}).Where("org_id = ? AND active = ?", orgID, true).Count(&count)
	default:
		return QuotaCheckResult{Allowed: true, Limit: -1}
	}

	if limit == -1 {
		return QuotaCheckResult{Allowed: true, Used: int(count), Limit: -1}
	}

	tier := org.SubscriptionTier
	if tier == "" {
		tier = "STARTER"
	}

	used := int(count)
	if used >= limit {
		plural := resourceLabel + "s"
		if limit == 1 {
			plural = resourceLabel
		}
		return QuotaCheckResult{
			Allowed: false,
			Used:    used,
			Limit:   limit,
			Message: fmt.Sprintf("Upgrade required: your %s plan allows up to %d %s.", tier, limit, plural),
		}
	}

	return QuotaCheckResult{Allowed: true, Used: used, Limit: limit}
}

// CheckFeatureEnabled checks if a feature is enabled for the org's tier.
func (q *QuotaService) CheckFeatureEnabled(orgID string, feature string) bool {
	_, limits, err := q.loadOrg(orgID)
	if err != nil {
		return false
	}

	switch feature {
	case "batch":
		return limits.MaxBatchFiles > 0
	case "compare":
		return limits.CompareEnabled
	case "share_links":
		return limits.MaxActiveShareLinks != 0
	case "admin_dashboard":
		return limits.AdminDashboard
	default:
		return false
	}
}

// RecordAnalysis inserts a UsageEvent for an analysis.
func (q *QuotaService) RecordAnalysis(orgID, jobID string, isOverage bool) {
	event := db.UsageEvent{
		ID:        uuid.New().String(),
		OrgID:     orgID,
		EventType: "ANALYSIS",
		RefID:     jobID,
		Overage:   isOverage,
		CreatedAt: time.Now(),
	}
	q.db.Create(&event)
}

// GetUsageSummary returns a complete usage summary for the org.
func (q *QuotaService) GetUsageSummary(orgID string) UsageSummary {
	org, limits, err := q.loadOrg(orgID)
	if err != nil {
		return UsageSummary{Tier: "STARTER"}
	}

	tier := org.SubscriptionTier
	if tier == "" {
		tier = "STARTER"
	}

	now := time.Now()
	periodStart, periodEnd := billingPeriod(org.BillingCycleAnchor, now)

	// Count analyses this period
	var analysisCount int64
	q.db.Model(&db.UsageEvent{}).
		Where("org_id = ? AND event_type = ? AND created_at >= ?", orgID, "ANALYSIS", periodStart).
		Count(&analysisCount)

	// Count overage analyses this period
	var overageCount int64
	q.db.Model(&db.UsageEvent{}).
		Where("org_id = ? AND event_type = ? AND overage = ? AND created_at >= ?", orgID, "ANALYSIS", true, periodStart).
		Count(&overageCount)

	// Count resources
	var userCount, profileCount, projectCount, shareLinkCount int64
	q.db.Model(&db.User{}).Where("org_id = ?", orgID).Count(&userCount)
	q.db.Model(&db.CapabilityProfile{}).Where("org_id = ?", orgID).Count(&profileCount)
	q.db.Model(&db.Project{}).Where("org_id = ? AND archived = ?", orgID, false).Count(&projectCount)
	q.db.Model(&db.ShareLink{}).Where("org_id = ? AND active = ?", orgID, true).Count(&shareLinkCount)

	return UsageSummary{
		Tier:        tier,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Analyses: ResourceUsage{
			Used:    int(analysisCount),
			Limit:   limits.AnalysesPerMonth,
			Overage: int(overageCount),
		},
		Users: ResourceUsage{
			Used:  int(userCount),
			Limit: limits.MaxUsers,
		},
		Profiles: ResourceUsage{
			Used:  int(profileCount),
			Limit: limits.MaxProfiles,
		},
		Projects: ResourceUsage{
			Used:  int(projectCount),
			Limit: limits.MaxProjects,
		},
		ShareLinks: ResourceUsage{
			Used:  int(shareLinkCount),
			Limit: limits.MaxActiveShareLinks,
		},
		Features: FeatureFlags{
			BatchUpload:    limits.MaxBatchFiles > 0,
			MaxBatchFiles:  limits.MaxBatchFiles,
			Compare:        limits.CompareEnabled,
			AdminDashboard: limits.AdminDashboard,
			CustomerPortal: limits.MaxActiveShareLinks != 0,
		},
	}
}
