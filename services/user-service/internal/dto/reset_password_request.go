package dto

// ForgotPasswordRequest is sent by the client to trigger a reset token email.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest is sent by the client with the token received via email
// and the new password they want to set.
type ResetPasswordRequest struct {
	Token       string `json:"token"        binding:"required"`
	NewPassword string `json:"new_password" binding:"required,password"`
}
