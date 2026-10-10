# THIẾT KẾ CHI TIẾT DỊCH VỤ TÍNH GIÁ (PRICING-SERVICE LLD)

> **Mã tài liệu:** `LLD-PRICING` | **Service:** `pricing-service` | **Database:** `pricingdb` (PostgreSQL)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [SRS v1.5](../01-srs/srs.md), [Hợp đồng liên service](../03-lld/00-hop-dong-lien-service.md), [UC-03 (UC-14, UC-15, UC-16)](../02-use-cases/uc-03-dat-xe-va-ghep-chuyen.md), [UC-07](../02-use-cases/uc-07-admin.md)

---

## 1. TRÁCH NHIỆM

- Tính toán khoảng cách lộ trình và thời gian di chuyển (ETA) giữa điểm đón và điểm trả qua OSRM Public API (fallback Haversine $\times 1.35$ khi timeout $\ge 400\text{ ms}$ hoặc lỗi mạng).
- Tính toán hệ số nhu cầu thời gian thực (`SurgeMultiplier`) theo tỷ lệ Cung/Cầu ($\text{Demand}/\text{Supply}$) trong phạm vi 1 ô Geohash 5.
- Tính toán giá cước chuyến đi và làm tròn tới 1.000 VND gần nhất theo bảng giá hiện hành.
- Quản lý cấu hình bảng giá (`BaseFare`, `PricePerKm`, ngưỡng $T$) lưu trong `pricingdb`, cache trong Redis, khởi tạo từ ENV khi chưa có bản ghi; cung cấp API cho Admin quản lý.
- Lắng nghe sự kiện `TripCreated` qua consumer group `pricing-group` từ `stream:trip_events` để tích lũy dữ liệu Nhu cầu (Demand) theo cửa sổ trượt 5 phút trên Redis ZSET.

---

## 2. BẢNG DB (PRICINGDB)

### 2.1. Bảng `pricing_configs` (Cấu hình bảng giá)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `INTEGER` | PK, DEFAULT `1` | Bản ghi cấu hình duy nhất toàn hệ thống (`CHECK id = 1`). |
| `base_fare` | `BIGINT` | NOT NULL | Cước mở cửa cơ sở (VND, số nguyên, $> 0$). |
| `price_per_km` | `BIGINT` | NOT NULL | Đơn giá mỗi km di chuyển (VND, số nguyên, $> 0$). |
| `surge_threshold` | `NUMERIC(4,2)` | NOT NULL | Ngưỡng T, thuộc [0.01, 99.99], mặc định 1.0. |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Mốc thời gian cập nhật cấu hình gần nhất (UTC). |

- **Chỉ mục & Ràng buộc toàn vẹn:**
  - PK: `id`. Singleton constraint: `CHECK (id = 1)`.
  - CHECK hợp lệ: `CHECK (base_fare > 0 AND price_per_km > 0 AND surge_threshold BETWEEN 0.01 AND 99.99)`.

---

## 3. API SPECIFICATION

Định dạng JSON chuẩn: `{"success": bool, "data": ..., "error": {"code": string, "message": string}}`. Header `X-User-Id` và `X-User-Role` do Gateway chuyển tiếp vào cho các endpoint công khai.

### 3.1. API Công khai (Public Endpoints)

| Method & Path | Tầng | Actor | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `POST /api/v1/pricing/estimate` | 1 | Đã đăng nhập (`CUSTOMER`, `DRIVER`, `ADMIN`) | `pickup_lat` (float, B)<br>`pickup_lng` (float, B)<br>`dropoff_lat` (float, B)<br>`dropoff_lng` (float, B) | `distance` (float, km)<br>`eta` (int, phút)<br>`source` ("OSRM" \| "HAVERSINE")<br>`surge_multiplier` (float)<br>`base_fare` (int)<br>`price_per_km` (int)<br>`total_fare` (int, VND) | `UNAUTHORIZED` (401: thiếu/sai token)<br>`VALIDATION_ERROR` (400: tọa độ sai hoặc đón trùng trả) |
| `GET /api/v1/admin/pricing/config` | 1 | `ADMIN` | *(None)* | `base_fare` (int)<br>`price_per_km` (int)<br>`surge_threshold` (float)<br>`updated_at` (iso8601) | `FORBIDDEN` (403: role != ADMIN) |
| `PUT /api/v1/admin/pricing/config` | 1 | `ADMIN` | `base_fare` (int, B, $> 0$)<br>`price_per_km` (int, B, $> 0$)<br>`surge_threshold` (float, B, $\in [0.01, 99.99]$, tối đa 2 chữ số thập phân) | `base_fare` (int)<br>`price_per_km` (int)<br>`surge_threshold` (float)<br>`updated_at` (iso8601) | `FORBIDDEN` (403: role != ADMIN)<br>`VALIDATION_ERROR` (400: thiếu trường, giá trị $\le 0$ hoặc surge_threshold ngoài [0.01, 99.99]) |
| `GET /health` *(do LLD đặt)* | 1 | Tất cả | *(None)* | `status` ("ok") | *(None)* |

