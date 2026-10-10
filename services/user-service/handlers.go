package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AppHandler struct {
	db  *gorm.DB
	rdb *redis.Client
	cfg *Config
}

func newAppHandler(db *gorm.DB, rdb *redis.Client, cfg *Config) *AppHandler {
	return &AppHandler{
		db:  db,
		rdb: rdb,
		cfg: cfg,
	}
}

func sendSuccess(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
		"error":   nil,
	})
}

func sendError(c *fiber.Ctx, status int, code string, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"data":    nil,
		"error": fiber.Map{
			"code":    code,
			"message": message,
		},
	})
}

// GET /health
func (h *AppHandler) Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "ok",
	})
}

// POST /api/v1/auth/register
type RegisterRequest struct {
	PhoneNumber string  `json:"phone_number"`
	Email       *string `json:"email"`
	Password    string  `json:"password"`
	FullName    string  `json:"full_name"`
	Role        string  `json:"role"`
}

func (h *AppHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu JSON không hợp lệ")
	}

	req.PhoneNumber = strings.TrimSpace(req.PhoneNumber)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Role = strings.TrimSpace(req.Role)

	if req.PhoneNumber == "" {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Số điện thoại không được để trống")
	}
	if req.FullName == "" {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Họ và tên không được để trống")
	}
	if len(req.Password) < 6 {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Mật khẩu phải từ 6 ký tự trở lên")
	}
	if req.Role != "CUSTOMER" && req.Role != "DRIVER" {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Vai trò phải là CUSTOMER hoặc DRIVER")
	}

	var cleanEmail *string
	if req.Email != nil {
		em := strings.TrimSpace(*req.Email)
		if em != "" {
			if !strings.Contains(em, "@") || !strings.Contains(em, ".") {
				return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Email không đúng định dạng")
			}
			cleanEmail = &em
		}
	}

	// Kiểm tra trùng lặp phone_number hoặc email
	var existing User
	q := h.db.Where("phone_number = ?", req.PhoneNumber)
	if cleanEmail != nil {
		q = h.db.Where("phone_number = ? OR email = ?", req.PhoneNumber, *cleanEmail)
	}
	if err := q.First(&existing).Error; err == nil {
		return sendError(c, fiber.StatusConflict, "USER_ALREADY_EXISTS", "Số điện thoại hoặc email đã tồn tại trong hệ thống")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		return sendError(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Lỗi mã hóa mật khẩu")
	}

	var driverStatus *string
	if req.Role == "DRIVER" {
		status := "OFFLINE"
		driverStatus = &status
	}

	now := time.Now().UTC()
	user := User{
		ID:           uuid.New(),
		PhoneNumber:  &req.PhoneNumber,
		Email:        cleanEmail,
		PasswordHash: string(hashedPassword),
		FullName:     req.FullName,
		Role:         req.Role,
		DriverStatus: driverStatus,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.db.Create(&user).Error; err != nil {
		return sendError(c, fiber.StatusConflict, "USER_ALREADY_EXISTS", "Số điện thoại hoặc email đã tồn tại trong hệ thống")
	}

	return sendSuccess(c, fiber.StatusCreated, fiber.Map{
		"user_id":    user.ID,
		"role":       user.Role,
		"full_name":  user.FullName,
		"created_at": user.CreatedAt.Format(time.RFC3339),
	})
}

// POST /api/v1/auth/login
type LoginRequest struct {
	PhoneOrEmail string `json:"phone_or_email"`
	Password     string `json:"password"`
}

func (h *AppHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu JSON không hợp lệ")
	}

	req.PhoneOrEmail = strings.TrimSpace(req.PhoneOrEmail)
	if req.PhoneOrEmail == "" || req.Password == "" {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Vui lòng nhập đầy đủ tài khoản và mật khẩu")
	}

	var user User
	err := h.db.Where("phone_number = ? OR email = ?", req.PhoneOrEmail, req.PhoneOrEmail).First(&user).Error
	if err != nil {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Số điện thoại/email hoặc mật khẩu không chính xác")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Số điện thoại/email hoặc mật khẩu không chính xác")
	}

	jti := uuid.New().String()
	now := time.Now().UTC()
	exp := now.Add(time.Duration(h.cfg.AccessTokenExpiry) * time.Second).Unix()

	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"role":    user.Role,
		"exp":     exp,
		"jti":     jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		slog.Error("failed to sign JWT", "error", err)
		return sendError(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Lỗi tạo token")
	}

	refreshToken := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = h.rdb.Set(ctx, "refresh:"+refreshToken, user.ID.String(), time.Duration(h.cfg.RefreshTokenExpiry)*time.Second).Err()
	if err != nil {
		slog.Error("failed to save refresh token in redis", "error", err)
		return sendError(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Lỗi lưu trữ phiên")
	}

	return sendSuccess(c, fiber.StatusOK, fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    h.cfg.AccessTokenExpiry,
		"user": fiber.Map{
			"id":            user.ID,
			"role":          user.Role,
			"full_name":     user.FullName,
			"driver_status": user.DriverStatus,
		},
	})
}

