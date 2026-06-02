package routes

import (
	"errors"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// defaultProfileRulesJSON is the canonical default capability-profile rule set.
// Keep in sync with the frontend DEFAULT_RULES in
// apps/web/src/app/admin/profile/page.tsx.
const defaultProfileRulesJSON = `{"minTraceWidthMM":0.15,"minClearanceMM":0.15,"minDrillDiamMM":0.3,"maxDrillDiamMM":6.3,"minAnnularRingMM":0.15,"maxAspectRatio":10,"minSolderMaskDamMM":0.1,"minEdgeClearanceMM":0.3,"minDrillToDrillMM":0.25,"minDrillToCopperMM":0.25,"minCopperSliverMM":0.1,"smallestPackageClass":"","maxTraceImbalanceRatio":2.0,"enableSilkscreenOnPadCheck":true,"maxComponentHeightTopMM":10,"maxComponentHeightBottomMM":5,"minComponentSpacingMM":0.5,"flagThroughHoleOnBottom":true,"minMountingHoleKeepoutMM":0.5}`

// getOrCreateDefaultProfile returns the org's default capability profile,
// creating one from defaultProfileRulesJSON if none exists. Centralizes logic
// that was previously copy-pasted across submission/batch handlers.
func getOrCreateDefaultProfile(database *gorm.DB, orgID string) (db.CapabilityProfile, error) {
	var profile db.CapabilityProfile
	err := database.Where("org_id = ? AND is_default = ?", orgID, true).First(&profile).Error
	if err == nil {
		return profile, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return profile, err
	}

	now := time.Now()
	profile = db.CapabilityProfile{
		ID:        uuid.New().String(),
		OrgID:     orgID,
		Name:      "Default",
		IsDefault: true,
		Rules:     []byte(defaultProfileRulesJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.Create(&profile).Error; err != nil {
		return profile, err
	}
	return profile, nil
}
