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
| **GW-18** | Thu hồi token với blacklist value chuỗi rỗng (T0) | `GET /api/v1/wallets/me` | `Authorization: Bearer <t0_token>` | Redis `blacklist:t0-empty-val` set value `""` | **401 Unauthorized**<br>`UNAUTHORIZED` | **Ca CHẠY TAY**. Key tồn tại là thu hồi (không so sánh value). |
| **GW-19** | Chuyển tiếp query string và phân trang (T1) | `GET /api/v1/admin/trips?status=COMPLETED&page=1&limit=1` | `Authorization: Bearer {{admin_token}}` | Tham số query string | **200 OK**<br>Khớp 100% khi gọi thẳng dispatch | **Ca CHẠY TAY**. Proxy chuyển tiếp nguyên vẹn query string sang service đích. |
| **GW-20** | Khớp route tĩnh `/trips/current` trước tham số động `:id` (T2) | `GET /api/v1/trips/current` | `Authorization: Bearer {{c1_token}}` | *(None)* | **200 OK**<br>`data: null` | **Ca CHẠY TAY**. Không bị router hiểu nhầm `current` thành UUID `:id`. |
| **GW-21** | Chống mạo danh nâng cao với header giả mạo mạnh (T3) | `GET /api/v1/wallets/me`<br>`GET /api/v1/admin/trips` | `Authorization: Bearer {{c1_token}}`<br>`X-User-Id: <id D1>`<br>`X-User-Role: ADMIN` | *(None)* | **200 OK** (trả đúng ví C1)<br>**403 Forbidden** (chặn Admin API) | **Ca CHẠY TAY**. Sanitize triệt để mọi header giả mạo từ client. |
| **GW-22** | Chặn các đường dẫn bất thường / path traversal (T4) | `GET /api/v1/trips/x/../../admin/trips`<br>`GET /api/v1/trips/..%2fadmin%2ftrips`<br>`GET /api/v1/trips/%2e%2e/admin/trips`<br>`GET /API/V1/ADMIN/TRIPS`<br>`GET /api/v1/admin//trips` | `Authorization: Bearer {{c1_token}}` | Tham số đường dẫn lạ | **404 / 400 / 403**<br>Tuyệt đối không 200 Admin | **Ca CHẠY TAY**. Không thể bypass phân quyền bằng biến thể đường dẫn. |
| **GW-23** | Timeout dịch vụ đích 15 giây (T5) | `POST /api/v1/pricing/estimate` | `Authorization: Bearer {{c1_token}}` | Pause `pricing-service` | **503 Service Unavailable**<br>`SERVICE_UNAVAILABLE` | **Ca CHẠY TAY**. Gateway trả 503 sau ~15s timeout. |
| **GW-24** | Đo kiểm tài nguyên RAM Gateway dưới tải (T6) | `GET /api/v1/wallets/me` | `Authorization: Bearer {{c1_token}}` | 200 requests (50 worker song song) | **200 OK** (100% requests thành công) | **Ca CHẠY TAY**. Đo RAM đỉnh so với định mức 25m. |

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

### 3.3. Các ca kiểm thử chạy tay bổ sung (`GW-18` đến `GW-24`)

#### GW-18: Thu hồi Token với Blacklist value chuỗi rỗng (`T0`)
- **Mục đích:** Chứng minh bản vá logic `IsBlacklisted`: key tồn tại trong Redis (`err == nil`) là token bị thu hồi, không phụ thuộc vào `val != ""` (cho phép cả value là chuỗi rỗng `""`).
- **Kịch bản:**
  1. Ký JWT HS256 hợp lệ với claims `user_id = C1`, `role = CUSTOMER`, `jti = "t0-empty-val"`, `exp = +10 phút`.
  2. Gọi `GET /api/v1/wallets/me` khi chưa set blacklist -> **200 OK** (token hợp lệ).
  3. Thực hiện `SET blacklist:t0-empty-val ""` trên Redis. Gọi lại -> **401 Unauthorized** (`"Token đã bị thu hồi"`).
  4. Thực hiện `DEL blacklist:t0-empty-val`. Gọi lại -> **200 OK**.
