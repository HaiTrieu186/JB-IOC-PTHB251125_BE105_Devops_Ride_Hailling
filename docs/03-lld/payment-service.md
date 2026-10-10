# THIẾT KẾ CHI TIẾT DỊCH VỤ THANH TOÁN (PAYMENT-SERVICE LLD)

> **Mã tài liệu:** `LLD-PAYMENT` | **Service:** `payment-service` | **Database:** `paymentdb` (PostgreSQL)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [SRS v1.5](../01-srs/srs.md), [Hợp đồng liên service](../03-lld/00-hop-dong-lien-service.md), [UC-05 (toàn bộ)](../02-use-cases/uc-05-vi-va-thanh-toan.md), [UC-03 (UC-17)](../02-use-cases/uc-03-dat-xe-va-ghep-chuyen.md)

---

## 1. TRÁCH NHIỆM

- Quản lý tài khoản ví ảo (`wallets`) cho Khách hàng và Tài xế, đảm bảo toàn vẹn số dư không âm.
- Cung cấp API nạp tiền demo (`top-up`), tra cứu số dư và xem lịch sử giao dịch cá nhân (`transactions`).
- Cung cấp API nội bộ `POST /internal/v1/wallets/check-balance` cho `dispatch-service` kiểm tra số dư trước khi đặt xe.
- Tiêu thụ sự kiện `TripCompleted` từ Redis Streams (`stream:trip_events`) qua consumer group `payment-group` để thực hiện khấu trừ ví khách, cộng ví tài xế và trích hoa hồng hệ thống nguyên tử.

---

## 2. BẢNG DB (PAYMENTDB)

### 2.1. Bảng `wallets` (Ví người dùng)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | PK, DEFAULT `gen_random_uuid()` | Định danh ví duy nhất toàn hệ thống. |
| `user_id` | `UUID` | NOT NULL, UNIQUE | Người dùng sở hữu ví (quan hệ 1-1). |
| `balance` | `BIGINT` | NOT NULL, DEFAULT `0`, CHECK `balance >= 0` | Số dư khả dụng (VND, số nguyên, không âm). |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian tạo ví UTC. |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian cập nhật số dư gần nhất UTC. |

- **Chỉ mục & Ràng buộc:** PK: `id`. UNIQUE: `user_id`. CHECK balance: `CHECK (balance >= 0)`.

### 2.2. Bảng `transactions` (Sổ giao dịch)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `BIGINT` | PK, GENERATED ALWAYS AS IDENTITY | Định danh dòng giao dịch tăng tự động. |
| `wallet_id` | `UUID` | NULLABLE, FK `wallets(id)` | Ví liên quan (dòng `COMMISSION` không có ví thì NULL). |
| `user_id` | `UUID` | NULLABLE | Người dùng liên quan (dòng `COMMISSION` thì NULL). |
| `trip_id` | `UUID` | NULLABLE | Mã chuyến đi phát sinh giao dịch (`TOPUP` thì NULL). |
| `type` | `VARCHAR(20)` | NOT NULL | `TRIP_PAYMENT`, `TRIP_INCOME`, `COMMISSION`, `TOPUP`. |
| `amount` | `BIGINT` | NOT NULL | Số tiền giao dịch (VND, số nguyên: âm hoặc dương). |
| `status` | `VARCHAR(10)` | NOT NULL | `SUCCESS`, `FAILED`. |
| `note` | `TEXT` | NULLABLE | Ghi chú lý do thất bại (ví dụ: `INSUFFICIENT_FUNDS_ON_COMPLETE`). |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian ghi nhận giao dịch UTC. |

