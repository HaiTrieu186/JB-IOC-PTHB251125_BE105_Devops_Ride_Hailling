package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"payment-service/internal/dto"
	"payment-service/internal/exception"
	"payment-service/internal/repository"
	"payment-service/internal/service"
)

type walletServiceImpl struct {
	walletRepo      repository.WalletRepository
	transactionRepo repository.TransactionRepository
	topupMaxAmount  int64
}

func NewWalletService(
	walletRepo repository.WalletRepository,
	transactionRepo repository.TransactionRepository,
	topupMaxAmount int64,
) service.WalletService {
	return &walletServiceImpl{
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		topupMaxAmount:  topupMaxAmount,
	}
}

func (s *walletServiceImpl) TopUp(ctx context.Context, userID uuid.UUID, amount int64) (*dto.TopUpResponse, *exception.AppError) {
	if amount <= 0 {
		return nil, exception.NewValidationError("Số tiền nạp phải lớn hơn 0")
	}
	if amount > s.topupMaxAmount {
		return nil, exception.NewValidationError(fmt.Sprintf("Số tiền nạp không được vượt quá %d VND", s.topupMaxAmount))
	}

	wallet, _, err := s.walletRepo.TopUp(ctx, userID, amount)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Không thể thực hiện nạp tiền", err)
	}

	return &dto.TopUpResponse{
		WalletID:    wallet.ID.String(),
		AddedAmount: amount,
		NewBalance:  wallet.Balance,
	}, nil
}

func (s *walletServiceImpl) GetMe(ctx context.Context, userID uuid.UUID) (*dto.WalletMeResponse, *exception.AppError) {
	wallet, err := s.walletRepo.EnsureWallet(ctx, userID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Không thể lấy thông tin ví", err)
	}

	return &dto.WalletMeResponse{
		WalletID: wallet.ID.String(),
		Balance:  wallet.Balance,
		Currency: "VND",
	}, nil
}

func (s *walletServiceImpl) GetTransactions(ctx context.Context, userID uuid.UUID, page, limit int) (*dto.TransactionListResponse, *exception.AppError) {
	if page < 1 {
		return nil, exception.NewValidationError("page phải lớn hơn hoặc bằng 1")
	}
	if limit < 1 || limit > 100 {
		return nil, exception.NewValidationError("limit phải nằm trong khoảng [1, 100]")
	}

	offset := (page - 1) * limit
	txs, total, err := s.transactionRepo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Không thể lấy danh sách giao dịch", err)
	}

	items := make([]dto.TransactionItemDTO, 0, len(txs))
	for _, t := range txs {
		var tripIDStr *string
		if t.TripID != nil {
			str := t.TripID.String()
			tripIDStr = &str
		}
		items = append(items, dto.TransactionItemDTO{
			ID:        t.ID,
			TripID:    tripIDStr,
			Type:      t.Type,
			Amount:    t.Amount,
			Status:    t.Status,
			CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	return &dto.TransactionListResponse{
		Transactions: items,
		Total:        total,
		Page:         page,
	}, nil
}

func (s *walletServiceImpl) CheckBalance(ctx context.Context, req dto.CheckBalanceRequest) (*dto.CheckBalanceResponse, *exception.AppError) {
	if req.RequiredAmount == nil {
		return nil, exception.NewValidationError("required_amount là bắt buộc")
	}
	if *req.RequiredAmount < 0 {
		return nil, exception.NewValidationError("required_amount không được âm")
	}

	userUUID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, exception.NewValidationError("user_id phải là UUID hợp lệ")
	}

	wallet, err := s.walletRepo.GetByUserID(ctx, userUUID)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi kiểm tra số dư ví", err)
	}

	var balance int64 = 0
	if wallet != nil {
		balance = wallet.Balance
	}

	sufficient := balance >= *req.RequiredAmount
	return &dto.CheckBalanceResponse{
		Sufficient: sufficient,
		Balance:    balance,
	}, nil
}
