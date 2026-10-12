# KỊCH BẢN KIỂM THỬ DỊCH VỤ THANH TOÁN (PAYMENT-SERVICE TEST SUITE)

> **Mã tài liệu:** `TEST-PAYMENT` | **Phiên bản:** `1.0`  
> **Dịch vụ kiểm thử:** `payment-service` (Port nội bộ: `8005`, Dev host: `127.0.0.1:8005`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD payment-service](../03-lld/payment-service.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

Bộ kịch bản kiểm thử bao gồm **18 ca kiểm thử HTTP** (chạy tự động qua Postman) và **4 ca kiểm thử bất đồng bộ/sự kiện/PEL** (chạy tay qua `redis-cli` và `psql`):
1. **Kiểm tra sức khỏe:** Endpoint `GET /health` (`W-00`).
2. **Nạp tiền & Xem số dư:** Khách hàng C1 nạp tiền (`W-01`), xem ví cá nhân (`W-02`), xem lịch sử giao dịch (`W-03`).
3. **Kiểm tra số dư nội bộ (dispatch-service):** Kiểm tra khi đủ tiền (`W-04a`) và khi không đủ tiền (`W-04b`).
4. **Kiểm soát truy cập (RBAC):** Chặn Quản trị viên (Admin) gọi API ví cá nhân (`W-05a`, `W-05b`, `W-05c`).
5. **Kiểm tra ràng buộc dữ liệu (Validation):** Nạp tiền sai số tiền, kiểm tra số dư sai input, phân trang không hợp lệ (`W-06a`..`W-06g`).
6. **Xác thực Header:** Bắt lỗi thiếu header, UUID sai định dạng, role không hợp lệ (`W-07a`..`W-07c`).
7. **Thanh toán tự động TripCompleted:** Trừ ví khách, cộng ví tài xế, trích hoa hồng, bảo toàn dòng tiền (`W-08`).
8. **Chống lặp thanh toán (Idempotency):** Replay cùng sự kiện TripCompleted không ghi trùng giao dịch (`W-09`).
9. **Xử lý số dư khách không đủ:** Ghi dòng FAILED, không trừ âm ví khách, không cộng ví tài xế (`W-10`).
10. **Phục hồi sự kiện tồn đọng (PEL Recovery):** Ngắt kết nối PostgreSQL khi nhận sự kiện, khôi phục DB và kiểm chứng consumer tự xử lý lại tin trong PEL (`W-11`).

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Method & Path | Headers | Body / Tham số | Kỳ vọng HTTP Status & Mã lỗi | Ghi chú & Dữ liệu tham chiếu |
|:---:|---|---|---|---|---|---|
| **W-00** | Kiểm tra trạng thái dịch vụ (Healthcheck) | `GET /health` | *(None)* | *(None)* | **200 OK**<br>`{"data":{"status":"ok"},"success":true}` | Chạy bằng Postman. Endpoint kiểm tra liveness/readiness container. |
| **W-01** | Khách hàng C1 nạp tiền vào ví | `POST /api/v1/wallets/top-up` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"amount": 50000}` | **200 OK**<br>`added_amount = 50000`, `new_balance` tăng đúng 50.000 | Chạy bằng Postman. Kiểm tra chênh lệch số dư trước/sau nạp tiền. |
| **W-02** | Khách hàng C1 xem thông tin ví cá nhân | `GET /api/v1/wallets/me` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **200 OK**<br>`wallet_id`, `balance > 0`, `currency = "VND"` | Chạy bằng Postman. Tạo ví lazy nếu chưa có. |
| **W-03** | Khách hàng C1 xem lịch sử giao dịch | `GET /api/v1/wallets/transactions?page=1&limit=20` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **200 OK**<br>`transactions` (mảng 6 trường), không có `COMMISSION` | Chạy bằng Postman. Bổ sung Lượt 6: không trả trường `note`. |
| **W-04a**| Dispatch kiểm tra số dư C1 (Đủ tiền) | `POST /internal/v1/wallets/check-balance` | `Content-Type: application/json` | `{"user_id": "{{c1_id}}", "required_amount": 50000}` | **200 OK**<br>`sufficient = true`, `balance >= 50000` | Chạy bằng Postman. Endpoint nội bộ cho dispatch-service. |
| **W-04b**| Dispatch kiểm tra số dư C3 (Thiếu tiền) | `POST /internal/v1/wallets/check-balance` | `Content-Type: application/json` | `{"user_id": "{{c3_id}}", "required_amount": 50000}` | **200 OK**<br>`sufficient = false`, `balance = 0` | Chạy bằng Postman. C3 chưa nạp tiền, coi như balance = 0. |
| **W-05a**| Admin cố xem thông tin ví cá nhân | `GET /api/v1/wallets/me` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Chạy bằng Postman. Admin không sở hữu ví. |
| **W-05b**| Admin cố gọi nạp tiền | `POST /api/v1/wallets/top-up` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"amount": 100000}` | **403 Forbidden**<br>`FORBIDDEN` | Chạy bằng Postman. Admin không sở hữu ví. |
| **W-05c**| Admin cố xem lịch sử giao dịch | `GET /api/v1/wallets/transactions` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Chạy bằng Postman. Admin không sở hữu ví. |
| **W-06a**| Nạp tiền với `amount = 0` | `POST /api/v1/wallets/top-up` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"amount": 0}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"Số tiền nạp phải lớn hơn 0"*. |
| **W-06b**| Nạp tiền vượt `TOPUP_MAX_AMOUNT` | `POST /api/v1/wallets/top-up` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"amount": 100000001}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Vượt trần 100.000.000 VND. |
| **W-06c**| Check-balance thiếu `required_amount` | `POST /internal/v1/wallets/check-balance` | `Content-Type: application/json` | `{"user_id": "{{c1_id}}"}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Phân biệt `nil` với `0`. |
| **W-06d**| Check-balance có `required_amount < 0` | `POST /internal/v1/wallets/check-balance` | `Content-Type: application/json` | `{"user_id": "{{c1_id}}", "required_amount": -1000}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi số tiền âm. |
| **W-06e**| Check-balance với `user_id` không phải UUID | `POST /internal/v1/wallets/check-balance` | `Content-Type: application/json` | `{"user_id": "not-a-uuid", "required_amount": 50000}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi cú pháp UUID. |
| **W-06f**| Phân trang transactions với `page < 1` | `GET /api/v1/wallets/transactions?page=0&limit=20` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"page phải lớn hơn hoặc bằng 1"*. |
| **W-06g**| Phân trang transactions với `limit > 100` | `GET /api/v1/wallets/transactions?page=1&limit=101` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"limit phải nằm trong khoảng [1, 100]"*. |
| **W-07a**| Gọi API ví thiếu header xác thực | `GET /api/v1/wallets/me` | *(None)* | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy bằng Postman. Thiếu `X-User-Id` / `X-User-Role`. |
| **W-07b**| Gọi API ví với `X-User-Id` sai định dạng | `GET /api/v1/wallets/me` | `X-User-Id: invalid-uuid`<br>`X-User-Role: CUSTOMER` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy bằng Postman. Quy ước bổ sung: UUID sai $\rightarrow$ 401. |
| **W-07c**| Gọi API ví với `X-User-Role` không hợp lệ | `GET /api/v1/wallets/me` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: HACKER` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy bằng Postman. Role không thuộc tập hợp cho phép. |
| **W-08** | Xử lý thanh toán TripCompleted thành công | *(Redis XADD)* | *(None)* | `trip_id`, `customer_id`, `driver_id`, `fare=50000` | **3 dòng sổ DB**<br>`TRIP_PAYMENT`, `TRIP_INCOME`, `COMMISSION` | **Ca CHẠY TAY**. Hoa hồng 7.500, tài xế nhận 42.500, khách bị trừ 50.000. |
| **W-09** | Replay cùng sự kiện TripCompleted | *(Redis XADD)* | *(None)* | CÙNG `trip_id` với ca W-08 | **Không tăng dòng sổ**<br>`skipping (idempotent)` | **Ca CHẠY TAY**. Đảm bảo Idempotency tuyệt đối. |
| **W-10** | TripCompleted khi khách thiếu tiền (C3) | *(Redis XADD)* | *(None)* | `customer_id: C3`, `fare=50000` | **1 dòng FAILED**<br>`balance_C3 = 0` (không âm) | **Ca CHẠY TAY**. Ghi dòng FAILED, không cộng tài xế. |
| **W-11** | Phục hồi xử lý PEL khi PostgreSQL khởi động lại | *(Docker & Redis)* | *(None)* | XADD khi Postgres stop, sau đó start lại | **XPENDING = 0**<br>`3 dòng sổ được tạo` | **Ca CHẠY TAY**. Chứng minh cơ chế phục hồi PEL 2–5s. |