- **Chỉ mục & Ràng buộc toàn vẹn:** Tạo bằng raw SQL theo quy ước Mục 5(l) Hợp đồng LLD-00 (không dùng GORM tag where).
  - PK: `id`.
  - CHECK type: `CHECK (type IN ('TRIP_PAYMENT', 'TRIP_INCOME', 'COMMISSION', 'TOPUP'))`.
  - CHECK status: `CHECK (status IN ('SUCCESS', 'FAILED'))`.
  - `idx_transactions_trip_type`: `CREATE UNIQUE INDEX ... ON transactions (trip_id, type) WHERE trip_id IS NOT NULL` (Đảm bảo Idempotency duy nhất theo chuyến và loại dòng).
  - `idx_transactions_user_created`: `CREATE INDEX ... ON transactions (user_id, created_at DESC)` (Tối ưu tra cứu lịch sử người dùng).

---

## 3. API SPECIFICATION

Định dạng JSON chuẩn: `{"success": bool, "data": ..., "error": {"code": string, "message": string}}`. Header `X-User-Id` và `X-User-Role` do Gateway chuyển tiếp vào cho các endpoint công khai.

### 3.1. API Công khai (Public Endpoints)

| Method & Path | Tầng | Actor | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `GET /api/v1/wallets/me` | 1 | `CUSTOMER`<br>`DRIVER` | *(None)* | `wallet_id` (uuid)<br>`balance` (int)<br>`currency` ("VND") | `FORBIDDEN` (403: role == ADMIN) |
| `POST /api/v1/wallets/top-up` | 1 | `CUSTOMER`<br>`DRIVER` | `amount` (int, B, $> 0$ và $\le \text{TOPUP\_MAX\_AMOUNT}$) | `wallet_id` (uuid)<br>`added_amount` (int)<br>`new_balance` (int) | `FORBIDDEN` (403: role == ADMIN)<br>`VALIDATION_ERROR` (400: amount $\le 0$ hoặc vượt trần `TOPUP_MAX_AMOUNT`) |
| `GET /api/v1/wallets/transactions` | 1 | `CUSTOMER`<br>`DRIVER` | Query: `page` (int, T, mặc định 1, $\ge 1$), `limit` (int, T, mặc định 20, $\in [1, 100]$) | `transactions` (array of `{id, trip_id, type, amount, status, created_at}`), `total` (int), `page` (int) | `FORBIDDEN` (403: role == ADMIN)<br>`VALIDATION_ERROR` (400: page $< 1$ hoặc limit ngoài $[1, 100]$) |
| `GET /health` *(do LLD đặt)* | 1 | Tất cả | *(None)* | `status` ("ok") | *(None)* |

*(B: Bắt buộc, T: Tùy chọn. Lưu ý: API transactions KHÔNG trả dòng type = 'COMMISSION' cho người dùng).*

### 3.2. API Nội bộ (Internal Endpoints)

| Method & Path | Tầng | Caller | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `POST /internal/v1/wallets/check-balance` | 1 | `dispatch-service` | `user_id` (uuid, B)<br>`required_amount` (int, B) | `sufficient` (bool)<br>`balance` (int) | `VALIDATION_ERROR` (400: thiếu trường hoặc required_amount $< 0$) |

*(Ghi chú: Nếu ví chưa có, lazy coi như balance = 0, sufficient = false. dispatch-service tự trả INSUFFICIENT_BALANCE khi sufficient = false).*

---

## 4. LUỒNG XỬ LÝ TỪNG BƯỚC

### 4.1. Nạp tiền demo (`POST /api/v1/wallets/top-up`) (UC-25)
1. Kiểm tra vai trò: `X-User-Role != 'ADMIN'` (nếu ADMIN $\rightarrow$ `FORBIDDEN` 403).
2. Validate số tiền: `amount > 0` và `amount <= TOPUP_MAX_AMOUNT` (mặc định 100.000.000 VND; vi phạm $\rightarrow$ `VALIDATION_ERROR` 400).
3. Tạo ví lazy cho `user_id` nếu chưa tồn tại:
   `INSERT INTO wallets (user_id, balance, created_at, updated_at) VALUES (<user_id>, 0, now(), now()) ON CONFLICT (user_id) DO NOTHING;`