- **Kết quả thực tế:**
  - Bước (1) Gọi đối chứng khi chưa blacklist: **HTTP 200 OK**, Body: `{"data":{"wallet_id":"ca529da2-4678-4d4c-bb4e-30cab7190f9d","balance":1718000,"currency":"VND"},"error":null,"success":true}`
  - Bước (2) SET `blacklist:t0-empty-val ""` trên Redis: `SET` trả về `OK`, `GET` trả về chuỗi rỗng `""` (`repr: '\n'`), `EXISTS` trả về `1`. Gọi lại `GET /api/v1/wallets/me` -> **HTTP 401 Unauthorized**, Body: `{"data":null,"error":{"code":"UNAUTHORIZED","message":"Token đã bị thu hồi"},"success":false}`.
  - Bước (3) DEL `blacklist:t0-empty-val`: `DEL` trả về `1`, `EXISTS` trả về `0`. Gọi lại `GET /api/v1/wallets/me` -> **HTTP 200 OK**, Body: `{"data":{"wallet_id":"ca529da2-4678-4d4c-bb4e-30cab7190f9d","balance":1718000,"currency":"VND"},"error":null,"success":true}`.

#### GW-19: Chuyển tiếp Query string và phân trang (`T1`)
- **Lệnh thực thi:**
  ```powershell
  # Qua Gateway (Port 8080) bằng token ADMIN:
  curl.exe -i -s -X GET "http://127.0.0.1:8080/api/v1/admin/trips?status=COMPLETED&page=1&limit=1" -H "Authorization: Bearer $adminToken"

  # Gọi trực tiếp dispatch-service (Port 8003):
  curl.exe -i -s -X GET "http://127.0.0.1:8003/api/v1/admin/trips?status=COMPLETED&page=1&limit=1" -H "X-User-Id: d4250302-ef38-418b-aa2e-b320c3868265" -H "X-User-Role: ADMIN"
  ```
- **Kết quả thực tế:** Cả 2 request trả về status **200 OK** và body JSON khớp 100% từng byte (`Content-Length: 680`, phân trang đúng `page: 1`, `total: 1`, lọc đúng `status: "COMPLETED"`).

#### GW-20: Định tuyến route tĩnh `/trips/current` (`T2`)
- **Lệnh thực thi:**
  ```powershell
  curl.exe -i -s -X GET "http://127.0.0.1:8080/api/v1/trips/current" -H "Authorization: Bearer $c1Token"
  ```
- **Kết quả thực tế:** HTTP **200 OK**, body `{"data":null,"error":null,"success":true}`. Không bị router hiểu nhầm `current` thành UUID `:id` (không bị lỗi `VALIDATION_ERROR` / 400).

#### GW-21: Chống mạo danh danh tính nâng cao (`T3`)
- **Lệnh thực thi:**
  ```powershell
  # Gửi kèm token C1 nhưng giả mạo X-User-Id của D1 và X-User-Role=ADMIN vào /wallets/me
  curl.exe -i -s -X GET "http://127.0.0.1:8080/api/v1/wallets/me" -H "Authorization: Bearer $c1Token" -H "X-User-Id: f9c952a0-5cba-4be9-9735-dc9f5ebfee71" -H "X-User-Role: ADMIN"

  # Gửi kèm token C1 nhưng giả mạo role ADMIN vào /admin/trips
  curl.exe -i -s -X GET "http://127.0.0.1:8080/api/v1/admin/trips" -H "Authorization: Bearer $c1Token" -H "X-User-Id: f9c952a0-5cba-4be9-9735-dc9f5ebfee71" -H "X-User-Role: ADMIN"
  ```
- **Kết quả thực tế:**
  - `GET /api/v1/wallets/me` trả về **200 OK** đúng ví của C1 (`wallet_id: ca529da2-4678-4d4c-bb4e-30cab7190f9d`), header giả bị sanitize triệt để.
  - `GET /api/v1/admin/trips` trả về **403 Forbidden** (`code: "FORBIDDEN"`), role ADMIN giả bị xóa sạch và từ chối truy cập.

