# KỊCH BẢN KIỂM THỬ DỊCH VỤ ĐỊNH VỊ (LOCATION-SERVICE TEST SUITE)

> **Mã tài liệu:** `TEST-LOC` | **Phiên bản:** `1.1`  
> **Dịch vụ kiểm thử:** `location-service` (Port nội bộ: `8002`, Dev host: `127.0.0.1:8002`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD location-service](../03-lld/location-service.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

Bộ kịch bản kiểm thử bao gồm **16 mã ca kiểm thử** (`L-01a` đến `L-07b`), trong đó Postman Collection thực thi tự động **14 request** (`L-01a`..`L-06e`), riêng 2 ca kiểm tra vòng đời TTL và cơ chế dọn dẹp ZSCORE (`L-07a`, `L-07b`) được thực hiện **CHẠY TAY** qua terminal:
1. Ghi nhận tọa độ GPS thời gian thực của 5 tài xế mẫu D1–D5 (`L-01a`..`L-01e`).
2. Quét ứng viên quanh điểm đón P1 trong bán kính 5 km (`L-02`), xác minh lọc bán kính và sắp xếp khoảng cách tăng dần (D1..D4 có mặt, D5 bị loại).
3. Truy vấn tài xế theo ô Geohash 5 (`L-03`) phục vụ Supply cho `pricing-service`.
4. Giám sát tài xế đang hoạt động cho Quản trị viên (`L-04`) và kiểm tra phân quyền RBAC (`L-05`).
5. Bắt các lỗi dữ liệu đầu vào (`L-06a`..`L-06e`).
6. Kiểm tra cơ chế lọc độ tươi GPS 15s và ZREM lười sau 30s TTL (`L-07a`, `L-07b` - Chạy tay bằng CLI).

> [!NOTE]
> **Đừng đổi tọa độ D1:** D1 (lng `106.6992`) cách biên ô `w3gv7` (`106.69921875`) khoảng 2 m.

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Method & Path | Headers | Body / Tham số | Kỳ vọng HTTP Status & Mã lỗi | Ghi chú & Dữ liệu tham chiếu |
|:---:|---|---|---|---|---|---|
| **L-01a** | Ghi nhận vị trí GPS tài xế D1 (gần P1) | `POST /internal/v1/locations/record` | `Content-Type: application/json` | `{"driver_id": "{{d1_id}}", "latitude": 10.7738, "longitude": 106.6992}` | **200 OK**<br>`{"success": true}` | D1 tại Chợ Bến Thành, ô `w3gv7`. |
| **L-01b** | Ghi nhận vị trí GPS tài xế D2 | `POST /internal/v1/locations/record` | `Content-Type: application/json` | `{"driver_id": "{{d2_id}}", "latitude": 10.7775, "longitude": 106.7032}` | **200 OK**<br>`{"success": true}` | D2 cách P1 ~795 m, ô `w3gvk`. |
| **L-01c** | Ghi nhận vị trí GPS tài xế D3 | `POST /internal/v1/locations/record` | `Content-Type: application/json` | `{"driver_id": "{{d3_id}}", "latitude": 10.7855, "longitude": 106.7105}` | **200 OK**<br>`{"success": true}` | D3 cách P1 ~1,989 m, ô `w3gvk`. |
| **L-01d** | Ghi nhận vị trí GPS tài xế D4 | `POST /internal/v1/locations/record` | `Content-Type: application/json` | `{"driver_id": "{{d4_id}}", "latitude": 10.7985, "longitude": 106.7240}` | **200 OK**<br>`{"success": true}` | D4 cách P1 ~4,054 m, ô `w3gvk`. |
| **L-01e** | Ghi nhận vị trí GPS tài xế D5 (ngoài 5 km) | `POST /internal/v1/locations/record` | `Content-Type: application/json` | `{"driver_id": "{{d5_id}}", "latitude": 10.8185, "longitude": 106.6588}` | **200 OK**<br>`{"success": true}` | D5 tại Tân Sơn Nhất (~6.67 km), ô `w3gve`. |
| **L-02** | Quét ứng viên gần điểm đón P1 bán kính 5 km | `POST /internal/v1/locations/candidates` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "radius_km": 5.0}` | **200 OK** | Trả về D1, D2, D3, D4 theo đúng thứ tự khoảng cách tăng dần (195 m, 795 m, 1989 m, 4054 m). D5 bị loại khỏi danh sách. |
| **L-03** | Lấy tài xế trong ô Geohash 5 của P1 (`w3gv7`) | `POST /internal/v1/locations/geohash-drivers` | `Content-Type: application/json` | `{"geohash5": "w3gv7"}` | **200 OK** | Chỉ có D1 nằm trong ô `w3gv7` $\rightarrow$ `driver_ids: ["{{d1_id}}"]`. |
| **L-04** | Quản trị viên xem tài xế active | `GET /api/v1/admin/drivers/active` | `X-User-Role: ADMIN`<br>`X-User-Id: {{admin_id}}` | *(None)* | **200 OK** | Trả về danh sách tài xế active kèm tọa độ và `last_seen` định dạng ISO 8601 UTC. |
| **L-05** | Tài xế cố gọi API Quản trị viên | `GET /api/v1/admin/drivers/active` | `X-User-Role: DRIVER`<br>`X-User-Id: {{d1_id}}` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Middleware chặn: *"Chỉ quản trị viên mới có quyền truy cập"*. |
| **L-06a** | Record tọa độ sai (vĩ độ ngoài [-85.05, 85.05]) | `POST /internal/v1/locations/record` | `Content-Type: application/json` | `{"driver_id": "{{d1_id}}", "latitude": 90.0, "longitude": 106.6992}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Thông điệp: *"Tọa độ không hợp lệ (latitude [-85.05112878, 85.05112878], longitude [-180, 180])"*. |
| **L-06b** | Record với driver_id sai UUID | `POST /internal/v1/locations/record` | `Content-Type: application/json` | `{"driver_id": "not-a-valid-uuid", "latitude": 10.7738, "longitude": 106.6992}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"driver_id không hợp lệ (phải đúng định dạng UUID)"*. |
| **L-06c** | Candidates với bán kính vượt tối đa (> 10 km) | `POST /internal/v1/locations/candidates` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "radius_km": 15.0}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"Bán kính tìm kiếm không hợp lệ (phải > 0 và <= MAX_RADIUS_KM)"*. |
| **L-06d** | Geohash-drivers với mã sai độ dài | `POST /internal/v1/locations/geohash-drivers` | `Content-Type: application/json` | `{"geohash5": "w3gv"}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"Mã geohash5 không hợp lệ (phải đúng 5 ký tự)"*. |
| **L-06e** | Candidates với bán kính bằng 0 | `POST /internal/v1/locations/candidates` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "radius_km": 0}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi: *"Bán kính tìm kiếm không hợp lệ (phải > 0 và <= MAX_RADIUS_KM)"*. |
| **L-07a** | Kịch bản TTL GPS > 15s (Chờ 16s sau record) | `POST /internal/v1/locations/candidates` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "radius_km": 5.0}` | **200 OK**<br>`candidates: []` | **Ca CHẠY TAY (không có trong Postman)**.<br>GPS quá 15s bị lọc bỏ khỏi candidates. Kiểm tra Redis key vẫn còn member:<br>`docker exec redis redis-cli -a <REDIS_PASSWORD> ZSCORE drivers:geo <d1_id>` $\rightarrow$ trả về score (chưa hết 30s TTL). |
| **L-07b** | Kịch bản ZREM lười sau 30s TTL (Chờ > 31s) | `POST /internal/v1/locations/candidates` | `Content-Type: application/json` | `{"pickup_lat": 10.7725, "pickup_lng": 106.6980, "radius_km": 5.0}` | **200 OK**<br>`candidates: []` | **Ca CHẠY TAY (không có trong Postman)**.<br>Key `last_seen` hết hạn $\rightarrow$ Candidates gọi MGET phát hiện `nil` và kích hoạt ZREM lười. Kiểm tra Redis member đã bị xóa:<br>`docker exec redis redis-cli -a <REDIS_PASSWORD> ZSCORE drivers:geo <d1_id>` $\rightarrow$ trả về `nil`. |

---

## 3. BƯỚC DỌN DẸP TRẠNG THÁI (TEARDOWN / CLEANUP)

- Kịch bản hoàn toàn Idempotent: Mỗi lần kiểm thử lại, chỉ cần chạy lại `L-01a`..`L-01e` để cập nhật lại tọa độ và làm tươi mốc thời gian `last_seen` cho 5 tài xế.
