package dto

type TopUpRequest struct {
	Amount int64 `json:"amount"`
}

type TopUpResponse struct {
	WalletID    string `json:"wallet_id"`
	AddedAmount int64  `json:"added_amount"`
	NewBalance  int64  `json:"new_balance"`
}

type WalletMeResponse struct {
	WalletID string `json:"wallet_id"`
	Balance  int64  `json:"balance"`
	Currency string `json:"currency"`
}

type CheckBalanceRequest struct {
	UserID         string `json:"user_id"`
	RequiredAmount *int64 `json:"required_amount"`
}

type CheckBalanceResponse struct {
	Sufficient bool  `json:"sufficient"`
	Balance    int64 `json:"balance"`
}
