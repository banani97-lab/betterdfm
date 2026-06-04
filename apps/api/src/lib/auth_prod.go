//go:build !dev

package lib

// Production build: the dev auth bypass does not exist. devUserClaims and
// devAdminClaims return nil so any code path that would have bypassed auth
// instead fails closed. The server also hard-fails at startup when JWT_ISSUER
// is empty (see cmd/server/main.go), so this is defense in depth.

// DevAuthBypassEnabled reports that this binary has NO dev auth bypass.
const DevAuthBypassEnabled = false

func devUserClaims() *UserClaims { return nil }

func devAdminClaims() *AdminClaims { return nil }