*(B: Bắt buộc, T: Tùy chọn)*

### 3.2. API Nội bộ (Internal Endpoints)

| Method & Path | Tầng | Caller | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `POST /internal/v1/pricing/estimate` | 1 | `dispatch-service` | `pickup_lat` (float, B)<br>`pickup_lng` (float, B)<br>`dropoff_lat` (float, B)<br>`dropoff_lng` (float, B) | `fare` (int, VND)<br>`distance_m` (int, mét)<br>`eta_seconds` (int, giây)<br>`surge_multiplier` (float) | `VALIDATION_ERROR` (400: tọa độ sai hoặc đón trùng trả) |

---

## 4. LUỒNG XỬ LÝ TỪNG BƯỚC

### 4.1. Lấy cấu hình Bảng giá (DB & Cache Redis)
1. Kiểm tra cache Redis: `GET pricing:config`. Nếu key tồn tại $\rightarrow$ parse JSON lấy `{base_fare, price_per_km, surge_threshold}`.
2. Nếu cache miss hoặc Redis cache gặp lỗi:
   - Truy vấn trực tiếp DB: `SELECT base_fare, price_per_km, surge_threshold, updated_at FROM pricing_configs WHERE id = 1`.
   - Các biến môi trường ENV (`DEFAULT_BASE_FARE`, `DEFAULT_PRICE_PER_KM`, `SURGE_THRESHOLD`) CHỈ dùng làm giá trị seed khởi tạo lần đầu duy nhất khi bảng `pricing_configs` rỗng; sau khi đã có bản ghi trong DB thì toàn bộ hệ thống đọc từ DB (cache Redis 1 giờ). Muốn thay đổi bảng giá lúc runtime, Admin dùng `PUT /api/v1/admin/pricing/config` (khi đó cache `pricing:config` bị `DEL`).
   - Nếu đọc cache miss nhưng Redis hoạt động bình thường, lưu cache Redis: `SET pricing:config <json> EX 3600` (TTL 1 giờ). Nếu Redis lỗi thì bỏ qua bước ghi cache, tiếp tục trả kết quả từ DB.
3. API `GET /api/v1/admin/pricing/config`: **luôn đọc thẳng từ DB** (không qua cache Redis vì cache không lưu trường `updated_at`).
4. Khi Admin cập nhật (`PUT /api/v1/admin/pricing/config`):
   - Validate `surge_threshold` $\in [0.01, 99.99]$ với tối đa 2 chữ số thập phân (sai $\rightarrow$ `VALIDATION_ERROR` 400).
   - Cập nhật DB: `INSERT INTO pricing_configs (id, base_fare, price_per_km, surge_threshold, updated_at) VALUES (1, $1, $2, $3, now()) ON CONFLICT (id) DO UPDATE SET base_fare = EXCLUDED.base_fare, price_per_km = EXCLUDED.price_per_km, surge_threshold = EXCLUDED.surge_threshold, updated_at = now()`.
   - Xóa cache Redis: `DEL pricing:config` (lỗi Redis chỉ log WARN). Trả về 200 OK.

### 4.2. Tính toán Khoảng cách & ETA (OSRM Fallback Haversine)
1. Gửi HTTP GET tới OSRM Public API với timeout = `OSRM_TIMEOUT_MS` (400 ms):  
   `${OSRM_BASE_URL}/route/v1/driving/${pickup_lng},${pickup_lat};${dropoff_lng},${dropoff_lat}?overview=false`.
