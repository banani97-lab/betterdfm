package lib

import (
	"os"
	"strings"
)

// RequireNonCUIAck reports whether uploads must carry an explicit
// acknowledgment that the design is NOT export-controlled (ITAR) or CUI.
//
// This is the alpha guardrail: while RapidDFM is not yet assessed to a
// FedRAMP Moderate equivalency body of evidence, it must not accept
// controlled technical data. The gate is enforced on every upload entry
// point (direct, batch, and shared-portal) and recorded per submission.
//
// It is controlled by NON_CUI_ALPHA_MODE and fails SAFE: the gate is ON
// unless the variable is explicitly set to a falsey value. Once equivalency
// is in place, set NON_CUI_ALPHA_MODE=false to lift the guardrail.
func RequireNonCUIAck() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NON_CUI_ALPHA_MODE"))) {
	case "false", "0", "off", "no", "disabled":
		return false
	default:
		return true
	}
}

// ErrNonCUIAckRequired is the user-facing message returned when an upload is
// rejected for missing the non-CUI acknowledgment.
const ErrNonCUIAckRequired = "This alpha does not accept ITAR-controlled or CUI / export-controlled technical data. You must confirm the design is not export-controlled before uploading."