4. Mở transaction DB:
   - `UPDATE wallets SET balance = balance + <amount>, updated_at = now() WHERE user_id = <user_id> RETURNING id, balance;`
   - `INSERT INTO transactions (wallet_id, user_id, trip_id, type, amount, status, created_at) VALUES (<wallet_id>, <user_id>, NULL, 'TOPUP', <amount>, 'SUCCESS', now());`
   - Commit transaction DB.
5. Trả về 200 OK với số dư mới.

### 4.2. Tra cứu Ví cá nhân (`GET /api/v1/wallets/me`) (UC-24)
1. Kiểm tra vai trò: `X-User-Role != 'ADMIN'` (nếu ADMIN $\rightarrow$ `FORBIDDEN` 403).
2. Tạo ví lazy nếu chưa có:
   `INSERT INTO wallets (user_id, balance, created_at, updated_at) VALUES (<user_id>, 0, now(), now()) ON CONFLICT (user_id) DO NOTHING;`
3. Truy vấn thông tin: `SELECT id, balance FROM wallets WHERE user_id = <user_id>;`
4. Trả về 200 OK với `{"wallet_id": id, "balance": balance, "currency": "VND"}`.

### 4.3. Tra cứu Lịch sử Giao dịch (`GET /api/v1/wallets/transactions`) (UC-24)
1. Kiểm tra vai trò: `X-User-Role != 'ADMIN'` (nếu ADMIN $\rightarrow$ `FORBIDDEN` 403).
2. Validate phân trang: `page >= 1` (mặc định 1), `limit` từ 1 đến 100 (mặc định 20). Nếu tham số không hợp lệ $\rightarrow$ `VALIDATION_ERROR` (400).
3. Truy vấn danh sách giao dịch từ PostgreSQL:
   - Điều kiện lọc: `WHERE user_id = <X-User-Id>` (tự động loại bỏ dòng hoa hồng hệ thống `COMMISSION` vì có `user_id IS NULL`).
   - Sắp xếp và phân trang: `ORDER BY created_at DESC LIMIT <limit> OFFSET <(page - 1) * limit>;`
   - Đếm tổng số bản ghi: `SELECT count(*) FROM transactions WHERE user_id = <X-User-Id>;`
4. Trả về 200 OK với `transactions: [...]`, `total`, `page`.

### 4.4. Kiểm tra số dư nội bộ (`POST /internal/v1/wallets/check-balance`)
1. Validate `user_id` và `required_amount >= 0`.
2. Truy vấn `SELECT balance FROM wallets WHERE user_id = <user_id>`.
3. Nếu không tìm thấy bản ghi $\rightarrow$ coi như `balance = 0`.
4. So khớp `sufficient = (balance >= required_amount)`.
5. Trả về 200 OK với `{"sufficient": bool, "balance": int}`.

### 4.5. Tiêu thụ sự kiện `TripCompleted` & Thanh toán tự động (UC-26)
1. **Phân loại sự kiện:** Đọc trực tiếp trường `type` của message từ `stream:trip_events`. Nếu `type` khác `TripCompleted` (`TripCreated`, `TripCancelled`, `TripExpired`) $\rightarrow$ Bỏ qua và gọi `XACK`.
2. **Kiểm tra message hỏng:** Nếu thiếu trường bắt buộc (`trip_id`, `customer_id`, `driver_id`, `fare`), UUID sai định dạng, `fare <= 0`, hoặc là entry rỗng (do tin nhắn bị xóa bởi `MAXLEN ~ 5000` của Stream khi đọc lại từ PEL) $\rightarrow$ log `ERROR` và gọi ngay `XACK stream:trip_events payment-group <msg_id>`.
3. **Kiểm tra tính lũy thừa (Idempotency):**
   `SELECT 1 FROM transactions WHERE trip_id = <trip_id> LIMIT 1;`  
   Nếu đã tồn tại bất kỳ bản ghi nào (bất kể `SUCCESS` hay `FAILED`) $\rightarrow$ Gửi ngay `XACK stream:trip_events payment-group <msg_id>`, bỏ qua xử lý.