2. Nếu OSRM trả về 200 OK và có routes:
   - $D_{\text{mét}} = \text{routes}[0].\text{distance}$ (làm tròn nguyên).
   - $ETA_{\text{giây}} = \text{routes}[0].\text{duration}$ (làm tròn nguyên).
   - Đánh dấu `source = "OSRM"`.
3. Nếu OSRM timeout ($> 400\text{ ms}$) hoặc mất mạng/lỗi HTTP:
   - Tính khoảng cách đường chim bay bằng công thức Haversine:
     $$D_{\text{mét}} = \lfloor\text{Haversine}(\text{pickup}, \text{dropoff}) \times 1.35 \times 1000 + 0.5\rfloor$$
     $$ETA_{\text{giây}} = \lfloor\frac{D_{\text{mét}} / 1000.0}{30} \times 3600 + 0.5\rfloor \quad (\text{tốc độ giả định } 30\text{ km/h})$$
   - Đánh dấu `source = "HAVERSINE"`.

### 4.3. Tính toán Hệ số Surge Pricing (UC-15)
1. Mã hóa tọa độ đón `(pickup_lat, pickup_lng)` thành mã Geohash 5 ký tự (`pickup_geohash5`) theo chuẩn chung với `dispatch-service`.
2. **Tính Nhu cầu (Demand):**
   - Loại bỏ các mốc cũ quá 5 phút trong ZSET: `ZREMRANGEBYSCORE demand:geo:<geohash5> -inf (now - 300)`.
   - Đếm số yêu cầu hợp lệ trong 5 phút trượt: $\text{Demand} = \text{ZCARD demand:geo:<geohash5>}$.
   - Nếu Redis lỗi khi thao tác ZSET Demand $\rightarrow$ Tự động Fallback gán `surge_multiplier = 1.0`.
3. **Tính Nguồn cung (Supply):**
   - Gọi nội bộ `POST /internal/v1/locations/geohash-drivers` (`geohash5`) sang `location-service` với timeout = `INTERNAL_HTTP_TIMEOUT_MS` (500 ms).
   - Nếu mảng `driver_ids` rỗng $\rightarrow \text{Supply} = 0$, **bỏ qua** việc gọi `user-service`.
   - Nếu có tài xế $\rightarrow$ Gọi MỘT lần `POST /internal/v1/users/filter-online` sang `user-service` với timeout = `INTERNAL_HTTP_TIMEOUT_MS` (500 ms).
   - $\text{Supply} = \text{len}(\text{online\_driver\_ids})$.
4. **Quy tắc tính Surge và Fallback (Mục 5(j) Hợp đồng LLD-00):**
   - Nếu gọi `location-service` hoặc `user-service` gặp lỗi/timeout $\rightarrow$ Tự động gán `surge_multiplier = 1.0` (không báo lỗi).
   - Nếu $\text{Demand} = 0 \rightarrow \text{surge\_multiplier} = 1.0$.
   - Nếu $\text{Supply} = 0$ và $\text{Demand} > 0 \rightarrow \text{surge\_multiplier} = 1.5$.
   - Trường hợp bình thường:
     $$\text{ratio} = \frac{\text{Demand} / \text{Supply}}{T}$$
     $$\text{raw\_surge} = \text{clamp}(\text{ratio},\; 1.0,\; 1.5)$$
     $$\text{surge\_multiplier} = \text{round}(\text{raw\_surge}, 2) \quad (\text{làm tròn 2 chữ số thập phân})$$
   - **Lưu ý:** `surge_multiplier` được làm tròn 2 chữ số thập phân ngay tại bước này và dùng giá trị đã làm tròn này cho cả công thức tính cước lẫn trả về trong response (không lưu cache Redis kết quả surge).

### 4.4. Tính Cước phí & Làm tròn
1. Khoảng cách km: $D_{\text{km}} = D_{\text{mét}} / 1000.0$.
2. Tính cước thô:
   $$\text{RawFare} = (\text{base\_fare} + D_{\text{km}} \times \text{price\_per\_km}) \times \text{surge\_multiplier}$$
   *(với `surge_multiplier` là giá trị đã làm tròn 2 chữ số thập phân từ Mục 4.3).*
