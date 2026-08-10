package context

import "rabhana/pkg/config"

type AuthConfig struct {
	JWTSecret            string
	JWTExpirationMinutes int
	OTPExpirationMinutes int
	OTPLength            int
	PasswordMinLength    int
	PasswordMaxLength    int
	MinInterests         int
	PhoneRegex           string
	RequireDocuments     bool
}

func LoadAuthConfig() *AuthConfig {
	return &AuthConfig{
		JWTSecret:            config.GetEnv("JWT_SECRET", ""),
		JWTExpirationMinutes: config.GetEnvAsInt("JWT_EXPIRATION_TIME_MINUTES", 525600),
		OTPExpirationMinutes: config.GetEnvAsInt("OTP_EXPIRATION_TIME_MINUTES", 5),
		OTPLength:            6,
		PasswordMinLength:    8,
		PasswordMaxLength:    16,
		MinInterests:         1,
		PhoneRegex:           `^01[0125]\d{8}$`,
		// Off by default: document verification gates nothing functionally —
		// the manual subscription grant is the real checkpoint.
		RequireDocuments: config.GetEnvAsBool("REQUIRE_DOCUMENTS", false),
	}
}