4. **Tự tính toán tiền số nguyên (Mục 4 Hợp đồng LLD-00, không đọc commission từ event):**
   - Hoa hồng hệ thống: $\text{commission} = \lfloor\frac{\text{fare} \times \text{COMMISSION\_RATE} + 50}{100}\rfloor$
   - Thu nhập tài xế nhận: $\text{driver\_income} = \text{fare} - \text{commission}$
   *(Bất biến bảo toàn dòng tiền: $\text{driver\_income} + \text{commission} = \text{fare}$)*.
5. Mở transaction DB:
   - Tạo lazy ví khách và ví tài xế nếu chưa có (`ON CONFLICT DO NOTHING`).
   - **Khóa 2 ví theo thứ tự cố định để chống Deadlock:**  
     So sánh chuỗi định danh: nếu `customer_id < driver_id` thì khóa ví khách trước rồi đến ví tài xế; ngược lại khóa ví tài xế trước rồi đến ví khách:  
     `SELECT id, balance FROM wallets WHERE user_id = <first_id> FOR UPDATE;`  
     `SELECT id, balance FROM wallets WHERE user_id = <second_id> FOR UPDATE;`
6. **Phân nhánh xử lý số dư khách:**
   - **Trường hợp đủ tiền ($\text{balance}_{\text{khách}} \ge \text{fare}$):**
     1. Trừ ví khách: `UPDATE wallets SET balance = balance - <fare>, updated_at = now() WHERE user_id = <customer_id>;`
     2. Cộng ví tài xế: `UPDATE wallets SET balance = balance + <driver_income>, updated_at = now() WHERE user_id = <driver_id>;`
     3. Ghi 3 dòng sổ giao dịch duy nhất theo `(trip_id, type)`:
        - Dòng khách: `{wallet_id: <wallet_c>, user_id: <customer_id>, trip_id: <trip_id>, type: 'TRIP_PAYMENT', amount: -fare, status: 'SUCCESS'}`.
        - Dòng tài xế: `{wallet_id: <wallet_d>, user_id: <driver_id>, trip_id: <trip_id>, type: 'TRIP_INCOME', amount: +driver_income, status: 'SUCCESS'}`.
        - Dòng hoa hồng: `{wallet_id: NULL, user_id: NULL, trip_id: <trip_id>, type: 'COMMISSION', amount: +commission, status: 'SUCCESS'}`.
     4. Commit transaction DB.
     5. Gửi `XACK stream:trip_events payment-group <msg_id>`.
   - **Trường hợp bất thường thiếu tiền ($\text{balance}_{\text{khách}} < \text{fare}$):**
     1. Tuyệt đối không trừ âm ví khách và không cộng ví tài xế.
     2. Ghi nhận 1 dòng giao dịch thất bại:  
        `{wallet_id: <wallet_c>, user_id: <customer_id>, trip_id: <trip_id>, type: 'TRIP_PAYMENT', amount: -fare, status: 'FAILED', note: 'INSUFFICIENT_FUNDS_ON_COMPLETE'}`.
     3. Commit transaction DB.
     4. Log nghiêm trọng `ERROR`, gửi `XACK stream:trip_events payment-group <msg_id>`.
7. **Xử lý lỗi DB tạm thời:** Nếu transaction DB gặp lỗi tạm thời (mất kết nối DB, timeout) $\rightarrow$ **KHÔNG XACK**, để message nằm lại trong PEL và sẽ được đọc lại sau 2–5 giây ở vòng lặp quét PEL kế tiếp.

