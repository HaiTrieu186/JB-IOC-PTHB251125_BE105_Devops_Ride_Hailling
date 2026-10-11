# KỊCH BẢN KIỂM THỬ DỊCH VỤ TÍNH GIÁ (PRICING-SERVICE TEST SUITE)

> **Mã tài liệu:** `TEST-PRICING` | **Phiên bản:** `1.0`  
> **Dịch vụ kiểm thử:** `pricing-service` (Port nội bộ: `8004`, Dev host: `127.0.0.1:8004`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD pricing-service](../03-lld/pricing-service.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

Bộ kịch bản kiểm thử bao gồm **21 ca kiểm thử**, bao phủ toàn diện:
1. **Kiểm tra sức khỏe:** Endpoint `GET /health` (`P-00`).
2. **Tính cước ước tính công khai & nội bộ:** API ước tính cước lộ trình $P1 \rightarrow P2$ cho khách hàng (`P-01`) và cho dispatch-service (`P-02`).
3. **Quản trị cấu hình bảng giá (FR-31):** Xem và cập nhật bảng giá của Admin (`P-03a`, `P-03b`).
4. **Kiểm soát truy cập & Phân quyền (RBAC):** Chặn người dùng thông thường truy cập API quản trị (`P-04a`, `P-04b`).
5. **Kiểm tra ràng buộc dữ liệu (Validation Errors):** Bắt các lỗi tọa độ, điểm đón trùng trả, giá trị cấu hình không hợp lệ (`P-05a`..`P-05f`).
6. **Hệ số giá biến động (Surge Pricing):** Kiểm thử công thức $\text{surge} = \text{clamp}((\text{Demand}/\text{Supply})/T, 1.0, 1.5)$ với các ngưỡng $T$ khác nhau (`P-06a`..`P-06d` và `P-06f` chạy tự động trên Postman; `P-06e` là ca chạy tay sau khi GPS hết hạn 15s).
7. **Ca chạy tay ngoại lệ & Fallback:** Kịch bản ép timeout OSRM fallback Haversine (`P-07`), fallback Surge khi service phụ thuộc ngừng hoạt động (`P-08`), và Surge khi Supply = 0 (`P-06e`). Các ca này không có trong Postman collection tự động.

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Method & Path | Headers | Body / Tham số | Kỳ vọng HTTP Status & Mã lỗi | Ghi chú & Dữ liệu tham chiếu |
|:---:|---|---|---|---|---|---|
| **P-00** | Kiểm tra trạng thái dịch vụ (Healthcheck) | `GET /health` | *(None)* | *(None)* | **200 OK**<br>`{"data":{"status":"ok"},"success":true}` | Chạy bằng Postman. Endpoint kiểm tra liveness/readiness container. |
| **P-01** | Khách hàng C1 tra cứu ước tính cước P1 $\rightarrow$ P2 | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`total_fare > 0` | Chạy bằng Postman. Lộ trình Chợ Bến Thành $\rightarrow$ Landmark 81. Source là `"OSRM"` hoặc `"HAVERSINE"` (nếu OSRM timeout). |
| **P-02** | Dispatch-service gọi báo giá nội bộ | `POST /internal/v1/pricing/estimate` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`fare > 0`, `distance_m > 0` | Chạy bằng Postman. Endpoint nội bộ trả `distance_m` và `eta_seconds`. |
| **P-03a** | Quản trị viên xem cấu hình bảng giá | `GET /api/v1/admin/pricing/config` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | *(None)* | **200 OK**<br>`base_fare, price_per_km, surge_threshold, updated_at` | Chạy bằng Postman. Đọc trực tiếp từ cơ sở dữ liệu `pricingdb`. |
| **P-03b** | Quản trị viên cập nhật cấu hình bảng giá | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 1.2}` | **200 OK**<br>`surge_threshold = 1.2` | Chạy bằng Postman. Cập nhật cấu hình và vô hiệu hóa cache Redis `pricing:config`. |
| **P-04a** | Khách hàng cố xem cấu hình bảng giá Admin | `GET /api/v1/admin/pricing/config` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Chạy bằng Postman. Bắt lỗi: *"Chỉ quản trị viên mới có quyền truy cập"*. |
| **P-04b** | Khách hàng cố sửa cấu hình bảng giá Admin | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"base_fare": 5000, "price_per_km": 5000, "surge_threshold": 1.0}` | **403 Forbidden**<br>`FORBIDDEN` | Chạy bằng Postman. Bắt lỗi: *"Chỉ quản trị viên mới có quyền truy cập"*. |
| **P-05a** | Tra cứu giá thiếu Header xác thực | `POST /api/v1/pricing/estimate` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy bằng Postman. Bắt lỗi: *"Yêu cầu xác thực tài khoản"*. |
| **P-05b** | Điểm đón trùng điểm trả | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7725, "dropoff_lng": 106.6980}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"Điểm đón và điểm trả không được trùng nhau"*. |
| **P-05c** | Vĩ độ vượt dải Mercator hợp lệ | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 89.0, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"Vĩ độ không hợp lệ, phải thuộc [-85.05112878, 85.05112878]"*. |
| **P-05d** | Cập nhật giá với `base_fare <= 0` | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 0, "price_per_km": 12000, "surge_threshold": 1.0}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"base_fare và price_per_km phải lớn hơn 0"*. |
| **P-05e** | Cập nhật giá với `surge_threshold` ngoài dải | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 100.0}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"surge_threshold phải nằm trong khoảng [0.01, 99.99]"*. |
| **P-05f** | Cập nhật giá với `surge_threshold` > 2 chữ số thập phân | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 1.255}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Chạy bằng Postman. Bắt lỗi: *"surge_threshold chỉ được tối đa 2 chữ số thập phân"*. |
| **P-06a** | Surge Pricing: $T=1.2$ (Supply=2, Demand=3) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.25` | **Chạy bằng Postman**. Pre-request tự PUT $T=1.2$ và tự record GPS D1, D2. $(3/2)/1.2 = 1.25$. |
| **P-06b** | Surge Pricing: $T=1.4$ (Supply=2, Demand=3) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.07` | **Chạy bằng Postman**. Pre-request tự PUT $T=1.4$. $(3/2)/1.4 = 1.0714... \rightarrow 1.07$. |
| **P-06c** | Surge Pricing: $T=5.0$ (Giới hạn dưới) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.0` | **Chạy bằng Postman**. Pre-request tự PUT $T=5.0$. $(3/2)/5.0 = 0.30 < 1.0 \rightarrow$ clamp 1.0. |
| **P-06d** | Surge Pricing: $T=0.05$ (Giới hạn trên) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.5` | **Chạy bằng Postman**. Pre-request tự PUT $T=0.05$. $(3/2)/0.05 = 30.0 > 1.5 \rightarrow$ clamp 1.5. |
| **P-06e** | Surge Pricing: Supply = 0, Demand > 0 | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.5` | **Ca CHẠY TAY (Postman không có)**: Chờ > 15s sau GPS record, Supply = 0, Demand = 3 $\rightarrow$ surge = 1.5. |
| **P-06f** | Surge Pricing: Demand = 0 (Ô Tân Sơn Nhất) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.8185, "pickup_lng": 106.6588, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.0` | **Chạy bằng Postman**. Điểm đón ô `w3gve` không có Demand trong 5 phút $\rightarrow$ surge = 1.0. |
| **P-07** | Fallback Haversine khi ép timeout OSRM $\le 1\text{ms}$ | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`source = "HAVERSINE"` | **Ca CHẠY TAY (Postman không có)**: Sửa `OSRM_TIMEOUT_MS=1` trong `.env`, recreate container. |
| **P-08** | Fallback an toàn khi location-service tắt | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.0` | **Ca CHẠY TAY (Postman không có)**: Stop location-service. LLD-00 5(j): không chặn đặt xe của khách. |

---

## 3. HƯỚNG DẪN THỰC THI & BƯỚC DỌN DẸP TRẠNG THÁI (TEARDOWN)

### 3.1. Tiền điều kiện trước khi chạy folder Postman `03 pricing-service`

Trước khi chạy folder Postman `03 pricing-service`, người kiểm thử **CHỈ CẦN LÀM TAY 2 VIỆC** sau:

1. **Đưa tài xế D1 và D2 về trạng thái ONLINE:**
   Lấy ID của D1 (`<d1_id>`) và D2 (`<d2_id>`) từ biến Postman `d1_id` và `d2_id` (sinh ra sau khi chạy folder `00 Seed`).  
   Gửi request `PATCH /api/v1/driver/status` tới `user-service` (port `8001`):
   ```bash
   # Đặt D1 ONLINE
   curl -s -X PATCH http://127.0.0.1:8001/api/v1/driver/status \
     -H "Content-Type: application/json" \
     -H "X-User-Id: <d1_id>" \
     -H "X-User-Role: DRIVER" \
     -d '{"status":"ONLINE"}'

   # Đặt D2 ONLINE
   curl -s -X PATCH http://127.0.0.1:8001/api/v1/driver/status \
     -H "Content-Type: application/json" \
     -H "X-User-Id: <d2_id>" \
     -H "X-User-Role: DRIVER" \
     -d '{"status":"ONLINE"}'
   ```

2. **Khởi tạo Nhu cầu (Demand = 3) qua Redis Stream (Demand sống 300 giây nên không vội):**
   ```powershell
   $now = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
   docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCreated version "1.0" trip_id "11111111-0000-0000-0000-000000000001" customer_id "c1" pickup_geohash5 "w3gv7" created_at $now
   docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCreated version "1.0" trip_id "11111111-0000-0000-0000-000000000002" customer_id "c1" pickup_geohash5 "w3gv7" created_at $now
   docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCreated version "1.0" trip_id "11111111-0000-0000-0000-000000000003" customer_id "c1" pickup_geohash5 "w3gv7" created_at $now
   ```

> **Lưu ý:** Bước ghi nhận GPS cho D1 và D2 tại điểm đón $P1$ (`10.7725, 106.6980`) đã được **tự động hóa hoàn toàn** trong Pre-request Script của `P-06a` (bằng `pm.sendRequest` tới `location-service`), đảm bảo hoàn thành ngay trước khi ước tính cước mà không phụ thuộc vào thao tác tay trong vòng 15 giây.

### 3.2. Dọn dẹp trạng thái sau kiểm thử (Teardown / Idempotent)

1. **Khôi phục cấu hình bảng giá về mặc định:**
   ```bash
   curl -s -X PUT http://127.0.0.1:8004/api/v1/admin/pricing/config \
     -H "Content-Type: application/json" \
     -H "X-User-Role: ADMIN" \
     -d '{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 1.0}'
   ```

2. **Dọn dẹp ZSET demand trong Redis:**
   ```bash
   docker exec redis redis-cli -a redis_secret_pass DEL demand:geo:w3gv7
   ```

3. **Đưa tài xế D1 và D2 về trạng thái OFFLINE (không để BUSY):**
   ```bash
   curl -s -X PATCH http://127.0.0.1:8001/api/v1/driver/status \
     -H "Content-Type: application/json" \
     -H "X-User-Id: <d1_id>" \
     -H "X-User-Role: DRIVER" \
     -d '{"status":"OFFLINE"}'

   curl -s -X PATCH http://127.0.0.1:8001/api/v1/driver/status \
     -H "Content-Type: application/json" \
     -H "X-User-Id: <d2_id>" \
     -H "X-User-Role: DRIVER" \
     -d '{"status":"OFFLINE"}'
   ```

4. **Khôi phục cấu hình môi trường OSRM (nếu đã thực hiện P-07):**
   Khôi phục `OSRM_TIMEOUT_MS=400` trong `.env` rồi cập nhật lại service:
   ```bash
   docker compose up -d pricing-service
   ```

5. **Bật lại location-service (nếu đã dừng khi thử P-08):**
   ```bash
   docker compose start location-service
   ```
