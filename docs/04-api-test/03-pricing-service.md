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
6. **Hệ số giá biến động (Surge Pricing):** Kiểm thử công thức $\text{surge} = \text{clamp}((\text{Demand}/\text{Supply})/T, 1.0, 1.5)$ với các ngưỡng $T$ khác nhau (`P-06a`..`P-06f`).
7. **Ca chạy tay ngoại lệ & Fallback:** Kịch bản ép timeout OSRM fallback Haversine (`P-07`) và fallback Surge khi service phụ thuộc ngừng hoạt động (`P-08`).

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Method & Path | Headers | Body / Tham số | Kỳ vọng HTTP Status & Mã lỗi | Ghi chú & Dữ liệu tham chiếu |
|:---:|---|---|---|---|---|---|
| **P-00** | Kiểm tra trạng thái dịch vụ (Healthcheck) | `GET /health` | *(None)* | *(None)* | **200 OK**<br>`{"data":{"status":"ok"},"success":true}` | Endpoint kiểm tra liveness/readiness container. |
| **P-01** | Khách hàng C1 tra cứu ước tính cước P1 $\rightarrow$ P2 | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`total_fare > 0` | Lộ trình Chợ Bến Thành $\rightarrow$ Landmark 81. Source là `"OSRM"` hoặc `"HAVERSINE"` (nếu OSRM timeout). |
| **P-02** | Dispatch-service gọi báo giá nội bộ | `POST /internal/v1/pricing/estimate` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`fare > 0`, `distance_m > 0` | Endpoint nội bộ trả `distance_m` và `eta_seconds`. |
| **P-03a** | Quản trị viên xem cấu hình bảng giá | `GET /api/v1/admin/pricing/config` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | *(None)* | **200 OK**<br>`base_fare, price_per_km, surge_threshold, updated_at` | Đọc trực tiếp từ cơ sở dữ liệu `pricingdb`. |
| **P-03b** | Quản trị viên cập nhật cấu hình bảng giá | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 1.2}` | **200 OK**<br>`surge_threshold = 1.2` | Cập nhật cấu hình và vô hiệu hóa cache Redis `pricing:config`. |
| **P-04a** | Khách hàng cố xem cấu hình bảng giá Admin | `GET /api/v1/admin/pricing/config` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Bắt lỗi: *"Chỉ quản trị viên mới có quyền truy cập"*. |
| **P-04b** | Khách hàng cố sửa cấu hình bảng giá Admin | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"base_fare": 5000, "price_per_km": 5000, "surge_threshold": 1.0}` | **403 Forbidden**<br>`FORBIDDEN` | Bắt lỗi: *"Chỉ quản trị viên mới có quyền truy cập"*. |
| **P-05a** | Tra cứu giá thiếu Header xác thực | `POST /api/v1/pricing/estimate` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **401 Unauthorized**<br>`UNAUTHORIZED` | Bắt lỗi: *"Yêu cầu xác thực tài khoản"*. |
| **P-05b** | Điểm đón trùng điểm trả | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7725, "dropoff_lng": 106.6980}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"Điểm đón và điểm trả không được trùng nhau"*. |
| **P-05c** | Vĩ độ vượt dải Mercator hợp lệ | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 89.0, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"Vĩ độ không hợp lệ, phải thuộc [-85.05112878, 85.05112878]"*. |
| **P-05d** | Cập nhật giá với `base_fare <= 0` | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 0, "price_per_km": 12000, "surge_threshold": 1.0}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"base_fare và price_per_km phải lớn hơn 0"*. |
| **P-05e** | Cập nhật giá với `surge_threshold` ngoài dải | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 100.0}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"surge_threshold phải nằm trong khoảng [0.01, 99.99]"*. |
| **P-05f** | Cập nhật giá với `surge_threshold` > 2 chữ số thập phân | `PUT /api/v1/admin/pricing/config` | `Content-Type: application/json`<br>`X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 1.255}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"surge_threshold chỉ được tối đa 2 chữ số thập phân"*. |
| **P-06a** | Surge Pricing: $T=1.2$ (Supply=2, Demand=3) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.25` | $(3/2)/1.2 = 1.25$. Nằm trong $(1.0, 1.5)$, chứng minh công thức chính xác. |
| **P-06b** | Surge Pricing: $T=1.4$ (Supply=2, Demand=3) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.07` | $(3/2)/1.4 = 1.0714... \rightarrow 1.07$. Chứng minh làm tròn 2 chữ số. |
| **P-06c** | Surge Pricing: $T=5.0$ (Giới hạn dưới) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.0` | $(3/2)/5.0 = 0.30 < 1.0 \rightarrow$ Bị clamp tối thiểu 1.0. |
| **P-06d** | Surge Pricing: $T=0.05$ (Giới hạn trên) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.5` | $(3/2)/0.05 = 30.0 > 1.5 \rightarrow$ Bị clamp tối đa 1.5. |
| **P-06e** | Surge Pricing: Supply = 0, Demand > 0 | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.5` | Chờ > 15s sau GPS record, Supply = 0, Demand = 3 $\rightarrow$ surge = 1.5. |
| **P-06f** | Surge Pricing: Demand = 0 (Ô Tân Sơn Nhất) | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.8185, "pickup_lng": 106.6588, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.0` | Điểm đón ô `w3gve` không có Demand trong 5 phút $\rightarrow$ surge = 1.0. |
| **P-07** | Fallback Haversine khi ép timeout OSRM $\le 1\text{ms}$ | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`source = "HAVERSINE"` | **Ca CHẠY TAY** (sửa `OSRM_TIMEOUT_MS=1` trong `.env`, recreate container). |
| **P-08** | Fallback an toàn khi location-service tắt | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "dropoff_lat": 10.7951, "dropoff_lng": 106.7218}` | **200 OK**<br>`surge_multiplier = 1.0` | **Ca CHẠY TAY** (stop location-service). LLD-00 5(j): không chặn đặt xe của khách. |