### 4.6. Khởi động và Vận hành Consumer Stream
1. Goroutine worker tiêu thụ stream chạy nền độc lập, lắng nghe `ctx.Done()` để dừng êm (graceful shutdown).
2. Tự khởi tạo group: `XGROUP CREATE stream:trip_events payment-group 0 MKSTREAM`. Bỏ qua lỗi `BUSYGROUP`.
3. **Vòng lặp tiêu thụ tin nhắn (Quy tắc Mục 5(m) Hợp đồng LLD-00):**
   - *Đọc tồn đọng PEL:* Mỗi chu kỳ (2–5 giây) gọi ĐÚNG 1 lượt `XREADGROUP GROUP payment-group payment-consumer-1 COUNT <PEL_BATCH_COUNT> STREAMS stream:trip_events 0` (KHÔNG lặp cho tới khi hết). Đếm số lần giao bằng `XPENDING` (delivery count) hoặc bộ đếm RAM `map[msg_id]int` (dọn khi XACK). Vượt trần `MAX_DELIVERY_ATTEMPTS` thì log `ERROR` rồi `XACK`. Nếu gặp lỗi vi phạm Unique Constraint (Postgres code 23505) thì coi là duplicate $\rightarrow$ `XACK` và bỏ qua.
   - *Đọc tin mới:* Gọi `XREADGROUP GROUP payment-group payment-consumer-1 BLOCK 2000 STREAMS stream:trip_events >`.
4. Sau khi giao dịch DB commit thành công, lệnh `XACK` nếu lỗi chỉ log `ERROR` và tiếp tục (message nằm trong PEL sẽ được bỏ qua ở lần chạy sau nhờ Idempotency).

---

## 5. XỬ LÝ NGOẠI LỆ (EXCEPTION HANDLING)

| Tình huống ngoại lệ | Ngữ cảnh phát sinh | Hành vi xử lý & Mã lỗi |
| :--- | :--- | :--- |
| Nạp tiền không hợp lệ | `amount <= 0` hoặc `amount > TOPUP_MAX_AMOUNT` | Trả `VALIDATION_ERROR` (400). |
| Phân trang không hợp lệ | `page < 1` hoặc `limit` ngoài [1, 100] | Trả `VALIDATION_ERROR` (400). |
| Kiểm tra số dư sai input | `required_amount < 0` hoặc sai user_id | Trả `VALIDATION_ERROR` (400). |
| Khách thiếu tiền lúc hoàn thành | Ví khách $< fare$ khi xử lý `TripCompleted` | Không trừ âm, ghi dòng `TRIP_PAYMENT` với `status = 'FAILED'`, log ERROR, vẫn gửi XACK. |
| Event bị giao lặp lại (Duplicate) | Consumer restart hoặc mạng chập chờn | `transactions` đã có `trip_id` (hoặc vi phạm unique 23505) $\rightarrow$ XACK và bỏ qua xử lý, chống trùng tiền. |
| Message Stream hỏng | Thiếu trường, UUID sai, fare $\le 0$, hoặc entry rỗng | Log ERROR, gửi XACK loại bỏ tin nhắn hỏng. |
| Lỗi DB tạm thời lúc tiêu thụ stream | Lỗi kết nối PostgreSQL khi xử lý thanh toán | KHÔNG XACK, giữ trong PEL để xử lý lại sau 2–5s (tối đa `MAX_DELIVERY_ATTEMPTS` lần). |
| Lỗi XACK sau commit DB | Redis timeout/mất mạng lúc gửi XACK | Log ERROR, không rollback DB. Message trong PEL sẽ được idempotency bỏ qua khi quét lại. |
| Admin gọi API ví | Người dùng có vai trò `ADMIN` gọi ví | Trả `FORBIDDEN` (403: Admin không sở hữu ví). |

### 5.1. Hạn chế đã biết
- **Khách thiếu tiền lúc trừ cước:** Tài xế không được cộng tiền chuyến đó; phát sinh nợ cần người vận hành can thiệp đối soát thủ công.
- **Khách đặt chuyến mới trước khi payment kịp trừ (FR-15):** Do luồng thanh toán diễn ra bất đồng bộ qua Redis Streams, khách hàng có thể đặt ngay cuốc xe mới khi số dư chưa bị trừ kịp.
- **Mất sự kiện thanh toán do crash:** Nếu `dispatch-service` crash giữa lúc `UPDATE` chuyến `COMPLETED` và `XADD` Stream (theo Mục 5.1 của `dispatch-service`), sự kiện `TripCompleted` không được phát đi dẫn đến thanh toán không chạy, cần đối soát DB để xử lý.

