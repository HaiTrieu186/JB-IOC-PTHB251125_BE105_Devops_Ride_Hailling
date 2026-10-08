# THIẾT KẾ CHI TIẾT DỊCH VỤ TÀI KHOẢN (USER-SERVICE LLD)

> **Mã tài liệu:** `LLD-USER` | **Service:** `user-service` | **Database:** `userdb` (PostgreSQL)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [SRS v1.5](../01-srs/srs.md), [Hợp đồng liên service](../03-lld/00-hop-dong-lien-service.md), [UC-01 đến UC-09](../02-use-cases/uc-01-tai-khoan.md), [UC-29](../02-use-cases/uc-07-admin.md)

---

## 1. TRÁCH NHIỆM

- Quản lý định danh tài khoản: đăng ký khách hàng/tài xế, xác thực đăng nhập, cấp phát và xoay vòng JWT/Refresh Token.
- Quản lý trạng thái làm việc của tài xế (`OFFLINE`, `ONLINE`, `BUSY`) và chuyển trạng thái có điều kiện phục vụ điều phối.
- Quản lý hồ sơ cá nhân và phương tiện di chuyển của tài xế.
- Thu hồi phiên đăng nhập: ghi nhận Blacklist Access Token, hủy Refresh Token và phát tín hiệu ngắt kết nối WebSocket.
- Tự động khởi tạo tài khoản Quản trị viên (Admin) từ biến môi trường lúc khởi động.

---

## 2. BẢNG DB (USERDB)

### 2.1. Bảng `users` (Tài khoản người dùng)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | PK, DEFAULT `gen_random_uuid()` | Định danh người dùng duy nhất toàn hệ thống. |
| `phone_number` | `VARCHAR(15)` | NULLABLE, UNIQUE | Số điện thoại dùng để đăng nhập (Admin có thể NULL, Postgres hỗ trợ nhiều NULL trong UNIQUE). |
| `email` | `VARCHAR(255)` | NULLABLE, UNIQUE | Email dùng đăng nhập hoặc nhận thông báo. |
| `password_hash` | `VARCHAR(255)` | NOT NULL | Mật khẩu băm bằng thuật toán `bcrypt` (cost 10). |
| `full_name` | `VARCHAR(100)` | NOT NULL | Họ và tên hiển thị của người dùng. |
| `role` | `VARCHAR(20)` | NOT NULL | `CUSTOMER`, `DRIVER`, `ADMIN`. |
| `driver_status` | `VARCHAR(20)` | NULLABLE | Trạng thái tài xế: `OFFLINE`, `ONLINE`, `BUSY` (chỉ áp dụng khi `role = 'DRIVER'`). |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian tạo tài khoản (UTC). |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian cập nhật gần nhất (UTC). |

- **Chỉ mục & Ràng buộc toàn vẹn:**
  - PK: `id`. UNIQUE: `phone_number`, `email`.
  - CHECK role: `role IN ('CUSTOMER', 'DRIVER', 'ADMIN')`.
  - CHECK driver_status: `(role = 'DRIVER' AND driver_status IN ('ONLINE', 'OFFLINE', 'BUSY')) OR (role != 'DRIVER' AND driver_status IS NULL)`.
  - `idx_users_role_status`: `CREATE INDEX ... ON users (role, driver_status)` (Lọc tài xế ONLINE và phân trang Admin).

### 2.2. Bảng `vehicles` (Phương tiện tài xế)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `BIGINT` | PK, GENERATED ALWAYS AS IDENTITY | Định danh bản ghi phương tiện tăng tự động. |
| `driver_id` | `UUID` | NOT NULL, UNIQUE, FK `users(id)` ON DELETE CASCADE | Tài xế sở hữu xe (quan hệ 1-1). |
| `license_plate` | `VARCHAR(20)` | NOT NULL, UNIQUE | Biển số xe đăng ký. |
| `vehicle_type` | `VARCHAR(20)` | NOT NULL | Loại phương tiện: `MOTORBIKE`, `CAR_4_SEATS`. |
| `brand` | `VARCHAR(50)` | NULLABLE | Hãng sản xuất (Honda, Yamaha, Toyota...). |
| `model` | `VARCHAR(50)` | NULLABLE | Dòng xe (Wave, AirBlade, Vios...). |
| `color` | `VARCHAR(30)` | NULLABLE | Màu sơn xe phục vụ khách nhận diện. |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian cập nhật thông tin xe (UTC). |

