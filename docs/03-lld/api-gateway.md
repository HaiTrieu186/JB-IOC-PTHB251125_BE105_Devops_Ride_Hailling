# THIẾT KẾ CHI TIẾT CỔNG API (API-GATEWAY LLD)

> **Mã tài liệu:** `LLD-GW` | **Service:** `api-gateway` | **Database:** Không có DB (Redis đọc blacklist)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [SRS v1.5](../01-srs/srs.md), [Hợp đồng liên service](../03-lld/00-hop-dong-lien-service.md), [Tất cả 7 LLD](../03-lld/)

---

## 1. TRÁCH NHIỆM

- Điểm tiếp nhận duy nhất cho toàn bộ REST API công khai từ Nginx (127.0.0.1:8080) vào mạng microservices.
- Xóa sạch mọi header `X-User-*` client gửi lên nhằm chống giả mạo danh tính (Header Spoofing).
- Xác thực tập trung JWT (HMAC-SHA256, `exp`) và kiểm tra danh sách đen (`blacklist:<jti>`) trên Redis.
- Gắn header chuẩn `X-User-Id` và `X-User-Role` trước khi forward tới service nội bộ; giữ nguyên `Authorization` khi gọi `user-service`.
- Phân quyền tập trung: Chặn mọi request vào `/api/v1/admin/*` nếu không có role `ADMIN` (trả 403 `FORBIDDEN`).
- Chặn tuyệt đối mọi request Internet truy cập đường dẫn nội bộ `/internal/*` (trả 404). (Không route WebSocket `/ws/`, do Nginx trỏ thẳng ws-gateway).

---

## 2. BẢNG DB

`api-gateway` **không có DB** quan hệ hay lưu trữ bền vững. Service chỉ kết nối Redis để đọc `blacklist:<jti>` phục vụ thu hồi token.

---

## 3. BẢNG ĐỊNH TUYẾN API (ROUTING TABLE)

Gồm đủ **26 endpoint công khai** của hệ thống và endpoint `/health` của chính gateway. Phân quyền chi tiết `CUSTOMER`/`DRIVER` do service đích tự kiểm tra.

*(Lưu ý thứ tự đăng ký route: Endpoint `GET /api/v1/trips/current` bắt buộc phải được đăng ký TRƯỚC `GET /api/v1/trips/:id` để tránh router nhận nhầm `current` thành tham số động `:id`).*

| Method & Path | Tầng | Service đích (URL ENV) | JWT | Phân quyền Gateway |
| :--- | :-: | :--- | :--- | :--- |
| `POST /api/v1/auth/register` | 1 | `USER_SERVICE_URL` | Không | Công khai |
| `POST /api/v1/auth/login` | 1 | `USER_SERVICE_URL` | Không | Công khai |
| `POST /api/v1/auth/refresh` | 1 | `USER_SERVICE_URL` | Không | Công khai |
| `POST /api/v1/auth/logout` | 3 | `USER_SERVICE_URL` | Có | Mọi role (giữ nguyên header Authorization) |
| `PATCH /api/v1/driver/status` *(do LLD đặt)* | 1 | `USER_SERVICE_URL` | Có | Mọi role (service đích kiểm tra DRIVER) |
| `GET /api/v1/users/me` *(do LLD đặt)* | 3 | `USER_SERVICE_URL` | Có | Mọi role |
| `PUT /api/v1/driver/vehicle` *(do LLD đặt)* | 3 | `USER_SERVICE_URL` | Có | Mọi role (service đích kiểm tra DRIVER) |
| `GET /api/v1/admin/users` | 3 | `USER_SERVICE_URL` | Có | `ADMIN` (Query: `role`, `driver_status`, `page`, `limit`) |
| `POST /api/v1/trips` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role (service đích kiểm tra CUSTOMER) |
| `POST /api/v1/trips/:id/accept` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role (service đích kiểm tra DRIVER) |
| `POST /api/v1/trips/:id/picking-up` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role (service đích kiểm tra DRIVER) |
| `POST /api/v1/trips/:id/start-trip` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role (service đích kiểm tra DRIVER) |
| `POST /api/v1/trips/:id/complete` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role (service đích kiểm tra DRIVER) |
| `POST /api/v1/trips/:id/cancel` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role |
| `GET /api/v1/trips/current` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role |
| `GET /api/v1/trips/:id` | 1 | `DISPATCH_SERVICE_URL` | Có | Mọi role |
| `POST /api/v1/admin/trips/:id/force-cancel` | 3 | `DISPATCH_SERVICE_URL` | Có | `ADMIN` |
| `GET /api/v1/admin/trips` | 3 | `DISPATCH_SERVICE_URL` | Có | `ADMIN` (Query: `status`, `page`, `limit`) |
| `POST /api/v1/pricing/estimate` | 1 | `PRICING_SERVICE_URL` | Có | Mọi role |
| `GET /api/v1/admin/pricing/config` | 3 | `PRICING_SERVICE_URL` | Có | `ADMIN` |
| `PUT /api/v1/admin/pricing/config` | 3 | `PRICING_SERVICE_URL` | Có | `ADMIN` |
| `GET /api/v1/wallets/me` | 1 | `PAYMENT_SERVICE_URL` | Có | Mọi role (service đích từ chối ADMIN) |
| `POST /api/v1/wallets/top-up` | 1 | `PAYMENT_SERVICE_URL` | Có | Mọi role (service đích từ chối ADMIN) |
| `GET /api/v1/wallets/transactions` | 1 | `PAYMENT_SERVICE_URL` | Có | Mọi role (service đích từ chối ADMIN) |
| `GET /api/v1/admin/drivers/active` | 3 | `LOCATION_SERVICE_URL` | Có | `ADMIN` |
| `GET /api/v1/admin/reports` | 2 | `AI_SERVICE_URL` | Có | `ADMIN` (Query: `start_time`, `end_time`) |
| `GET /health` *(do LLD đặt)* | 1 | *(Chính Gateway)* | Không | Trả `{"status": "ok"}` |

