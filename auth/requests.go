package auth

type RegisterRequest struct {
	Username   string
	Password   string
	Email      string
	Mobile     string
	ClientType string
}

type PasswordLoginRequest struct {
	Username   string
	Password   string
	ClientType string
}

type OTPRequest struct {
	Mobile     string
	ClientType string
}

type OTPVerifyRequest struct {
	Mobile     string
	Code       string
	ClientType string
}