---

## 3. HƯỚNG DẪN THỰC THI & BƯỚC DỌN DẸP TRẠNG THÁI (TEARDOWN)

### 3.1. Thiết lập tiền điều kiện cho các ca Surge Pricing (`P-06a`..`P-06f`)
1. **Khởi tạo Nhu cầu (Demand = 3) qua Redis Stream:**
   ```powershell
   $now = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
   docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCreated version "1.0" trip_id "11111111-0000-0000-0000-000000000001" customer_id "c1" pickup_geohash5 "w3gv7" created_at $now
   docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCreated version "1.0" trip_id "11111111-0000-0000-0000-000000000002" customer_id "c1" pickup_geohash5 "w3gv7" created_at $now
   docker exec redis redis-cli -a redis_secret_pass XADD stream:trip_events "*" type TripCreated version "1.0" trip_id "11111111-0000-0000-0000-000000000003" customer_id "c1" pickup_geohash5 "w3gv7" created_at $now
   ```
2. **Khởi tạo Nguồn cung (Supply = 2) qua location-service:**
   Bắn gói tin GPS cho D1 và D2 tại điểm đón $P1$ (ô `w3gv7`):
   ```bash
   curl -s -X POST http://127.0.0.1:8002/internal/v1/locations/record -H "Content-Type: application/json" -d '{"driver_id": "b6b9c2e8-ba47-4c06-a273-96ee9deec91e", "latitude": 10.7725, "longitude": 106.6980}'
   curl -s -X POST http://127.0.0.1:8002/internal/v1/locations/record -H "Content-Type: application/json" -d '{"driver_id": "f00a41df-95f0-4dfb-8a86-ebec3848149a", "latitude": 10.7725, "longitude": 106.6980}'
   ```
3. **Thực thi gọi báo giá trong vòng 15 giây** sau khi record GPS.

### 3.2. Dọn dẹp trạng thái sau kiểm thử (Teardown / Idempotent)
- Khôi phục cấu hình bảng giá về mặc định:
  ```bash
  curl -s -X PUT http://127.0.0.1:8004/api/v1/admin/pricing/config -H "Content-Type: application/json" -H "X-User-Role: ADMIN" -d '{"base_fare": 10000, "price_per_km": 12000, "surge_threshold": 1.0}'
  ```
- Dọn dẹp ZSET demand trong Redis:
  ```bash
  docker exec redis redis-cli -a redis_secret_pass DEL demand:geo:w3gv7
  ```