- **Chỉ mục:** PK: `id`. UNIQUE: `driver_id`, `license_plate`.

---

## 3. API SPECIFICATION

Định dạng JSON chuẩn: `{"success": bool, "data": ..., "error": {"code": string, "message": string}}`. Header `X-User-Id` và `X-User-Role` do Gateway chuyển tiếp vào cho các endpoint yêu cầu xác thực.

### 3.1. API Công khai (Public Endpoints)

| Method & Path | Tầng | Actor | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `POST /api/v1/auth/register` | 1 | Khách vãng lai | `phone_number` (string, B)<br>`email` (string, T)<br>`password` (string, B, $\ge 6$ ký tự)<br>`full_name` (string, B)<br>`role` ("CUSTOMER" \| "DRIVER", B) | `user_id` (uuid)<br>`role` (string)<br>`full_name` (string)<br>`created_at` (iso8601) | `VALIDATION_ERROR` (400: sai định dạng/pass ngắn)<br>`USER_ALREADY_EXISTS` (409: trùng phone/email) |
| `POST /api/v1/auth/login` | 1 | Người dùng | `phone_or_email` (string, B)<br>`password` (string, B) | `access_token` (jwt)<br>`refresh_token` (opaque string)<br>`expires_in` (int: 3600)<br>`user` (`{id, role, full_name, driver_status}`) | `VALIDATION_ERROR` (400: thiếu trường)<br>`UNAUTHORIZED` (401: sai tài khoản hoặc mật khẩu) |
| `POST /api/v1/auth/refresh` | 1 | Người dùng | `refresh_token` (string, B) | `access_token` (jwt)<br>`refresh_token` (opaque string mới)<br>`expires_in` (int: 3600) | `VALIDATION_ERROR` (400: thiếu token)<br>`INVALID_REFRESH_TOKEN` (401: token không tồn tại/đã dùng) |
| `POST /api/v1/auth/logout` | 3 | Người dùng | `refresh_token` (string, B) | `message` ("Đăng xuất thành công") | `VALIDATION_ERROR` (400: thiếu refresh_token)<br>`DRIVER_CANNOT_LOGOUT` (400: tài xế đang BUSY)<br>`INVALID_REFRESH_TOKEN` (401: token sai/không chính chủ) |
| `PATCH /api/v1/driver/status` *(do LLD đặt)* | 1 | `DRIVER` | `status` ("ONLINE" \| "OFFLINE", B) | `driver_id` (uuid)<br>`status` (string) | `FORBIDDEN` (403: role != DRIVER)<br>`VALIDATION_ERROR` (400: sai giá trị)<br>`DRIVER_BUSY` (400: tài xế đang BUSY cố chuyển OFFLINE) |
| `GET /api/v1/users/me` *(do LLD đặt)* | 3 | Người dùng | *(None)* | `id` (uuid), `phone_number` (string), `email` (string), `full_name` (string), `role` (string), `driver_status` (string\|null) | `UNAUTHORIZED` (401: thiếu thông tin xác thực) |
| `PUT /api/v1/driver/vehicle` *(do LLD đặt)* | 3 | `DRIVER` | `license_plate` (string, B)<br>`vehicle_type` (string, B)<br>`brand` (string, T)<br>`model` (string, T)<br>`color` (string, T) | `driver_id` (uuid)<br>`license_plate` (string)<br>`vehicle_type` (string)<br>`updated_at` (iso8601) | `FORBIDDEN` (403: role != DRIVER)<br>`VALIDATION_ERROR` (400: thiếu trường bắt buộc) |
| `GET /api/v1/admin/users` | 3 | `ADMIN` | Query: `role` (T), `driver_status` (T: "ONLINE" \| "OFFLINE" \| "BUSY"), `page` (T, mặc định 1, *(do LLD đặt)*), `limit` (T, mặc định 20, tối đa 100, *(do LLD đặt)*) | `users` (array: `[{id, phone_number, email, full_name, role, driver_status, created_at}]`), `total` (int), `page` (int) | `FORBIDDEN` (403: role != ADMIN) |
| `GET /health` *(do LLD đặt)* | 1 | Tất cả | *(None)* | `status` ("ok") | *(None)* |

*(B: Bắt buộc, T: Tùy chọn)*