// POST /api/v1/auth/refresh
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AppHandler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu JSON không hợp lệ")
	}

	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Thiếu refresh_token")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Thu hồi nguyên tử bằng GETDEL
	userIDStr, err := h.rdb.GetDel(ctx, "refresh:"+req.RefreshToken).Result()
	if err != nil || userIDStr == "" {
		return sendError(c, fiber.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token không tồn tại hoặc đã được sử dụng")
	}

	var user User
	if err := h.db.Where("id = ?", userIDStr).First(&user).Error; err != nil {
		return sendError(c, fiber.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Người dùng không tồn tại")
	}

	newJTI := uuid.New().String()
	now := time.Now().UTC()
	exp := now.Add(time.Duration(h.cfg.AccessTokenExpiry) * time.Second).Unix()

	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"role":    user.Role,
		"exp":     exp,
		"jti":     newJTI,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	newAccessToken, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		slog.Error("failed to sign new JWT", "error", err)
		return sendError(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Lỗi tạo token")
	}

	newRefreshToken := uuid.New().String()
	err = h.rdb.Set(ctx, "refresh:"+newRefreshToken, user.ID.String(), time.Duration(h.cfg.RefreshTokenExpiry)*time.Second).Err()
	if err != nil {
		slog.Error("failed to save new refresh token in redis", "error", err)
		return sendError(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Lỗi lưu trữ phiên")
	}

	return sendSuccess(c, fiber.StatusOK, fiber.Map{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
		"expires_in":    h.cfg.AccessTokenExpiry,
	})
}

// POST /api/v1/auth/logout
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AppHandler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu JSON không hợp lệ")
	}

	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Thiếu refresh_token")
	}

	userID := c.Get("X-User-Id")
	userRole := c.Get("X-User-Role")
	if userID == "" || userRole == "" {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Thiếu header xác thực X-User-Id hoặc X-User-Role")
	}

	authHeader := c.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Thiếu hoặc sai định dạng Authorization header")
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	parsedToken, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(h.cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil || !parsedToken.Valid {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Access token không hợp lệ")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Access token không hợp lệ")
	}

	tokenUserID, _ := claims["user_id"].(string)
	if tokenUserID != userID {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Token không khớp với người dùng")
	}

	jti, _ := claims["jti"].(string)
	expFloat, _ := claims["exp"].(float64)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Kiểm tra Refresh Token trong Redis
	storedUserID, err := h.rdb.Get(ctx, "refresh:"+req.RefreshToken).Result()
	if err != nil || storedUserID != userID {
		return sendError(c, fiber.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh token không tồn tại hoặc không chính chủ")
	}

	// 2. Nếu là DRIVER: kiểm tra không BUSY rồi chuyển OFFLINE
	if userRole == "DRIVER" {
		res := h.db.Exec("UPDATE users SET driver_status = 'OFFLINE', updated_at = now() WHERE id = ? AND driver_status != 'BUSY'", userID)
		if res.Error != nil {
			return sendError(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Lỗi cập nhật trạng thái tài xế")
		}
		if res.RowsAffected == 0 {
			var u User
			if err := h.db.Where("id = ?", userID).First(&u).Error; err == nil && u.DriverStatus != nil && *u.DriverStatus == "BUSY" {
				return sendError(c, fiber.StatusBadRequest, "DRIVER_CANNOT_LOGOUT", "Tài xế đang trong chuyến đi, không thể đăng xuất")
			}
		}
	}

	// 3. Xóa Refresh Token
	h.rdb.Del(ctx, "refresh:"+req.RefreshToken)

	// 4. Blacklist Access Token
	nowUnix := time.Now().UTC().Unix()
	ttlSec := int64(expFloat) - nowUnix
	if ttlSec <= 0 {
		ttlSec = 1
	}
	if jti != "" {
		h.rdb.Set(ctx, "blacklist:"+jti, "revoked", time.Duration(ttlSec)*time.Second)
	}

	// 5. Phát tín hiệu ngắt kết nối WebSocket
	wsControlMsg, _ := json.Marshal(map[string]string{
		"action":  "DISCONNECT",
		"user_id": userID,
	})
	h.rdb.Publish(ctx, "ride:ws_control", string(wsControlMsg))

	return sendSuccess(c, fiber.StatusOK, fiber.Map{
		"message": "Đăng xuất thành công",
	})
}

// PATCH /api/v1/driver/status
type DriverStatusRequest struct {
	Status string `json:"status"`
}

func (h *AppHandler) UpdateDriverStatus(c *fiber.Ctx) error {
	userRole := c.Get("X-User-Role")
	if userRole != "DRIVER" {
		return sendError(c, fiber.StatusForbidden, "FORBIDDEN", "Chỉ tài xế mới có quyền cập nhật trạng thái")
	}

	driverID := c.Get("X-User-Id")
	if driverID == "" {
		return sendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Thiếu header X-User-Id")
	}

	var req DriverStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu JSON không hợp lệ")
	}

	req.Status = strings.TrimSpace(req.Status)
	if req.Status != "ONLINE" && req.Status != "OFFLINE" {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Trạng thái phải là ONLINE hoặc OFFLINE")
	}

	res := h.db.Exec("UPDATE users SET driver_status = ?, updated_at = now() WHERE id = ? AND role = 'DRIVER' AND driver_status != 'BUSY'", req.Status, driverID)
	if res.Error != nil {
		return sendError(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Lỗi cập nhật trạng thái tài xế")
	}

	if res.RowsAffected == 0 {
		var u User
		if err := h.db.Where("id = ?", driverID).First(&u).Error; err == nil && u.DriverStatus != nil && *u.DriverStatus == "BUSY" {
			return sendError(c, fiber.StatusBadRequest, "DRIVER_BUSY", "Tài xế đang bận chuyến xe, không thể chuyển đổi trạng thái")
		}
		return sendError(c, fiber.StatusBadRequest, "DRIVER_BUSY", "Không thể cập nhật trạng thái tài xế")
	}

	return sendSuccess(c, fiber.StatusOK, fiber.Map{
		"driver_id": driverID,
		"status":    req.Status,
	})
}

// POST /internal/v1/users/filter-online
type FilterOnlineRequest struct {
	DriverIDs []string `json:"driver_ids"`
}

func (h *AppHandler) FilterOnline(c *fiber.Ctx) error {
	var req FilterOnlineRequest
	if err := c.BodyParser(&req); err != nil || req.DriverIDs == nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu driver_ids không hợp lệ")
	}

	if len(req.DriverIDs) == 0 {
		return sendSuccess(c, fiber.StatusOK, fiber.Map{
			"online_driver_ids": []string{},
		})
	}

	if len(req.DriverIDs) > 500 {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Mảng driver_ids vượt quá giới hạn tối đa 500 phần tử")
	}

	parsedUUIDs := make([]uuid.UUID, 0, len(req.DriverIDs))
	for _, idStr := range req.DriverIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Phần tử không phải UUID hợp lệ: "+idStr)
		}
		parsedUUIDs = append(parsedUUIDs, id)
	}

	var onlineIDs []string
	err := h.db.Model(&User{}).
		Where("id IN ? AND role = 'DRIVER' AND driver_status = 'ONLINE'", parsedUUIDs).
		Pluck("id", &onlineIDs).Error
	if err != nil {
		return sendError(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Lỗi truy vấn cơ sở dữ liệu")
	}

	if onlineIDs == nil {
		onlineIDs = []string{}
	}

	return sendSuccess(c, fiber.StatusOK, fiber.Map{
		"online_driver_ids": onlineIDs,
	})
}