---

## 4. LUỒNG XỬ LÝ TỪNG BƯỚC

### 4.1. Chuỗi Middleware xử lý Request (Theo thứ tự đánh số)
1. **Middleware 1 (Sanitize Headers):** Xóa bỏ toàn bộ header có tiền tố `X-User-` (`X-User-Id`, `X-User-Role`) do client gửi lên nhằm chống mạo danh.
2. **Middleware 2 (Block Internal & Unknown Routes):**
   - Nếu path bắt đầu bằng `/internal/` $\rightarrow$ Trả ngay 404 `NOT_FOUND` (chặn tuyệt đối không route ra ngoài Internet).
   - Nếu request method và path không khớp bất kỳ route nào trong Bảng định tuyến Mục 3 $\rightarrow$ Trả ngay 404 `NOT_FOUND`.
3. **Middleware 3 (Public Route Bypass):** Nếu request thuộc danh sách không cần JWT (đăng ký, đăng nhập, làm mới, `/health`) $\rightarrow$ Bỏ qua xác thực, chuyển thẳng tới Middleware 6.
4. **Middleware 4 (JWT Authentication):**
   - Trích xuất token từ header `Authorization: Bearer <token>`. Thiếu token $\rightarrow$ Trả 401 `UNAUTHORIZED`.
   - Xác thực chữ ký JWT với ràng buộc thuật toán: **chỉ chấp nhận thuật toán `HS256`**, từ chối dứt khoát thuật toán `none` và các thuật toán khác; giải mã với `JWT_SECRET`. Kiểm tra hạn dùng `exp`. Sai token, hết hạn hoặc sai thuật toán $\rightarrow$ Trả 401 `UNAUTHORIZED`.
   - Kiểm tra Redis: `GET blacklist:<jti>`. Nếu Redis lỗi $\rightarrow$ Trả 503 `SERVICE_UNAVAILABLE` (fail-closed). Nếu key tồn tại $\rightarrow$ Trả 401 `UNAUTHORIZED`.
   - Gắn header chuyển tiếp: `X-User-Id = claims.user_id`, `X-User-Role = claims.role`.
   - Nếu route đích là `user-service` $\rightarrow$ giữ nguyên header `Authorization`.
5. **Middleware 5 (Admin Authorization Guard):** Nếu path bắt đầu bằng `/api/v1/admin/`, kiểm tra `claims.role == 'ADMIN'`. Nếu sai $\rightarrow$ Trả ngay 403 `FORBIDDEN`.
6. **Middleware 6 (Reverse Proxy Dispatcher):** Forward HTTP request sang service đích tương ứng với timeout = `PROXY_TIMEOUT_MS`.
   - **Nguyên tắc Proxy Passthrough:** Khi nhận phản hồi từ service đích, Gateway trả nguyên trạng HTTP status code và response body của service đích về cho client (không bọc lại hay can thiệp dữ liệu).
   - Gateway chỉ tự sinh response lỗi JSON khi chính Gateway từ chối request (401, 403, 404, 503).
   - Nếu service đích mất kết nối hoặc timeout quá `PROXY_TIMEOUT_MS` $\rightarrow$ Trả 503 `SERVICE_UNAVAILABLE`.

---