3. Làm tròn tới 1.000 VND gần nhất (chia số nguyên với nửa lên):
   $$\text{fare} = \lfloor\frac{\text{RawFare} + 500}{1000}\rfloor \times 1000$$
4. Trả về kết quả:
   - Endpoint công khai: `distance` ($D_{\text{km}}$), `eta` = $\lceil ETA_{\text{giây}} / 60 \rceil$ (`ceil(eta_seconds / 60)`, số nguyên phút), `source`, `surge_multiplier` (đã làm tròn 2 chữ số thập phân), `base_fare`, `price_per_km`, `total_fare` ($\text{fare}$).
   - Endpoint nội bộ: `fare`, `distance_m` ($D_{\text{mét}}$), `eta_seconds` ($ETA_{\text{giây}}$), `surge_multiplier` (đã làm tròn 2 chữ số thập phân).

### 4.5. Tiến trình Tích lũy Demand từ Redis Streams (`stream:trip_events`)
1. Service khởi chạy goroutine worker tiêu thụ stream với consumer group `pricing-group` và tên consumer `pricing-consumer-1`.
2. **Khởi tạo group (Idempotent):** Thực thi `XGROUP CREATE stream:trip_events pricing-group 0 MKSTREAM`. Bỏ qua lỗi `BUSYGROUP`.
3. **Vòng lặp tiêu thụ tin nhắn (Quy tắc Mục 5(m) Hợp đồng LLD-00):**
   - *Đọc tồn đọng PEL:* Mỗi chu kỳ (2–5 giây) gọi ĐÚNG 1 lượt `XREADGROUP GROUP pricing-group pricing-consumer-1 COUNT <PEL_BATCH_COUNT> STREAMS stream:trip_events 0` (KHÔNG lặp cho tới khi hết). Đếm số lần giao bằng `XPENDING` (delivery count) hoặc bộ đếm RAM `map[msg_id]int` (dọn khi XACK). Vượt trần `MAX_DELIVERY_ATTEMPTS` thì log `ERROR` rồi `XACK`. Nếu gặp lỗi vi phạm Unique Constraint (Postgres code 23505) thì coi là duplicate $\rightarrow$ `XACK` và bỏ qua.
   - *Đọc tin mới:* Gọi `XREADGROUP GROUP pricing-group pricing-consumer-1 BLOCK 2000 STREAMS stream:trip_events >`.
4. **Xử lý sự kiện từ stream:**
   - **Phân loại sự kiện:** Đọc trực tiếp trường `type` của message (không suy đoán từ trường khác).
   - **Xử lý message hỏng:** Nếu message thiếu trường bắt buộc, sai định dạng UUID, hoặc là entry rỗng (do tin nhắn đã bị xóa bởi cơ chế `MAXLEN ~ 5000` của Redis Stream trong lúc đọc PEL) $\rightarrow$ log `ERROR` và gọi ngay `XACK stream:trip_events pricing-group <msg_id>`.
   - **Xử lý sự kiện `TripCreated` (`type == "TripCreated"`):**
     - Trích xuất: `trip_id`, `pickup_geohash5`, `created_at`.
     - *Quy tắc lọc độ tươi:* Nếu $(now - created\_at) > 300\text{s}$ (cũ hơn 5 phút do đọc lại PEL/restart) $\rightarrow$ Bỏ qua không tích lũy, gọi ngay `XACK stream:trip_events pricing-group <msg_id>`.
     - Nếu $\le 300\text{s}$:
       1. Ghi nhận vào ZSET: `ZADD demand:geo:<pickup_geohash5> <unix_created_at> <trip_id>`.
       2. Đặt TTL an toàn cho toàn bộ key: `EXPIRE demand:geo:<pickup_geohash5> 300`.
       3. Nếu ghi Redis thành công $\rightarrow$ Xác nhận `XACK stream:trip_events pricing-group <msg_id>`. Nếu gặp sự cố Redis tạm thời $\rightarrow$ **KHÔNG XACK**, để message nằm lại trong PEL và sẽ được đọc lại sau 2–5 giây ở vòng lặp kế tiếp (tối đa `MAX_DELIVERY_ATTEMPTS` lần).
   - **Các sự kiện khác:** Nếu `type` là `TripCompleted`, `TripCancelled`, `TripExpired` $\rightarrow$ Bỏ qua và `XACK`.