### 3.2. API Nội bộ (Internal Endpoints)

| Method & Path | Tầng | Caller | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `POST /internal/v1/users/filter-online` | 1 | `dispatch`<br>`pricing` | `driver_ids` (array of string, B) | `online_driver_ids` (array of string) | `VALIDATION_ERROR` (400: sai kiểu dữ liệu hoặc mảng > 500 phần tử; mảng rỗng [] trả 200 với online_driver_ids = []) |
| `POST /internal/v1/drivers/:id/status` | 1 | `dispatch` | Path: `id` (uuid, B)<br>Body: `from_status` (string, B: "ONLINE" \| "BUSY")<br>`to_status` (string, B: "ONLINE" \| "BUSY") | `driver_id` (uuid)<br>`status` (string) | `VALIDATION_ERROR` (400: thiếu/sai trường hoặc cặp trạng thái khác (ONLINE->BUSY) và (BUSY->ONLINE))<br>`DRIVER_NOT_AVAILABLE` (400: trạng thái hiện tại khác from_status hoặc không tìm thấy) |

---

## 4. LUỒNG XỬ LÝ TỪNG BƯỚC

### 4.1. Đăng ký & Đăng nhập (UC-01, UC-02)
- **Đăng ký:** Validate dữ liệu $\rightarrow$ Kiểm tra trùng lặp `phone_number` / `email` trong `users` (nếu có $\rightarrow$ `USER_ALREADY_EXISTS` 409) $\rightarrow$ Băm mật khẩu bằng `bcrypt.GenerateFromPassword(pwd, 10)` $\rightarrow$ INSERT `users` (`role = CUSTOMER` thì `driver_status = NULL`; `role = DRIVER` thì `driver_status = 'OFFLINE'`). Trả về 201 Created.
- **Đăng nhập:** Tra cứu người dùng theo `phone_number` hoặc `email` $\rightarrow$ So khớp `bcrypt.CompareHashAndPassword` (thất bại $\rightarrow$ `UNAUTHORIZED` 401) $\rightarrow$ Sinh JWT Access Token (claims: `user_id`, `role`, `exp = now + 3600`, `jti = uuid()`) $\rightarrow$ Sinh Refresh Token ngẫu nhiên (opaque UUID) $\rightarrow$ Lưu Redis: `SET refresh:<token> <user_id> EX 604800` (7 ngày). Trả về 200 OK kèm cặp token.

### 4.2. Xoay vòng Refresh Token nguyên tử (UC-08)
1. Tiếp nhận `refresh_token` từ request body.
2. Thu hồi nguyên tử bằng lệnh Redis: `GETDEL refresh:<token>`.
3. Nếu kết quả trả về `nil` (token không tồn tại, hết hạn, hoặc đã bị request khác dùng): Trả ngay `INVALID_REFRESH_TOKEN` (401).
4. Nếu tìm thấy `user_id`:
   - Truy vấn thông tin người dùng từ `userdb` để lấy `role` hiện tại.
   - Sinh Access Token mới (kèm `jti` mới, thời hạn 1 giờ).
   - Sinh Refresh Token mới và lưu Redis: `SET refresh:<new_token> <user_id> EX 604800`.
   - Trả về 200 OK với cặp token mới.

### 4.3. Đăng xuất & Thu hồi phiên (UC-09)
1. **Xác thực Refresh Token trước:** Nhận `refresh_token` từ request body. Thực hiện `GET refresh:<token>`. Nếu giá trị trả về khác `user_id` (từ header `X-User-Id`) hoặc `nil` $\rightarrow$ Trả `INVALID_REFRESH_TOKEN` (401), chưa thay đổi bất kỳ trạng thái nào.
2. **Xử lý trạng thái tài xế:** Nếu `role = 'DRIVER'`, thực thi:
   `UPDATE users SET driver_status = 'OFFLINE', updated_at = now() WHERE id = <user_id> AND driver_status != 'BUSY'`.
   Nếu 0 dòng affected (tài xế đang `BUSY`) $\rightarrow$ Trả `DRIVER_CANNOT_LOGOUT` (400).