// POST /internal/v1/drivers/:id/status
type DriverInternalStatusRequest struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

func (h *AppHandler) UpdateDriverInternalStatus(c *fiber.Ctx) error {
	idStr := c.Params("id")
	if _, err := uuid.Parse(idStr); err != nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "ID tài xế không phải UUID hợp lệ")
	}

	var req DriverInternalStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu JSON không hợp lệ")
	}

	req.FromStatus = strings.TrimSpace(req.FromStatus)
	req.ToStatus = strings.TrimSpace(req.ToStatus)

	isValidPair := (req.FromStatus == "ONLINE" && req.ToStatus == "BUSY") || (req.FromStatus == "BUSY" && req.ToStatus == "ONLINE")
	if !isValidPair {
		return sendError(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Cặp chuyển đổi trạng thái không hợp lệ")
	}

	res := h.db.Exec("UPDATE users SET driver_status = ?, updated_at = now() WHERE id = ? AND role = 'DRIVER' AND driver_status = ?", req.ToStatus, idStr, req.FromStatus)
	if res.Error != nil {
		return sendError(c, fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Lỗi cập nhật trạng thái tài xế")
	}

	if res.RowsAffected == 0 {
		return sendError(c, fiber.StatusBadRequest, "DRIVER_NOT_AVAILABLE", "Trạng thái hiện tại khác from_status hoặc không tìm thấy tài xế")
	}

	return sendSuccess(c, fiber.StatusOK, fiber.Map{
		"driver_id": idStr,
		"status":    req.ToStatus,
	})
}