## 5. XỬ LÝ NGOẠI LỆ (EXCEPTION HANDLING)

| Tình huống ngoại lệ | Ngữ cảnh phát sinh | Hành vi xử lý & Mã lỗi |
| :--- | :--- | :--- |
| Route không tồn tại | Request không khớp bất kỳ đường dẫn nào trong bảng định tuyến | Trả `NOT_FOUND` (404). |
| Truy cập API nội bộ từ Internet | Client gửi request bắt đầu bằng `/internal/*` | Trả `NOT_FOUND` (404). |
| Token thiếu / sai / hết hạn / bị thu hồi / sai thuật toán | Client gửi request cần JWT nhưng token không hợp lệ | Trả `UNAUTHORIZED` (401). |
| Truy cập trái phép endpoint Admin | Client không có role ADMIN gọi `/api/v1/admin/*` | Trả `FORBIDDEN` (403). |
| Gateway lỗi Redis | Redis sập hoặc timeout khi tra cứu `blacklist:<jti>` | Trả `SERVICE_UNAVAILABLE` (503: fail-closed). |
| Service đích sập hoặc timeout | Service nội bộ không phản hồi quá `PROXY_TIMEOUT_MS` | Trả `SERVICE_UNAVAILABLE` (503). |

### 5.1. Hạn chế đã biết
- **Khởi động lại Gateway:** Khi reload/restart container `api-gateway`, các request HTTP đang dở dang sẽ bị ngắt tạm thời trong 1-2 giây. Nginx giữ kết nối và client có thể retry.

---

## 6. SỰ KIỆN & KHÓA REDIS

### 6.1. Dữ liệu Redis đọc
- `blacklist:<jti>`: String, TTL $exp - now$. Tra cứu lúc xác thực JWT. Nếu tồn tại $\rightarrow$ từ chối (401).

### 6.2. Kênh Pub/Sub và Stream
`api-gateway` **không** phát hay nhận trực tiếp bất kỳ Pub/Sub hay Redis Stream nào (chỉ đóng vai trò HTTP Reverse Proxy).

---

## 7. CẤU HÌNH (ENV) VÀ TÀI NGUYÊN

### 7.1. Bảng biến môi trường
| Tên biến ENV | Mặc định | Ý nghĩa & Mục đích sử dụng |
| :--- | :--- | :--- |
| `PORT` | `8080` | Cổng HTTP của `api-gateway` (Nginx map 127.0.0.1:8080). |
| `JWT_SECRET` | `secret-key-ride-hailing-devops` | Khóa bí mật chung xác thực chữ ký HMAC-SHA256 của JWT. |
| `REDIS_ADDR` | `redis:6379` | Địa chỉ kết nối Redis container trong Docker. |
| `REDIS_PASSWORD` | `redis_secret_pass` | Mật khẩu xác thực kết nối Redis (demo, ghi đè trong .env). |
| `USER_SERVICE_URL` | `http://user-service:8001` | Base URL gọi nội bộ `user-service`. |
| `LOCATION_SERVICE_URL` | `http://location-service:8002` | Base URL gọi nội bộ `location-service`. |
| `DISPATCH_SERVICE_URL` | `http://dispatch-service:8003` | Base URL gọi nội bộ `dispatch-service`. |
| `PRICING_SERVICE_URL` | `http://pricing-service:8004` | Base URL gọi nội bộ `pricing-service`. |
| `PAYMENT_SERVICE_URL` | `http://payment-service:8005` | Base URL gọi nội bộ `payment-service`. |
| `AI_SERVICE_URL` | `http://ai-service:8006` | Base URL gọi nội bộ `ai-service`. |
| `PROXY_TIMEOUT_MS` | `15000` | Timeout tối đa khi proxy request sang service nội bộ (ms, đặt 15000 vì (a) POST /trips có thể gọi tuần tự pricing, payment, location, user; (b) GET /api/v1/admin/reports có thể chờ Gemini tối đa LLM_TIMEOUT_SECONDS (10 giây) rồi fallback RULE_BASED, nên timeout của gateway phải lớn hơn timeout LLM cộng thời gian truy vấn DB). |

### 7.2. Tài nguyên & Thứ tự khởi động
- **Ràng buộc bộ nhớ VPS 1GB:** `mem_limit` Docker đề xuất: 25 MB; `GOMEMLIMIT = 22MiB` (chống OOM).
- **Thứ tự khởi động dịch vụ (Startup Sequence):**
  1. Kết nối Redis, ping kiểm tra `PONG`.
  2. Khởi động HTTP web server (Fiber) lắng nghe trên cổng `8080` tiếp nhận request.