---

## 5. XỬ LÝ NGOẠI LỆ (EXCEPTION HANDLING)

| Tình huống ngoại lệ | Ngữ cảnh phát sinh | Hành vi xử lý & Mã lỗi |
| :--- | :--- | :--- |
| Tọa độ không hợp lệ | Tọa độ ngoài dải chuẩn ở Mục 4 LLD-00 (vĩ độ [-85.05112878, 85.05112878], kinh độ [-180, 180]) | Trả `VALIDATION_ERROR` (400). |
| Trùng điểm đón và trả | Tọa độ đón và trả giống hệt nhau | Trả `VALIDATION_ERROR` (400). |
| Tham số cấu hình giá sai | `base_fare <= 0`, `price_per_km <= 0` hoặc `surge_threshold` ngoài [0.01, 99.99] | Trả `VALIDATION_ERROR` (400). |
| OSRM Public API lỗi/timeout | OSRM timeout $> 400\text{ ms}$ hoặc mất kết nối | Tự động Fallback Haversine $\times 1.35$, trả 200 OK kèm `source = "HAVERSINE"`. |
| Service phụ thuộc lỗi | `location-service` hoặc `user-service` timeout $> 500\text{ ms}$ hoặc mất mạng | Tự động Fallback gán `surge_multiplier = 1.0` (Mục 5(j) LLD-00), trả 200 OK. |
| Redis Demand lỗi | Lỗi kết nối Redis khi đọc ZSET Demand | Tự động Fallback gán `surge_multiplier = 1.0`, trả 200 OK. |
| Redis Cache Config lỗi | Lỗi kết nối Redis khi đọc cache `pricing:config` | Đọc trực tiếp từ PostgreSQL `pricingdb`, trả 200 OK. |
| Sai vai trò Quản trị viên | Người dùng thông thường gọi API cấu hình giá | Trả `FORBIDDEN` (403). |
| Consumer gặp lỗi Redis/DB | Lỗi Redis tạm thời khi ghi Demand | KHÔNG XACK, giữ trong PEL để xử lý lại sau 2–5s (tối đa `MAX_DELIVERY_ATTEMPTS` lần). |
| Message Stream hỏng | Thiếu trường, sai kiểu, hoặc entry rỗng do MAXLEN | Log ERROR, gọi XACK loại bỏ tin nhắn hỏng. |

### 5.1. Hạn chế đã biết
- **Độ chính xác cước Fallback Haversine:** Khi OSRM mất mạng, cước phí tính bằng khoảng cách Haversine $\times 1.35$ có thể chênh lệch khoảng 5%–10% so với đường bộ thực tế. Đây là trade-off chấp nhận được nhằm đảm bảo tính sẵn sàng cao (High Availability), không bao giờ chặn luồng đặt xe của khách.

---

## 6. SỰ KIỆN & KHÓA REDIS

### 6.1. Dữ liệu Redis quản lý
- `pricing:config`: String, TTL 3600s *(Key nội bộ của pricing-service)*. Cache bảng giá `{base_fare, price_per_km, surge_threshold}`. Xóa bằng `DEL pricing:config` khi Admin cập nhật giá thành công (Mục 4.1).
- `demand:geo:<geohash5>`: ZSET, TTL 300s (5 phút). Score: Unix timestamp UTC, Member: `trip_id`. Đếm Demand trong cửa sổ trượt 5 phút bằng `ZREMRANGEBYSCORE` và `ZCARD`.

### 6.2. Kênh Pub/Sub
- `pricing-service` **không** phát hay nhận trực tiếp bất kỳ kênh Pub/Sub nào.

### 6.3. Sự kiện Redis Stream lắng nghe (`stream:trip_events`)
- **Stream:** `stream:trip_events`. Consumer Group: `pricing-group`. Consumer Name: `pricing-consumer-1`.
- **Sự kiện quan tâm:** Phân loại bằng trường `type`. Xử lý sự kiện `TripCreated` (`type="TripCreated"`, `version="1.0"`, `trip_id`, `customer_id`, `pickup_geohash5`, `created_at`). Các sự kiện khác (`TripCompleted`, `TripCancelled`, `TripExpired`) bỏ qua và `XACK`.