3. **Thu hồi phiên & Dọn dẹp:**
   - Xóa Refresh Token: `DEL refresh:<token>`.
   - Đưa Access Token vào blacklist: `user-service` tự giải mã và kiểm chữ ký JWT bằng `JWT_SECRET` từ header `Authorization` (api-gateway phải giữ nguyên header `Authorization` khi chuyển tiếp request logout) để lấy `jti` và `exp`. Tính $TTL_{\text{còn lại}} = \max(exp - now, 1)$. Thực thi: `SET blacklist:<jti> "revoked" EX <TTL_còn_lại>`.
   - Ngắt WebSocket thời gian thực: `PUBLISH ride:ws_control {"action": "DISCONNECT", "user_id": "<user_id>"}`.
   - Trả về 200 OK.

*Ghi chú:* `user-service` tự giải mã và kiểm chữ ký JWT bằng `JWT_SECRET` để lấy `jti` và `exp` (gateway chỉ chuyển `X-User-Id` và `X-User-Role`); `api-gateway` phải giữ nguyên header `Authorization` khi chuyển request logout.

### 4.4. Cập nhật trạng thái Tài xế (UC-04)
- Nhận yêu cầu đổi sang `ONLINE` hoặc `OFFLINE`.
- Nếu chuyển sang `OFFLINE`: Thực thi UPDATE có điều kiện `UPDATE users SET driver_status = 'OFFLINE', updated_at = now() WHERE id = <user_id> AND role = 'DRIVER' AND driver_status != 'BUSY'`. Nếu 0 dòng affected (tài xế đang `BUSY`) $\rightarrow$ Trả `DRIVER_BUSY` (400).
- Nếu chuyển sang `ONLINE`: `UPDATE users SET driver_status = 'ONLINE', updated_at = now() WHERE id = <user_id> AND role = 'DRIVER' AND driver_status != 'BUSY'`. Nếu 0 dòng $\rightarrow$ Trả `DRIVER_BUSY` (400). Trả về 200 OK.

### 4.5. Xử lý API Nội bộ cho Dispatch / Pricing
- **`POST /internal/v1/users/filter-online`:**
  - Nhận mảng `driver_ids`. Mảng rỗng `[]` trả về ngay 200 OK với `{"online_driver_ids": []}`. Nếu sai kiểu dữ liệu hoặc mảng $> 500$ phần tử $\rightarrow$ Trả `VALIDATION_ERROR` (400).
  - Thực thi: `SELECT id FROM users WHERE id IN (<driver_ids>) AND role = 'DRIVER' AND driver_status = 'ONLINE'`.
  - Trả về danh sách `online_driver_ids: []`.
- **`POST /internal/v1/drivers/:id/status`:**
  - Nhận `id`, `from_status`, `to_status`. Chỉ chấp nhận 2 cặp chuyển đổi: `(ONLINE → BUSY)` và `(BUSY → ONLINE)`. Nếu nhận cặp trạng thái khác $\rightarrow$ Trả `VALIDATION_ERROR` (400).
  - Thực thi UPDATE có điều kiện nguyên tử:  
    `UPDATE users SET driver_status = <to_status>, updated_at = now() WHERE id = <id> AND role = 'DRIVER' AND driver_status = <from_status>`.
  - Nếu số dòng affected $= 0$ (tài xế không tồn tại hoặc trạng thái hiện tại khác `from_status`): Trả ngay lỗi `DRIVER_NOT_AVAILABLE` (400).
  - Nếu thành công: Trả về `{"driver_id": "<id>", "status": "<to_status>"}`.

---

## 5. XỬ LÝ NGOẠI LỆ (EXCEPTION HANDLING)

| Ngoại lệ | Điều kiện kích hoạt | Xử lý & Mã lỗi |
| :--- | :--- | :--- |
| Trùng lặp tài khoản | Đăng ký số điện thoại hoặc email đã có trong `users` | Rollback DB, trả `USER_ALREADY_EXISTS` (409). |
| Mật khẩu không khớp | Đăng nhập sai mật khẩu hoặc tài khoản không tồn tại | Chống timing attack, trả `UNAUTHORIZED` (401). |
| Race condition Refresh | 2 request cùng gửi 1 refresh token song song | `GETDEL` chỉ cho 1 request lấy được user_id; request kia nhận `nil` $\rightarrow$ Trả `INVALID_REFRESH_TOKEN` (401). |
| Tài xế BUSY đăng xuất | Tài xế đang chở khách gửi API logout | Chặn đăng xuất, giữ nguyên phiên $\rightarrow$ Trả `DRIVER_CANNOT_LOGOUT` (400). |
| Tài xế BUSY tắt app | Tài xế đang chở khách cố chuyển trạng thái OFFLINE | Không cho phép đổi trạng thái $\rightarrow$ Trả `DRIVER_BUSY` (400). |
| Tranh chấp nhận chuyến | Dispatch gọi đổi ONLINE $\rightarrow$ BUSY nhưng tài xế đã bận | 0 dòng affected $\rightarrow$ Trả `DRIVER_NOT_AVAILABLE` (400) để Dispatch nhả lock. |

