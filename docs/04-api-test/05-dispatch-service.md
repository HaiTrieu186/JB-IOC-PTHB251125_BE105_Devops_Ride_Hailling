# KỊCH BẢN KIỂM THỬ DỊCH VỤ ĐIỀU PHỐI (DISPATCH-SERVICE TEST SUITE)

> **Mã tài liệu:** `TEST-DISPATCH` | **Phiên bản:** `1.0`  
> **Dịch vụ kiểm thử:** `dispatch-service` (Port nội bộ: `8003`, Dev host: `127.0.0.1:8003`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD dispatch-service](../03-lld/dispatch-service.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

Bộ kịch bản kiểm thử bao gồm **28 ca kiểm thử HTTP** (chạy tự động qua Postman và script PowerShell) cùng các bước xác minh sự kiện phân tán:
1. **Kiểm tra sức khỏe:** Endpoint `GET /health` (`DP-00`).
2. **Luồng nghiệp vụ đầy đủ (Happy Path):** C1 đặt xe, D1 nhận, đón khách, bắt đầu chở, hoàn thành và quyết toán ví tự động (`DP-01a`..`DP-01f`).
3. **Tranh cuốc đồng thời (Distributed Lock):** 2 tài xế D1 và D2 cùng accept 1 chuyến xe (`DP-02`).
4. **Kiểm tra ví & Chống đặt trùng:** C3 ví 0 VND (`DP-03`), C1 đặt lần 2 khi đang có chuyến active (`DP-04`).
5. **Xử lý hết hạn & 0 tài xế:** Đặt xe tại nơi không có tài xế (`DP-05`), hết hạn tìm xe 30 giây qua worker (`DP-06`).
6. **Các nhánh hủy chuyến:** Khách hủy MATCHING/ACCEPTED, tài xế hủy ACCEPTED/PICKING_UP, chặn hủy IN_TRIP (`DP-07a`..`DP-07e`).
7. **Quản trị viên can thiệp:** Force-cancel và danh sách phân trang (`DP-08a`..`DP-08c`).
8. **Kiểm tra biên & RBAC:** Idempotent accept (`DP-09`), không được mời (`DP-10`), trip không tồn tại (`DP-11`), sai role (`DP-12a`, `DP-12b`), đua hủy/nhận (`DP-13`), quyền admin (`DP-14a`, `DP-14b`), tra cứu chuyến hiện tại (`DP-15`).

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Method & Path | Headers | Body / Query | Kỳ vọng HTTP Status & Mã lỗi | Ghi chú & Dọn dẹp |
|:---:|---|---|---|---|---|---|
| **DP-00** | Healthcheck | `GET /health` | *(None)* | *(None)* | **200 OK**<br>`{"status":"ok"}` | Liveness check |
| **DP-01a**| Khách C1 tạo chuyến P1 $\rightarrow$ P2 | `POST /api/v1/trips` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `pickup_lat: 10.7725`, `pickup_lng: 106.6980`, `dropoff_lat: 10.7951`, `dropoff_lng: 106.7218` | **201 Created**<br>`status = "MATCHING"`, có `id`, `fare`, `matching_expires_at` | D1 ONLINE + GPS quanh P1 |
| **DP-01b**| Tài xế D1 nhận cuốc (Accept) | `POST /api/v1/trips/{{trip_id}}/accept` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **200 OK**<br>`status = "ACCEPTED"`, `driver_id = {{d1_id}}` | Ghi `trip:link:{{d1_id}}` |
| **DP-01c**| Tài xế D1 đến điểm đón (Picking Up) | `POST /api/v1/trips/{{trip_id}}/picking-up` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **200 OK**<br>`status = "PICKING_UP"` | Phát pub/sub update |
| **DP-01d**| Tài xế D1 bắt đầu chở (Start Trip) | `POST /api/v1/trips/{{trip_id}}/start-trip` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **200 OK**<br>`status = "IN_TRIP"` | Phát pub/sub update |
| **DP-01e**| Tài xế D1 hoàn thành cuốc (Complete) | `POST /api/v1/trips/{{trip_id}}/complete` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **200 OK**<br>`status = "COMPLETED"`, có `completed_at` | Xóa `trip:link`, D1 về ONLINE, ví tự động quyết toán |
| **DP-01f**| Khách xem chi tiết chuyến và dòng thời gian | `GET /api/v1/trips/{{trip_id}}` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **200 OK**<br>`status = "COMPLETED"`, `status_timeline` đủ 6 bước | Timeline sắp theo `id ASC` |
| **DP-02** | D1 và D2 tranh cuốc đồng thời | `POST /api/v1/trips/{{trip_id}}/accept` | `X-User-Id: {{d1/d2_id}}`<br>`X-User-Role: DRIVER` | *(None)* | 1 người **200 OK**<br>1 người **409 Conflict** (`TRIP_ALREADY_TAKEN`) | Khóa phân tán SET NX |
| **DP-03** | Khách C3 ví thiếu tiền đặt xe | `POST /api/v1/trips` | `X-User-Id: {{c3_id}}`<br>`X-User-Role: CUSTOMER` | *(Tọa độ P1 $\rightarrow$ P2)* | **400 Bad Request**<br>`INSUFFICIENT_BALANCE` | C3 số dư = 0 VND |
| **DP-04** | Khách C1 đặt lần 2 khi đang có chuyến active | `POST /api/v1/trips` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(Tọa độ P1 $\rightarrow$ P2)* | **409 Conflict**<br>`ACTIVE_TRIP_EXISTS` | Bắt lỗi chuyến active |
| **DP-05** | Đặt xe khu vực không có tài xế (Hà Nội) | `POST /api/v1/trips` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(Tọa độ Ho Guom)* | **201 Created**<br>`status = "EXPIRED"` | 0 tài xế online gần |
| **DP-06** | Chuyến MATCHING hết hạn 30s | `GET /api/v1/trips/{{trip_id}}` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **200 OK**<br>`status = "EXPIRED"`, timeline note: `TIMEOUT_30S` | Worker 2s tự động quét |
| **DP-07a**| Khách hủy chuyến lúc MATCHING | `POST /api/v1/trips/{{trip_id}}/cancel` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | `{"reason": "Doi y"}` | **200 OK**<br>`status = "CANCELLED"` | Không có tài xế can thiệp |
| **DP-07b**| Khách hủy chuyến lúc ACCEPTED | `POST /api/v1/trips/{{trip_id}}/cancel` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **200 OK**<br>`status = "CANCELLED"` | D1 tự động hoàn tác về ONLINE |
| **DP-07c**| Tài xế hủy chuyến lúc ACCEPTED | `POST /api/v1/trips/{{trip_id}}/cancel` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | `{"reason": "Hong xe"}` | **200 OK**<br>`status = "CANCELLED"` | D1 tự động hoàn tác về ONLINE |
| **DP-07d**| Tài xế hủy chuyến lúc PICKING_UP | `POST /api/v1/trips/{{trip_id}}/cancel` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **200 OK**<br>`status = "CANCELLED"` | D1 tự động hoàn tác về ONLINE |
| **DP-07e**| Khách hoặc tài xế cố hủy lúc IN_TRIP | `POST /api/v1/trips/{{trip_id}}/cancel` | `X-User-Id: {{c1/d1_id}}`<br>`X-User-Role: CUSTOMER/DRIVER` | *(None)* | **400 Bad Request**<br>`INVALID_TRIP_STATUS` | Chuyến đang chạy không cho hủy |
| **DP-08a**| Admin Force-cancel lý do rỗng | `POST /api/v1/admin/trips/{{trip_id}}/force-cancel` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"reason": "   "}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Lý do là bắt buộc |
| **DP-08b**| Admin Force-cancel thành công | `POST /api/v1/admin/trips/{{trip_id}}/force-cancel` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | `{"reason": "Su co nghiem trong"}` | **200 OK**<br>`status = "CANCELLED"`, `cancelled_by = "ADMIN"` | Giải phóng D1 và xóa link |
| **DP-08c**| Admin xem danh sách chuyến có phân trang | `GET /api/v1/admin/trips?page=1&limit=10` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | *(None)* | **200 OK**<br>`trips` (mảng), `total > 0`, `page = 1` | Sắp xếp `created_at DESC` |
| **DP-09** | Cùng tài xế accept lần 2 | `POST /api/v1/trips/{{trip_id}}/accept` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **200 OK**<br>`status = "ACCEPTED"`, `driver_id = {{d1_id}}` | Idempotent |
| **DP-10** | Tài xế ngoài bán kính (D5) accept | `POST /api/v1/trips/{{trip_id}}/accept` | `X-User-Id: {{d5_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **403 Forbidden**<br>`NOT_OFFERED` | D5 không trong invited_drivers |
| **DP-11** | Accept trip_id không tồn tại | `POST /api/v1/trips/{{fake_id}}/accept` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **404 Not Found**<br>`TRIP_NOT_FOUND` | ID ngẫu nhiên |
| **DP-12a**| Tài xế gọi POST /trips | `POST /api/v1/trips` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(Tọa độ P1 $\rightarrow$ P2)* | **403 Forbidden**<br>`FORBIDDEN` | Sai vai trò |
| **DP-12b**| Khách gọi POST /accept | `POST /api/v1/trips/{{trip_id}}/accept` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Sai vai trò |
| **DP-13** | Khách hủy MATCHING rồi tài xế accept | `POST /api/v1/trips/{{trip_id}}/accept` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER` | *(None)* | **400 Bad Request**<br>`INVALID_TRIP_STATUS` | D1 vẫn giữ ONLINE, không kẹt BUSY |
| **DP-14a**| Admin xem chi tiết qua GET /trips/:id | `GET /api/v1/trips/{{trip_id}}` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Admin phải dùng /admin/trips |
| **DP-14b**| Admin truyền page không hợp lệ | `GET /api/v1/admin/trips?page=abc` | `X-User-Id: {{admin_id}}`<br>`X-User-Role: ADMIN` | *(None)* | **400 Bad Request**<br>`VALIDATION_ERROR` | Parse integer fail |
| **DP-15** | Khách xem chuyến hiện tại khi rảnh | `GET /api/v1/trips/current` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER` | *(None)* | **200 OK**<br>`data = null` | Trả null khi không có cuốc active |

---

## 3. LỆNH CURL MẪU CHẠY THỦ CÔNG

### 3.1. Đặt xe (Customer C1):
```powershell
$body = '{"pickup_lat":10.7725,"pickup_lng":106.6980,"pickup_address":"Cho Ben Thanh, Quan 1, TP.HCM","dropoff_lat":10.7951,"dropoff_lng":106.7218,"dropoff_address":"Landmark 81, Binh Thanh, TP.HCM"}'
$body | curl.exe -s --data-binary "@-" -H "Content-Type: application/json" -H "X-User-Id: d6ace9fc-5290-46a5-826d-d69f38c76c73" -H "X-User-Role: CUSTOMER" http://127.0.0.1:8003/api/v1/trips
```

### 3.2. Nhận chuyến (Driver D1):
```powershell
curl.exe -s -X POST -H "X-User-Id: b6b9c2e8-ba47-4c06-a273-96ee9deec91e" -H "X-User-Role: DRIVER" http://127.0.0.1:8003/api/v1/trips/<trip_id>/accept
```

### 3.3. Hoàn thành chuyến (Driver D1):
```powershell
curl.exe -s -X POST -H "X-User-Id: b6b9c2e8-ba47-4c06-a273-96ee9deec91e" -H "X-User-Role: DRIVER" http://127.0.0.1:8003/api/v1/trips/<trip_id>/complete
```
