package context

import "rabhana/db/sqlc"

type AuthContext struct {
	Config  *AuthConfig
	Queries *sqlc.Queries
}

func NewAuthContext(cfg *AuthConfig, queries *sqlc.Queries) *AuthContext {
	return &AuthContext{Config: cfg, Queries: queries}
}
