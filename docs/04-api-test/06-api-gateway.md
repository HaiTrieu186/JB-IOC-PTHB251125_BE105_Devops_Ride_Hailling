# KỊCH BẢN KIỂM THỬ CỔNG API (API-GATEWAY TEST SUITE)

> **Mã tài liệu:** `TEST-GATEWAY` | **Phiên bản:** `1.0`  
> **Dịch vụ kiểm thử:** `api-gateway` (Port tiếp nhận: `127.0.0.1:8080`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD api-gateway](../03-lld/api-gateway.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

Bộ kịch bản kiểm thử bao gồm **15 ca kiểm thử tự động qua Postman** (Folder `06 api-gateway`) và **2 ca kiểm thử khả năng chịu lỗi hạ tầng (Fail-Closed / Service Unavailable)** chạy thủ công bằng script PowerShell:
1. **Đăng nhập & Luồng nghiệp vụ cơ bản:** Đăng nhập tài khoản qua Gateway (`GW-01`), tài xế D1 cập nhật trạng thái ONLINE (`GW-02`), khách C1 ước tính cước lộ trình (`GW-03`), tra cứu số dư ví cá nhân (`GW-04`), Quản trị viên xem cấu hình cước (`GW-05`).
2. **Kiểm soát phân quyền Quản trị (Admin RBAC):** Chặn tài khoản khách hàng CUSTOMER gọi API quản trị `/api/v1/admin/*` (`GW-06`).
3. **Xác thực JWT & Bảo mật Token:** Bắt lỗi khi không gửi token (`GW-07`), từ chối dứt khoát token có thuật toán `none` (`GW-08`).
4. **Chống mạo danh danh tính (Header Sanitization):** Xóa sạch header giả mạo `X-User-Id` / `X-User-Role` khi không có token (`GW-09`) và khi có token hợp lệ (`GW-10`).
5. **Chặn API nội bộ từ Internet:** Chặn tuyệt đối mọi request vào đường dẫn `/internal/*` khi không có token (`GW-11`) và khi có token hợp lệ (`GW-12`).
6. **Xử lý Route lạ & Method sai:** Chặn đường dẫn không tồn tại trả 404 (`GW-13`), chặn HTTP method sai trả 404 thay vì 405 (`GW-14`).
7. **Đăng xuất & Thu hồi phiên qua Danh sách đen Redis:** Đăng xuất tài khoản C2 và chặn sử dụng lại access token cũ (`GW-15`).
8. **Chịu lỗi dịch vụ đích & Redis (Fail-Closed):** Service đích không phản hồi hoặc AI service chưa triển khai trả 503 (`GW-16`), Redis gặp sự cố trả 503 ngắt xác thực an toàn (`GW-17`).

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Method & Path | Headers | Body / Tham số | Kỳ vọng HTTP Status & Mã lỗi | Ghi chú & Dữ liệu tham chiếu |
|:---:|---|---|---|---|---|---|
| **GW-01** | Đăng nhập tài khoản C1 qua Gateway | `POST /api/v1/auth/login` | `Content-Type: application/json` | `{"phone_or_email":"0901000001","password":"Test@1234"}` | **200 OK**<br>`access_token`, `refresh_token` | Chạy tự động Postman. Lưu token C1 cho các ca sau. |
| **GW-02** | Tài xế D1 bật trạng thái ONLINE | `PATCH /api/v1/driver/status` | `Content-Type: application/json`<br>`Authorization: Bearer {{d1_token}}` | `{"status":"ONLINE"}` | **200 OK**<br>`status = "ONLINE"` | Chạy tự động Postman. Chuẩn bị tài xế sẵn sàng nhận cuốc. |
| **GW-03** | Khách C1 ước tính cước $P1 \rightarrow P2$ | `POST /api/v1/pricing/estimate` | `Content-Type: application/json`<br>`Authorization: Bearer {{gw_c1_token}}` | `{"pickup_lat":10.7725,"pickup_lng":106.6980,"dropoff_lat":10.7951,"dropoff_lng":106.7218}` | **200 OK**<br>`total_fare > 0` | Chạy tự động Postman. Proxy thành công sang pricing-service. |
| **GW-04** | Khách C1 tra cứu thông tin ví | `GET /api/v1/wallets/me` | `Authorization: Bearer {{gw_c1_token}}` | *(None)* | **200 OK**<br>`balance`, `currency="VND"` | Chạy tự động Postman. Gateway bóc JWT gắn `X-User-*` sang payment. |
| **GW-05** | Admin tra cứu cấu hình giá | `GET /api/v1/admin/pricing/config` | `Authorization: Bearer {{admin_token}}` | *(None)* | **200 OK**<br>`base_fare`, `price_per_km` | Chạy tự động Postman. Admin guard cho phép role `ADMIN`. |
| **GW-06** | Khách C1 gọi endpoint Admin bị chặn | `GET /api/v1/admin/pricing/config` | `Authorization: Bearer {{gw_c1_token}}` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Chạy tự động Postman. Admin guard từ chối role `CUSTOMER`. |
| **GW-07** | Gọi endpoint yêu cầu auth nhưng thiếu Token | `GET /api/v1/wallets/me` | *(None)* | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy tự động Postman. Thiếu header `Authorization`. |
| **GW-08** | Gửi Token mang thuật toán `none` | `GET /api/v1/wallets/me` | `Authorization: Bearer <alg_none_token>` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy tự động Postman. Từ chối thuật toán không an toàn. |
| **GW-09** | Gửi header giả mạo danh tính không token | `GET /api/v1/wallets/me` | `X-User-Id: 00000000-...`<br>`X-User-Role: ADMIN` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy tự động Postman. Sanitize xóa header giả $\rightarrow$ thiếu token $\rightarrow$ 401. |
| **GW-10** | Gửi header giả kèm Token C1 hợp lệ | `GET /api/v1/wallets/me` | `Authorization: Bearer {{gw_c1_token}}`<br>`X-User-Id: 00000000-...`<br>`X-User-Role: ADMIN` | *(None)* | **200 OK**<br>Trả về đúng ví của C1 | Chạy tự động Postman. Sanitize xóa header giả và gán lại từ claims token. |
| **GW-11** | Chặn truy cập API nội bộ từ ngoài (không token) | `POST /internal/v1/users/filter-online` | `Content-Type: application/json` | `{"driver_ids":[]}` | **404 Not Found**<br>`NOT_FOUND` | Chạy tự động Postman. Tiền tố `/internal/*` bị chặn ngay từ Sanitize. |
| **GW-12** | Chặn truy cập API nội bộ từ ngoài (kèm token) | `POST /internal/v1/users/filter-online` | `Content-Type: application/json`<br>`Authorization: Bearer {{gw_c1_token}}` | `{"driver_ids":[]}` | **404 Not Found**<br>`NOT_FOUND` | Chạy tự động Postman. Chặn 404 TRƯỚC bước kiểm tra JWT. |
| **GW-13** | Gọi đường dẫn không tồn tại trong hệ thống | `GET /api/v1/nonexistent-route-xyz` | *(None)* | *(None)* | **404 Not Found**<br>`NOT_FOUND` | Chạy tự động Postman. Catch-all `/*` trả 404 NOT_FOUND. |
| **GW-14** | Gọi Method không hợp lệ (`GET /api/v1/trips`) | `GET /api/v1/trips` | *(None)* | *(None)* | **404 Not Found**<br>`NOT_FOUND` | Chạy tự động Postman. Phương thức sai bị ép về 404 NOT_FOUND (không 405). |
| **GW-15** | Đăng xuất C2 và thử dùng lại token cũ | `GET /api/v1/wallets/me` | `Authorization: Bearer {{c2_token}}` | Pre-request: Logout C2 qua `POST /api/v1/auth/logout` | **401 Unauthorized**<br>`UNAUTHORIZED` | Chạy tự động Postman. Tra cứu Redis `blacklist:<jti>` phát hiện token đã thu hồi. |
| **GW-16** | Service đích sập hoặc timeout | `GET /api/v1/admin/reports` *(hoặc stop pricing)* | `Authorization: Bearer {{admin_token}}` | *(None)* | **503 Service Unavailable**<br>`SERVICE_UNAVAILABLE` | **Ca CHẠY TAY**. Proxy timeout/conn-refused chuyển thành lỗi 503. |
| **GW-17** | Redis sập gây lỗi tra cứu danh sách đen | `GET /api/v1/wallets/me` | `Authorization: Bearer {{c1_token}}` | *(None khi stop redis)* | **503 Service Unavailable**<br>`SERVICE_UNAVAILABLE` | **Ca CHẠY TAY**. Cơ chế Fail-Closed từ chối phục vụ khi hạ tầng bảo mật lỗi. |

---

## 3. HƯỚNG DẪN THỰC THI

### 3.1. Chạy tự động qua Newman
Thực thi kiểm thử liên tiếp 2 lần cùng folder `00 Seed`:

```powershell
npx newman run docs/04-api-test/postman/ride-hailing.postman_collection.json -e docs/04-api-test/postman/local.postman_environment.json --folder "00 Seed" --folder "06 api-gateway"
```

### 3.2. Ca chạy tay kiểm tra khả năng chịu lỗi (`GW-16`, `GW-17`)

#### Kiểm tra 503 khi service đích sập (`GW-16`):
```powershell
docker stop pricing-service
curl.exe -i -s -X POST "http://127.0.0.1:8080/api/v1/pricing/estimate" -H "Content-Type: application/json" -H "Authorization: Bearer $c1Token" -d '{"pickup_lat":10.7725,"pickup_lng":106.6980,"dropoff_lat":10.7951,"dropoff_lng":106.7218}'
# Kỳ vọng: HTTP/1.1 503 Service Unavailable, mã SERVICE_UNAVAILABLE
docker start pricing-service
```

#### Kiểm tra 503 Fail-Closed khi Redis sập (`GW-17`):
```powershell
docker stop redis
curl.exe -i -s -X GET "http://127.0.0.1:8080/api/v1/wallets/me" -H "Authorization: Bearer $c1Token"
# Kỳ vọng: HTTP/1.1 503 Service Unavailable, mã SERVICE_UNAVAILABLE
docker start redis
```