---

## 6. SỰ KIỆN & KHÓA REDIS

### 6.1. Khóa phân tán & Dữ liệu tạm
`payment-service` **không** sử dụng khóa phân tán Redis. Toàn bộ cơ chế kiểm soát tranh chấp (concurrency control) được thực thi trực tiếp trên PostgreSQL bằng `SELECT ... FOR UPDATE` với thứ tự khóa ID cố định để triệt tiêu Deadlock.

### 6.2. Kênh Pub/Sub
`payment-service` **không** phát hay nhận trực tiếp bất kỳ kênh Pub/Sub nào.

### 6.3. Sự kiện Redis Stream lắng nghe (`stream:trip_events`)
- **Stream:** `stream:trip_events`. Consumer Group: `payment-group`. Consumer Name: `payment-consumer-1`.
- **Sự kiện quan tâm:** Phân loại bằng trường `type`. Xử lý sự kiện `TripCompleted` (`type="TripCompleted"`, `version="1.0"`, `trip_id`, `customer_id`, `driver_id`, `fare`, `completed_at`). Bỏ qua các sự kiện khác (`TripCreated`, `TripCancelled`, `TripExpired`) và `XACK`.

---

## 7. CẤU HÌNH (ENV) VÀ TÀI NGUYÊN

### 7.1. Bảng biến môi trường
| Tên biến ENV | Mặc định | Ý nghĩa & Mục đích sử dụng |
| :--- | :--- | :--- |
| `PORT` | `8005` | Cổng HTTP nội bộ của `payment-service` trong mạng Docker. |
| `DATABASE_URL` | `postgres://payment_user:payment_pass@postgres:5432/paymentdb?sslmode=disable` | Chuỗi kết nối cơ sở dữ liệu `paymentdb`. |
| `REDIS_ADDR` | `redis:6379` | Địa chỉ kết nối Redis container trong Docker. |
| `REDIS_PASSWORD` | `redis_secret_pass` | Mật khẩu xác thực kết nối Redis (demo, ghi đè trong .env). |
| `COMMISSION_RATE` | `15` | Phần trăm hoa hồng hệ thống trích từ giá cước chuyến đi (nguyên). |
| `TOPUP_MAX_AMOUNT` | `100000000` | Số tiền nạp ví tối đa cho phép trong một lần giao dịch (VND, 100.000.000 VND). |
| `MAX_DELIVERY_ATTEMPTS` | `5` | Số lần thử lại tối đa trước khi loại bỏ message hỏng khỏi PEL (Mục 5(m) LLD-00). |
| `PEL_BATCH_COUNT` | `10` | Số lượng message tối đa đọc mỗi chu kỳ quét PEL (Mục 5(m) LLD-00). |

### 7.2. Tài nguyên & Thứ tự khởi động
- **Connection Pool PostgreSQL (Mục 5(h) LLD-00):** `MaxOpenConns = 10`, `MaxIdleConns = 3`, `ConnMaxLifetime = 30m`.
- **Ràng buộc bộ nhớ VPS 1GB:** `mem_limit` Docker đề xuất: 30 MB; `GOMEMLIMIT = 26MiB` (chống OOM).
- **Thứ tự khởi động dịch vụ (Startup Sequence):**
  1. Kết nối PostgreSQL `paymentdb` và thực thi migration bảng `wallets`, `transactions`.
  2. Kết nối Redis, ping kiểm tra `PONG`.
  3. Khởi tạo consumer group `payment-group` trên `stream:trip_events` (`MKSTREAM`, bỏ qua lỗi `BUSYGROUP`).
  4. Khởi chạy goroutine worker tiêu thụ stream (vòng lặp quét PEL định kỳ mỗi 2–5 giây kết hợp đọc tin mới `>`).
  5. Khởi động HTTP web server (Fiber) lắng nghe cổng `8005` tiếp nhận request.