---

## 3. HƯỚNG DẪN THỰC THI & KỊCH BẢN CHẠY TAY

### 3.1. Chạy tự động qua Postman
Các ca `W-00` đến `W-07c` được tích hợp sẵn trong folder `04 payment-service` của Postman collection. Chạy trực tiếp sau khi hoàn tất folder `00 Seed`.

### 3.2. Ca chạy tay kiểm tra luồng sự kiện Redis Streams & PostgreSQL (`W-08`..`W-11`)

#### Bước 1: Thanh toán TripCompleted hợp lệ (`W-08`)
```powershell
$now = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$tripId = [guid]::NewGuid().ToString()
docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCompleted version "1.0" trip_id $tripId customer_id "<c1_id>" driver_id "<d1_id>" fare 50000 completed_at $now

# Kiểm tra log và 3 dòng sổ trong DB:
docker logs --tail 5 payment-service
docker exec postgres psql -U payment_user -d paymentdb -c "SELECT id, trip_id, type, amount, status FROM transactions WHERE trip_id = '$tripId';"
```

#### Bước 2: Replay sự kiện kiểm tra Idempotency (`W-09`)
```powershell
# Gửi lại ĐÚNG trip_id vừa tạo ở Bước 1:
docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCompleted version "1.0" trip_id $tripId customer_id "<c1_id>" driver_id "<d1_id>" fare 50000 completed_at $now

# Kiểm tra log ghi nhận skipping:
docker logs --tail 3 payment-service
# Kiểm tra số bản ghi trong DB vẫn đúng 3:
docker exec postgres psql -U payment_user -d paymentdb -c "SELECT count(*) FROM transactions WHERE trip_id = '$tripId';"
```

