//go:build dev

package lib

// This file is compiled ONLY into dev builds (go build -tags dev). It provides
// the local auth bypass used when JWT_ISSUER is empty. It is physically absent
// from production binaries, which are built without the dev tag.

// DevAuthBypassEnabled reports that this binary contains the dev auth bypass.
const DevAuthBypassEnabled = true

func devUserClaims() *UserClaims {
	return &UserClaims{Sub: "dev-user", Email: "dev@localhost", OrgID: "default-org", Role: "ADMIN"}
}

func devAdminClaims() *AdminClaims {
	return &AdminClaims{Sub: "dev-admin", Email: "admin@localhost"}
}