---

## 6. SỰ KIỆN & KHÓA REDIS

### 6.1. Tài nguyên Redis quản lý
- `refresh:<token>`: String, TTL 7 ngày (`604800s`). Giá trị lưu: chuỗi `user_id`. Quản lý vòng đời Refresh Token, xoay vòng bằng `GETDEL`.
- `blacklist:<jti>`: String, TTL $exp - now$. Giá trị lưu: `"revoked"`. Thu hồi quyền truy cập của Access Token khi người dùng chủ động đăng xuất.

### 6.2. Kênh Pub/Sub phát đi
- `ride:ws_control`: Kênh điều khiển kết nối WebSocket của Gateway.
  - Payload: `{"action": "DISCONNECT", "user_id": "<uuid>"}`.
  - Mục đích: Khi người dùng đăng xuất, `user-service` phát thông điệp để `ws-gateway` ngắt kết nối WebSocket của phiên làm việc tương ứng.

---

## 7. CẤU HÌNH (ENV) VÀ TÀI NGUYÊN

### 7.1. Bảng biến môi trường
| Tên biến ENV | Mặc định | Ý nghĩa & Mục đích sử dụng |
| :--- | :--- | :--- |
| `PORT` | `8001` | Cổng HTTP nội bộ của `user-service` trong mạng Docker. |
| `DATABASE_URL` | `postgres://user_user:user_pass@postgres:5432/userdb?sslmode=disable` | Chuỗi kết nối cơ sở dữ liệu `userdb`. |
| `REDIS_ADDR` | `redis:6379` | Địa chỉ kết nối Redis container trong Docker. |
| `REDIS_PASSWORD` | `redis_secret_pass` | Mật khẩu xác thực Redis (giá trị demo, ghi đè trong .env). |
| `JWT_SECRET` | `secret-key-ride-hailing-devops` | Khóa bí mật dùng chung ký HMAC-SHA256 cho JWT Access Token. |
| `ACCESS_TOKEN_EXPIRY` | `3600` | Thời hạn Access Token tính bằng giây (1 giờ). |
| `REFRESH_TOKEN_EXPIRY`| `604800` | Thời hạn Refresh Token tính bằng giây (7 ngày). |
| `ADMIN_EMAIL` | `admin@ridehailing.local` | Email tài khoản quản trị viên khởi tạo tự động. |
| `ADMIN_PASSWORD` | `Admin@123456` | Mật khẩu tài khoản quản trị viên khởi tạo ban đầu. |

### 7.2. Tài nguyên & Thứ tự khởi động
- **Connection Pool PostgreSQL (Mục 5(h) LLD-00):** `MaxOpenConns = 5`, `MaxIdleConns = 2`, `ConnMaxLifetime = 30m`.
- **Ràng buộc bộ nhớ VPS 1GB:** `mem_limit` Docker đề xuất: 30 MB; `GOMEMLIMIT = 26MiB` (chống OOM).
- **Thứ tự khởi động dịch vụ (Startup Sequence):**
  1. Kết nối PostgreSQL `userdb` và thực thi migration bảng `users`, `vehicles`.
  2. Kết nối Redis và kiểm tra ping.
  3. Khởi tạo tài khoản Admin từ ENV (UC-07, Idempotent): Kiểm tra nếu `email = ADMIN_EMAIL` chưa tồn tại trong bảng `users`, chèn bản ghi mới với `email = ADMIN_EMAIL`, `password_hash = bcrypt(ADMIN_PASSWORD)`, `phone_number = NULL`, `full_name = "Administrator"`, `role = 'ADMIN'`, `driver_status = NULL`. Admin đăng nhập bằng email. Bỏ qua nếu email đã tồn tại.
  4. Khởi động HTTP web server (Fiber) bắt đầu tiếp nhận request.