#### Bước 3: Khách hàng thiếu tiền (`W-10`)
```powershell
$tripFailId = [guid]::NewGuid().ToString()
docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCompleted version "1.0" trip_id $tripFailId customer_id "<c3_id>" driver_id "<d1_id>" fare 50000 completed_at $now

# Kiểm tra log ERROR và dòng sổ FAILED:
docker logs --tail 3 payment-service
docker exec postgres psql -U payment_user -d paymentdb -c "SELECT id, trip_id, type, amount, status, note FROM transactions WHERE trip_id = '$tripFailId';"
docker exec postgres psql -U payment_user -d paymentdb -c "SELECT balance FROM wallets WHERE user_id = '<c3_id>';"
```

#### Bước 4: Kiểm tra phục hồi sự kiện PEL (`W-11`)
```powershell
# 1. Tạm dừng PostgreSQL
docker stop postgres

# 2. Bắn sự kiện TripCompleted
$tripPelId = [guid]::NewGuid().ToString()
docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCompleted version "1.0" trip_id $tripPelId customer_id "<c1_id>" driver_id "<d1_id>" fare 50000 completed_at $now

# 3. Kiểm tra tin nhắn nằm trong PEL
docker exec redis redis-cli -a redis_secret_pass XPENDING stream:trip_events payment-group

# 4. Khởi động lại PostgreSQL
docker start postgres
Start-Sleep -Seconds 5

# 5. Kiểm tra PEL đã được giải phóng và DB đã ghi nhận
docker exec redis redis-cli -a redis_secret_pass XPENDING stream:trip_events payment-group
docker exec postgres psql -U payment_user -d paymentdb -c "SELECT id, trip_id, type, amount, status FROM transactions WHERE trip_id = '$tripPelId';"
```

---

## 4. DỌN DẸP TRẠNG THÁI (TEARDOWN / IDEMPOTENT)

- Mỗi lần chạy kịch bản sử dụng `trip_id` dạng UUID mới sinh ngẫu nhiên (`[guid]::NewGuid()`), đảm bảo kịch bản có thể chạy lặp lại vô hạn lần mà không gây xung đột khóa Unique Constraint.
- Để kiểm tra số dư ví, kịch bản Postman so khớp dựa trên **chênh lệch trước/sau** (`new_balance - old_balance`), không so sánh số tuyệt đối để giữ tính độc lập giữa các lượt chạy.
