package dto

type RegisterRequest struct {
	PhoneNumber string  `json:"phone_number"`
	Email       *string `json:"email"`
	Password    string  `json:"password"`
	FullName    string  `json:"full_name"`
	Role        string  `json:"role"`
}

type RegisterResponse struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	FullName  string `json:"full_name"`
	CreatedAt string `json:"created_at"`
}

type LoginRequest struct {
	PhoneOrEmail string `json:"phone_or_email"`
	Password     string `json:"password"`
}

type UserInfo struct {
	ID           string  `json:"id"`
	Role         string  `json:"role"`
	FullName     string  `json:"full_name"`
	DriverStatus *string `json:"driver_status"`
}

type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	User         UserInfo `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}
