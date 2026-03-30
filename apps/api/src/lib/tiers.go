package lib

// TierLimits defines the resource and feature limits for a subscription tier.
type TierLimits struct {
	AnalysesPerMonth    int  // -1 = unlimited
	MaxUsers            int  // -1 = unlimited
	MaxProfiles         int
	MaxProjects         int  // -1 = unlimited
	MaxBatchFiles       int  // 0 = disabled
	CompareEnabled      bool
	MaxActiveShareLinks int  // -1 = unlimited, 0 = disabled
	AdminDashboard      bool
	OverageRateCents    int
}

// Tiers maps subscription tier names to their limits.
var Tiers = map[string]TierLimits{
	"STARTER": {
		AnalysesPerMonth: 50, MaxUsers: -1, MaxProfiles: 1,
		MaxProjects: 10, MaxBatchFiles: 0, CompareEnabled: false,
		MaxActiveShareLinks: 0, AdminDashboard: false, OverageRateCents: 200,
	},
	"PROFESSIONAL": {
		AnalysesPerMonth: 250, MaxUsers: -1, MaxProfiles: 5,
		MaxProjects: -1, MaxBatchFiles: 10, CompareEnabled: true,
		MaxActiveShareLinks: 5, AdminDashboard: false, OverageRateCents: 200,
	},
	"ENTERPRISE": {
		AnalysesPerMonth: -1, MaxUsers: -1, MaxProfiles: -1,
		MaxProjects: -1, MaxBatchFiles: 50, CompareEnabled: true,
		MaxActiveShareLinks: -1, AdminDashboard: true, OverageRateCents: 200,
	},
}

// GetTierLimits returns the limits for the given tier, defaulting to STARTER.
func GetTierLimits(tier string) TierLimits {
	if t, ok := Tiers[tier]; ok {
		return t
	}
	return Tiers["STARTER"]
}