---

## 7. CẤU HÌNH (ENV) VÀ TÀI NGUYÊN

### 7.1. Bảng biến môi trường
| Tên biến ENV | Mặc định | Ý nghĩa & Mục đích sử dụng |
| :--- | :--- | :--- |
| `PORT` | `8004` | Cổng HTTP nội bộ của `pricing-service` trong mạng Docker. |
| `DATABASE_URL` | `postgres://pricing_user:pricing_pass@postgres:5432/pricingdb?sslmode=disable` | Chuỗi kết nối cơ sở dữ liệu `pricingdb`. |
| `REDIS_ADDR` | `redis:6379` | Địa chỉ kết nối Redis container trong Docker. |
| `REDIS_PASSWORD` | `redis_secret_pass` | Mật khẩu xác thực kết nối Redis (demo, ghi đè trong .env). |
| `DEFAULT_BASE_FARE` | `10000` | Giá cước mở cửa cơ sở mặc định ban đầu (VND). |
| `DEFAULT_PRICE_PER_KM`| `12000` | Đơn giá cước di chuyển mỗi km mặc định ban đầu (VND). |
| `SURGE_THRESHOLD` | `1.0` | Ngưỡng kích hoạt hệ số biến động giá $T$ mặc định (phải thuộc [0.01, 99.99]). |
| `OSRM_BASE_URL` | `https://router.project-osrm.org` | URL dịch vụ định tuyến OSRM công cộng. |
| `OSRM_TIMEOUT_MS` | `400` | Timeout tối đa khi gọi OSRM trước khi Fallback sang Haversine (ms). |
| `INTERNAL_HTTP_TIMEOUT_MS` | `500` | Timeout tối đa khi gọi REST nội bộ sang location-service và user-service (ms). |
| `LOCATION_SERVICE_URL`| `http://location-service:8002` | Base URL gọi nội bộ `location-service`. |
| `USER_SERVICE_URL` | `http://user-service:8001` | Base URL gọi nội bộ `user-service`. |
| `MAX_DELIVERY_ATTEMPTS` | `5` | Số lần thử lại tối đa trước khi loại bỏ message hỏng khỏi PEL (Mục 5(m) LLD-00). |
| `PEL_BATCH_COUNT` | `10` | Số lượng message tối đa đọc mỗi chu kỳ quét PEL (Mục 5(m) LLD-00). |

### 7.2. Tài nguyên & Thứ tự khởi động
- **Connection Pool PostgreSQL (Mục 5(h) LLD-00):** `MaxOpenConns = 5`, `MaxIdleConns = 2`, `ConnMaxLifetime = 30m`.
- **Ràng buộc bộ nhớ VPS 1GB:** `mem_limit` Docker đề xuất: 30 MB; `GOMEMLIMIT = 26MiB` (chống OOM).
- **Thứ tự khởi động dịch vụ (Startup Sequence):**
  1. Kết nối PostgreSQL `pricingdb` và thực thi migration bảng `pricing_configs`.
  2. Kiểm tra giá trị `SURGE_THRESHOLD` từ ENV: nếu không thuộc khoảng `[0.01, 99.99]` (hoặc có quá 2 chữ số thập phân) $\rightarrow$ Dừng khởi động với log `ERROR` rõ ràng. Nếu hợp lệ, khởi tạo bản ghi cấu hình giá mặc định từ ENV (`id = 1`) nếu bảng `pricing_configs` đang rỗng.
  3. Kết nối Redis, kiểm tra ping `PONG`.
  4. Khởi tạo consumer group `pricing-group` trên `stream:trip_events` (`MKSTREAM`, bỏ qua lỗi `BUSYGROUP`).
  5. Khởi chạy goroutine worker tiêu thụ stream (vòng lặp quét PEL định kỳ mỗi 2–5 giây kết hợp đọc tin mới `>`).
  6. Khởi động HTTP web server (Fiber) lắng nghe trên cổng `8004` tiếp nhận request.