#### GW-22: Xử lý các đường dẫn bất thường / Path Traversal (`T4`)
- **Các biến thể kiểm thử:**
  - `GET /api/v1/trips/x/../../admin/trips` -> **404 Not Found** (`NOT_FOUND`)
  - `GET /api/v1/trips/..%2fadmin%2ftrips` -> **400 Bad Request** (`VALIDATION_ERROR`, "Mã chuyến xe không hợp lệ" từ dispatch-service)
  - `GET /api/v1/trips/%2e%2e/admin/trips` -> **404 Not Found** (`NOT_FOUND`)
  - `GET /API/V1/ADMIN/TRIPS` -> **403 Forbidden** (`FORBIDDEN`, middleware Admin Guard chặn)
  - `GET /api/v1/admin//trips` -> **404 Not Found** (`NOT_FOUND`)
- **Kết quả thực tế:** 100% các biến thể đều bị chặn (404/400/403), tuyệt đối không lộ dữ liệu quản trị (không trả 200).

#### GW-23: Kiểm tra Gateway Timeout 15s (`T5`)
- **Lệnh thực thi:**
  ```powershell
  docker pause pricing-service
  curl.exe -i -s -w "\nTime: %{time_total}s\n" -X POST "http://127.0.0.1:8080/api/v1/pricing/estimate" -H "Content-Type: application/json" -H "Authorization: Bearer $c1Token" -d '{"pickup_lat":10.7725,"pickup_lng":106.6980,"dropoff_lat":10.7951,"dropoff_lng":106.7218}'
  docker unpause pricing-service
  ```
- **Kỳ vọng:** HTTP/1.1 503 Service Unavailable, code `SERVICE_UNAVAILABLE` sau 14-17 giây.
- **Kết quả thực tế:** Thực thi script `scratch/test_gw23_t5.py` (khối `try/finally` bảo đảm luôn unpause `pricing-service`):
  - Lệnh `docker pause pricing-service`: returncode 0.
  - Gọi `POST /api/v1/pricing/estimate` qua port 8080: Gateway chờ 15.00 giây (đúng cấu hình `PROXY_TIMEOUT_MS=15000`) và trả về:
    - Status: **HTTP/1.1 503 Service Unavailable**
    - Body: `{"data":null,"error":{"code":"SERVICE_UNAVAILABLE","message":"Dịch vụ đích không phản hồi hoặc quá thời gian chờ"},"success":false}`
    - Thời gian đo: **15.00 giây**.
  - Khối `finally` unpause thành công `pricing-service` (returncode 0), container phục hồi trạng thái `healthy`.


#### GW-24: Đo kiểm tài nguyên RAM Gateway dưới tải 200 requests (`T6`)
- **Kịch bản thực thi:** Script Python stdlib chạy 200 requests `GET /api/v1/wallets/me` bằng token C1 qua port 8080 (50 worker song song chia 2 đợt).
- **Kết quả thực tế:**
  - Tỷ lệ thành công: **200/200 requests (100%)**, 0 lỗi.
  - Thời gian xử lý: mỗi đợt 100 request hoàn tất trong ~0.12 giây.
  - RAM lúc rảnh: **4.30 MiB**.
  - RAM đỉnh dưới tải: **9.56 MiB** (~38.2% định mức 25 MB). Định mức `mem_limit: 25m` và `GOMEMLIMIT: 22MiB` hoàn toàn đủ và tối ưu.

---

## 4. HẠN CHẾ ĐÃ BIẾT

Lỗi 4xx của Fiber ngoài 401/403/404/405 (ví dụ 413 Payload Too Large khi body vượt quá 4MB, 400 Bad Request khi cú pháp request hỏng) giữ nguyên mã HTTP status của Fiber nhưng trả `code: "SERVICE_UNAVAILABLE"` trong envelope lỗi JSON vì tài liệu LLD không định nghĩa mã lỗi riêng cho các trường hợp ngoại lệ này.


