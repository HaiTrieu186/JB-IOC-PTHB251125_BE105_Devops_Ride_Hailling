package dto

type TransactionItemDTO struct {
	ID        int64   `json:"id"`
	TripID    *string `json:"trip_id"`
	Type      string  `json:"type"`
	Amount    int64   `json:"amount"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

type TransactionListResponse struct {
	Transactions []TransactionItemDTO `json:"transactions"`
	Total        int64                `json:"total"`
	Page         int                  `json:"page"`
}
