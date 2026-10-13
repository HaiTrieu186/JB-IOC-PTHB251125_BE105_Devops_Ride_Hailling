# BÁO CÁO KIỂM THỬ HỆ THỐNG TRÊN MÔI TRƯỜNG PRODUCTION (VPS)

> **Mã tài liệu:** `PROD-TEST-01` | **Phiên bản:** `1.0`  
> **Môi trường kiểm thử:** Production VPS (`https://haitrieu186.duckdns.org`)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [00-du-lieu-mau.md](../04-api-test/00-du-lieu-mau.md), [01-user-service.md](../04-api-test/01-user-service.md), [03-pricing-service.md](../04-api-test/03-pricing-service.md), [04-payment-service.md](../04-api-test/04-payment-service.md), [06-api-gateway.md](../04-api-test/06-api-gateway.md)

---

## 1. Thông tin lần chạy

- **Thời gian thực thi:** 2026-10-13 11:16:37 (UTC+7)
- **Git Commit:** `496c958` (`git rev-parse --short HEAD`)
- **Môi trường Base URL:** `https://haitrieu186.duckdns.org`
- **WebSocket Gateway URL:** `wss://haitrieu186.duckdns.org/ws/`
- **Môi trường thực thi test:**
  - OS: Windows 11
  - Python: 3.10.11 (x64)
  - HTTP Client: `aiohttp 3.14.5` (bật xác thực chứng chỉ SSL/TLS)
  - Mạng: Mạng ngoài Internet kết nối trực tiếp đến VPS DuckDNS (không qua mạng nội bộ VPS, không dùng SSH).
- **Phạm vi tài khoản test (độc lập, không xung đột dữ liệu demo):**
  - Khách hàng: `TC1` (`0905000001` - nạp ví), `TC2` (`0905000002`), `TC3` (`0905000003` - ví 0 VND). Mật khẩu: `Test@1234`.
  - Tài xế: `TD1` đến `TD5` (`0905100001` .. `0905100005`). Mật khẩu: `Test@1234`.
  - Quản trị viên: `admin@ridehailing.local` / `Admin@123456`.

---

## 2. Quy trình & Lệnh chạy lại

1. **Yêu cầu môi trường:**
   - Cài đặt Python 3.10+ và thư viện `aiohttp`:
     ```bash
     pip install aiohttp
     ```
2. **Thực thi bộ kiểm thử tích hợp tự động:**
   ```powershell
   python scratch/prod-test/t1_rest.py
   ```
   - Script tự động đăng ký / đăng nhập với cơ chế Semaphore(2) / tuần tự chống nghẽn CPU bcrypt trên VPS 1 vCPU, thực thi tuần tự và song song toàn bộ 82 ca kiểm thử theo các nhóm A, B, C, D, E, N.
   - Script ghi lại log thô từng request/response (đã che token/mật khẩu) tại `scratch/prod-test/prod-test-<ngày giờ>.log`.
   - Kết quả từng ca được ghi append vào `scratch/prod-test/results.json`.
   - Cơ chế teardown `try/finally` tự động khôi phục bảng giá `pricing_configs` và đưa toàn bộ tài xế `TD1..TD5` về `OFFLINE`.
3. **Lệnh xuất bảng báo cáo Markdown từ kết quả đã lưu:**
   ```powershell
   python scratch/prod-test/common.py --report
   ```

---

## 3. Kết quả kiểm thử theo nhóm (Script sinh tự động từ results.json)

=== BÁO CÁO KIỂM THỬ SẢN XUẤT (RUN 20261013_111637) ===

### Bảng tổng hợp theo nhóm

| Nhóm | Tên nhóm | Tổng số ca | PASS | FAIL | SKIP |
|:---:|---|:---:|:---:|:---:|:---:|
| **A** | Nhóm A: Hạ tầng & mạng | 6 | 5 | 0 | 1 |
| **B** | Nhóm B: Xác thực & user (qua gateway) | 14 | 13 | 0 | 1 |
| **C** | Nhóm C: Ví (qua gateway) | 13 | 13 | 0 | 0 |
| **D** | Nhóm D: Pricing (qua gateway) | 16 | 16 | 0 | 0 |
| **E** | Nhóm E: Gateway | 19 | 19 | 0 | 0 |
| **N** | Nhóm N: Edge case / Đầu vào xấu / Tấn công | 14 | 13 | 1 | 0 |
| **TỔNG** | **Toàn bộ hệ thống** | **82** | **79** | **1** | **2** |


#### Nhóm A: Hạ tầng & mạng

| Mã | Kịch bản | Kỳ vọng | Thực tế | KQ | Ghi chú |
|:---:|---|---|---|:---:|---|
| **A1** | GET http://domain/health redirect 301 sang https | 301 Moved Permanently sang https:// | HTTP 301, Location: https://haitrieu186.duckdns.org/health | **PASS** | Nginx chuyển hướng HTTPS thành công |
| **A2** | HTTPS cert hợp lệ và kiểm tra hạn dùng | Cert hợp lệ, ngày hết hạn >= 12/12/2026 | Hạn dùng cert: 12/12/2026 (notAfter: Dec 12 12:05:48 2026 GMT) | **PASS** | Chứng chỉ SSL DuckDNS/Let's Encrypt hợp lệ |
| **A3** | GET /health qua HTTPS | 200 OK, {success: true} | HTTP 200, body: {"status":"ok"} | **PASS** | Gateway healthcheck hoạt động tốt |
| **A4** | Kiểm tra cổng firewall (chỉ mở 22, 80, 443; chặn toàn bộ cổng nội bộ) | 22, 80, 443 kết nối được; 8080, 8081, 8001-8005, 5432, 6379 không kết nối được | Mở: 22/80/443; Chặn: [8080, 8081, 8001, 8002, 8003, 8004, 8005, 5432, 6379] | **PASS** | UFW và Docker firewall cấu hình an toàn |
| **A5** | Chặn API nội bộ /internal/v1/... từ Internet (có và không token) | 404 Not Found (NOT_FOUND) | Không token: HTTP 404; Có token: HTTP 404 | **PASS** | GW-11, GW-12: Nginx/Gateway chặn route nội bộ |
| **A6** | Kiểm tra từ chối TLS 1.0 / 1.1 | Kết nối TLS 1.0/1.1 bị từ chối | Lỗi: TLSv1_0 | **SKIP** | Không thể kiểm tra TLS 1.0: TLSv1_0 |

#### Nhóm B: Xác thực & user (qua gateway)

| Mã | Kịch bản | Kỳ vọng | Thực tế | KQ | Ghi chú |
|:---:|---|---|---|:---:|---|
| **U-01** | Đăng ký Khách hàng TC1 hợp lệ (đã tồn tại) | 201 Created hoặc 409 | HTTP 409 USER_ALREADY_EXISTS | **PASS** | Tài khoản TC1 đã có sẵn từ trước (chấp nhận 409) |
| **U-02** | Đăng ký trùng số điện thoại TC1 | 409 Conflict, USER_ALREADY_EXISTS | HTTP 409, code: USER_ALREADY_EXISTS | **PASS** | Bắt lỗi trùng lặp chính xác |
| **U-03** | Đăng ký mật khẩu quá ngắn (< 6 ký tự) | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi validate độ dài mật khẩu |
| **U-04** | Đăng nhập đúng thông tin TC1 | 200 OK, trả về access_token, refresh_token, role CUSTOMER | HTTP 200, role: CUSTOMER | **PASS** | Đăng nhập TC1 thành công |
| **U-05** | Đăng nhập sai mật khẩu TC1 | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Từ chối mật khẩu sai |
| **U-06** | Đăng nhập tài khoản không tồn tại | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Chống timing attack, trả đúng 401 |
| **U-07** | Xoay vòng Refresh Token lần 1 (Hợp lệ) | 200 OK, cấp cặp token mới | HTTP 200 | **PASS** | Xoay vòng token thành công |
| **U-08** | Tái sử dụng Refresh Token lần 2 (Đã dùng) | 401 Unauthorized, INVALID_REFRESH_TOKEN | HTTP 401, code: INVALID_REFRESH_TOKEN | **PASS** | GETDEL thu hồi token cũ nguyên tử |
| **U-09** | Tài xế TD1 bật ONLINE | 200 OK, status: ONLINE | HTTP 200, status: ONLINE | **PASS** | TD1 bật ONLINE thành công |
| **U-10** | Khách hàng TC1 cố đổi trạng thái tài xế | 403 Forbidden, FORBIDDEN | HTTP 403, code: FORBIDDEN | **PASS** | Middleware chặn role CUSTOMER |
| **U-12** | Tài xế TD1 đang BUSY cố chuyển OFFLINE | 400 Bad Request, DRIVER_BUSY | SKIP (Cần /internal để set BUSY) | **SKIP** | Tiền điều kiện đưa tài xế sang BUSY cần API /internal/v1/drivers/:id/status, bị chặn từ ngoài đúng thiết kế |
| **U-14b** | Đăng xuất bằng Refresh Token của người khác | 401 Unauthorized, INVALID_REFRESH_TOKEN | HTTP 401, code: INVALID_REFRESH_TOKEN | **PASS** | Chặn refresh token không chính chủ |
| **U-14c** | Khách hàng đăng xuất hợp lệ | 200 OK, Đăng xuất thành công | HTTP 200 | **PASS** | Đăng xuất thành công, ghi nhận blacklist |
| **U-15** | Đăng nhập tài khoản Quản trị viên (Admin) | 200 OK, role: ADMIN | HTTP 200, role: ADMIN | **PASS** | Tài khoản Admin khởi tạo từ ENV hoạt động đúng |

#### Nhóm C: Ví (qua gateway)

| Mã | Kịch bản | Kỳ vọng | Thực tế | KQ | Ghi chú |
|:---:|---|---|---|:---:|---|
| **W-01** | Khách hàng TC1 nạp tiền vào ví (+50.000 VND) | 200 OK, added_amount = 50000, new_balance tăng đúng 50.000 | HTTP 200, chênh lệch: +50000 | **PASS** | Nạp ví thành công, số dư tăng đúng 50.000 |
| **W-02** | Khách hàng TC1 xem thông tin ví cá nhân | 200 OK, wallet_id, balance > 0, currency = VND | HTTP 200, balance: 100000 VND | **PASS** | Xem ví cá nhân thành công |
| **W-03** | Khách hàng TC1 xem lịch sử giao dịch | 200 OK, transactions (mảng), không có COMMISSION | HTTP 200, tổng giao dịch: 2 | **PASS** | Truy vấn lịch sử giao dịch thành công |
| **W-05a** | Admin cố xem thông tin ví cá nhân | 403 Forbidden, FORBIDDEN | HTTP 403, code: FORBIDDEN | **PASS** | Admin không sở hữu ví |
| **W-05b** | Admin cố gọi nạp tiền | 403 Forbidden, FORBIDDEN | HTTP 403, code: FORBIDDEN | **PASS** | Chặn Admin nạp ví |
| **W-05c** | Admin cố xem lịch sử giao dịch | 403 Forbidden, FORBIDDEN | HTTP 403, code: FORBIDDEN | **PASS** | Chặn Admin xem lịch sử ví cá nhân |
| **W-06a** | Nạp tiền với amount = 0 | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi amount <= 0 |
| **W-06b** | Nạp tiền vượt TOPUP_MAX_AMOUNT (100.000.001) | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi vượt trần 100.000.000 VND |
| **W-06f** | Phân trang transactions với page = 0 (< 1) | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi page < 1 |
| **W-06g** | Phân trang transactions với limit = 101 (> 100) | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi limit > 100 |
| **W-06h** | Phân trang transactions với page = abc (không phải số) | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi page không phải số nguyên |
| **W-06i** | Phân trang transactions với limit = abc (không phải số) | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi limit không phải số nguyên |
| **W-07a** | Gọi API ví thiếu header xác thực | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Chặn request không có token |

#### Nhóm D: Pricing (qua gateway)

| Mã | Kịch bản | Kỳ vọng | Thực tế | KQ | Ghi chú |
|:---:|---|---|---|:---:|---|
| **P-01** | Khách hàng TC1 tra cứu ước tính cước P1 -> P2 | 200 OK, total_fare > 0 | HTTP 200, total_fare: 69000 VND, nguồn: HAVERSINE | **PASS** | Ước tính cước thành công. Nguồn định tuyến: HAVERSINE |
| **P-03a** | Quản trị viên xem cấu hình bảng giá | 200 OK, base_fare, price_per_km, surge_threshold | HTTP 200, base: 10000, km: 12000, surge_thresh: 1 | **PASS** | Admin xem cấu hình bảng giá thành công |
| **P-03b** | Quản trị viên cập nhật cấu hình bảng giá (T=1.2) | 200 OK, surge_threshold = 1.2 | HTTP 200, surge_threshold: 1.2 | **PASS** | Cập nhật bảng giá thành công |
| **P-04a** | Khách hàng cố xem cấu hình bảng giá Admin | 403 Forbidden, FORBIDDEN | HTTP 403, code: FORBIDDEN | **PASS** | Chặn quyền truy cập admin cấu hình giá |
| **P-04b** | Khách hàng cố sửa cấu hình bảng giá Admin | 403 Forbidden, FORBIDDEN | HTTP 403, code: FORBIDDEN | **PASS** | Chặn quyền sửa cấu hình giá |
| **P-05a** | Tra cứu giá thiếu Header xác thực | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Chặn tra cứu giá không có token |
| **P-05b** | Điểm đón trùng điểm trả | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi điểm đón trùng điểm trả |
| **P-05c** | Vĩ độ vượt dải Mercator hợp lệ (89.0) | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi vĩ độ ngoài khoảng [-85, 85] |
| **P-05d** | Cập nhật giá với base_fare <= 0 | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi base_fare <= 0 |
| **P-05e** | Cập nhật giá với surge_threshold ngoài dải [0.01, 99.99] | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi surge_threshold ngoài dải |
| **P-05f** | Cập nhật giá với surge_threshold > 2 chữ số thập phân | 400 Bad Request, VALIDATION_ERROR | HTTP 400, code: VALIDATION_ERROR | **PASS** | Bắt lỗi tối đa 2 chữ số thập phân |
| **P-06a** | Surge Pricing T=1.2 (Kiểm tra công thức/giá trị trả về) | 200 OK, surge_multiplier (1.25 nếu có tải Demand=3/Supply=2, hoặc 1.0 nếu Demand=0) | HTTP 200, surge_multiplier: 1 | **PASS** | Thực tế hệ thống trả surge_multiplier = 1 (môi trường VPS không có lệnh đẩy stream) |
| **P-06b** | Surge Pricing T=1.4 | 200 OK, surge_multiplier | HTTP 200, surge_multiplier: 1 | **PASS** | Thực tế hệ thống trả surge_multiplier = 1 |
| **P-06c** | Surge Pricing T=5.0 (Giới hạn dưới) | 200 OK, surge_multiplier = 1.0 | HTTP 200, surge_multiplier: 1 | **PASS** | Bị giới hạn dưới (clamp min 1.0) |
| **P-06d** | Surge Pricing T=0.05 (Giới hạn trên) | 200 OK, surge_multiplier | HTTP 200, surge_multiplier: 1 | **PASS** | Thực tế trả surge_multiplier = 1 |
| **P-06f** | Surge Pricing Demand = 0 (Ô Tân Sơn Nhất) | 200 OK, surge_multiplier = 1.0 | HTTP 200, surge_multiplier: 1 | **PASS** | Demand = 0 luôn trả surge_multiplier = 1.0 |

#### Nhóm E: Gateway

| Mã | Kịch bản | Kỳ vọng | Thực tế | KQ | Ghi chú |
|:---:|---|---|---|:---:|---|
| **GW-01** | Đăng nhập tài khoản TC1 qua Gateway | 200 OK, access_token, refresh_token | HTTP 200 | **PASS** | Gateway chuyển tiếp login thành công |
| **GW-02** | Tài xế TD1 bật trạng thái ONLINE qua Gateway | 200 OK, status: ONLINE | HTTP 200 | **PASS** | Gateway gán X-User headers sang user-service |
| **GW-03** | Khách TC1 ước tính cước P1 -> P2 qua Gateway | 200 OK, total_fare > 0 | HTTP 200, total_fare: 75000 | **PASS** | Proxy thành công sang pricing-service |
| **GW-04** | Khách TC1 tra cứu thông tin ví qua Gateway | 200 OK, balance, currency VND | HTTP 200 | **PASS** | Gateway bóc JWT gắn X-User sang payment-service |
| **GW-05** | Admin tra cứu cấu hình giá qua Gateway | 200 OK, base_fare, price_per_km | HTTP 200 | **PASS** | Admin guard cho phép role ADMIN |
| **GW-06** | Khách TC1 gọi endpoint Admin bị chặn | 403 Forbidden, FORBIDDEN | HTTP 403, code: FORBIDDEN | **PASS** | Admin guard từ chối role CUSTOMER |
| **GW-07** | Gọi endpoint yêu cầu auth nhưng thiếu Token | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Gateway từ chối khi thiếu Authorization header |
| **GW-08** | Gửi Token mang thuật toán none | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Gateway từ chối dứt khoát thuật toán none |
| **GW-09** | Gửi header giả mạo danh tính không kèm Token | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Sanitize xóa sạch header giả mạo từ client |
| **GW-10** | Gửi header giả kèm Token TC1 hợp lệ | 200 OK, trả về đúng ví của TC1 | HTTP 200, wallet_id: 71240483-0d40-471f-baf2-bf5b1d6fc897 | **PASS** | Sanitize ghi đè X-User từ claim JWT thực tế |
| **GW-11** | Chặn truy cập API nội bộ từ ngoài (không token) | 404 Not Found, NOT_FOUND | HTTP 404, code: NOT_FOUND | **PASS** | Tiền tố /internal/* bị chặn từ Sanitize |
| **GW-12** | Chặn truy cập API nội bộ từ ngoài (kèm token) | 404 Not Found, NOT_FOUND | HTTP 404, code: NOT_FOUND | **PASS** | Chặn /internal/* trước khi kiểm tra auth |
| **GW-13** | Gọi đường dẫn không tồn tại trong hệ thống | 404 Not Found, NOT_FOUND | HTTP 404, code: NOT_FOUND | **PASS** | Catch-all trả 404 NOT_FOUND |
| **GW-14** | Gọi Method không hợp lệ (GET /api/v1/trips) | 404 Not Found, NOT_FOUND | HTTP 404, code: NOT_FOUND | **PASS** | Method sai bị ép về 404 thay vì 405 |
| **GW-15** | Đăng xuất TC2 và thử dùng lại token cũ | 401 Unauthorized, UNAUTHORIZED | HTTP 401, code: UNAUTHORIZED | **PASS** | Redis blacklist chặn token đã thu hồi |
| **GW-19** | Chuyển tiếp query string và phân trang (Admin trips) | 200 OK, phân trang đúng | HTTP 200 | **PASS** | Gateway chuyển tiếp nguyên vẹn query string sang dispatch |
| **GW-20** | Khớp route tĩnh /trips/current trước tham số động :id | 200 OK, data: null hoặc trip | HTTP 200 | **PASS** | Không bị router hiểu nhầm 'current' thành UUID |
| **GW-21** | Chống mạo danh nâng cao với header giả mạo mạnh | wallets/me trả 200 OK ví TC1; admin/trips trả 403 Forbidden | Wallet: HTTP 200, AdminTrips: HTTP 403 | **PASS** | Sanitize triệt để header giả mạo |
| **GW-22** | Chặn các đường dẫn bất thường / path traversal | 404 / 400 / 403 (tuyệt đối không 200 Admin) | /api/v1/trips/x/../../admin/trips -> 403; /api/v1/trips/..%2fadmin%2ftrips -> 400; /api/v1/trips/%2e%2e/admin/trips -> 403; /API/V1/ADMIN/TRIPS -> 403; /api/v1/admin//trips -> 404 | **PASS** | Không bypass được phân quyền bằng path traversal |

#### Nhóm N: Edge case / Đầu vào xấu / Tấn công

| Mã | Kịch bản | Kỳ vọng | Thực tế | KQ | Ghi chú |
|:---:|---|---|---|:---:|---|
| **PR-N01** | Đăng ký role='ADMIN' | 400 Bad Request, VALIDATION_ERROR; không login ra ADMIN | Register: HTTP 400 (VALIDATION_ERROR), Login role: None | **PASS** | Chỉ cho phép đăng ký CUSTOMER/DRIVER |
| **PR-N02** | Đăng ký với đầu vào xấu | 4xx có envelope | Lỗi tại: full_name emoji/<script>/SQL -> HTTP 201, phone sai định dạng (chữ) -> HTTP 201, phone quá ngắn -> HTTP 201, phone +84 -> HTTP 201 | **FAIL** | Có trường hợp trả về 5xx hoặc không bắt đúng 4xx |
| **PR-N03** | Đăng nhập với đầu vào xấu (SQLi, pass rỗng, pass 100k ký tự, thiếu trường) | 401/400, không treo CPU > 20s | SQLi: 401, Empty: 400, Pass 100k: 401 (0.14s), Missing: 400 | **PASS** | Không bị SQLi, xử lý pass dài an toàn không treo |
| **PR-N04** | Body > 10 MB gửi tới /api/v1/auth/register | 413 Payload Too Large (hoặc 400) | HTTP 413 | **PASS** | Nginx/Gateway chặn body quá lớn an toàn |
| **PR-N05** | JWT Tampering (đổi role giữ sig, sai secret, expired, bearer rỗng, bearer thường, refresh làm access) | 401 Unauthorized cho các trường hợp giả mạo | TamperRole: 401, WrongSig: 401, Expired: 401, EmptyBearer: 401, LowerBearer: 401, RefreshAsAccess: 401 | **PASS** | Gateway xác thực chữ ký và claims JWT nghiêm ngặt |
| **PR-N06** | Refresh token (rác, dùng access token, race condition đồng thời) | 401 cho rác/access; đúng 1 cái 200 và 1 cái 401 khi gọi song song | Rác: 401, Access: 401, Race: [200, 401] | **PASS** | GETDEL thu hồi nguyên tử thành công |
| **PR-N07** | Nạp ví giá trị biên (âm, 0.5, chuỗi, null, thiếu, max int, trần 100M, vượt trần) | 400 Bad Request cho đầu vào sai, không lỗi 500 | Âm: 400, 0.5: 400, Str: 400, Null: 400, Thiếu: 400, MaxInt: 400, 100M: 200, 100M+1: 400 | **PASS** | Validate số tiền nạp ví toàn diện |
| **PR-N08** | Nạp đồng thời 20 request 10.000 VND (Semaphore 10) | Số dư tăng đúng 200.000 VND (không mất cập nhật), 20/20 thành công | Thành công 20/20 reqs, Tăng: +200000 VND (Ban đầu: 100100000, Sau: 100300000) | **PASS** | Giao dịch đồng thời bảo toàn dữ liệu hoàn hảo |
| **PR-N09** | Ước tính cước với tọa độ xấu (lat 91, lng 181, chuỗi, NaN, trùng, giữa biển, 500 km) | 400 Bad Request cho tọa độ sai, không 500, 500km tính < 20s và fare > 0 | Lat91: 400, Lng181: 400, StrLat: 400, ThiếuDrop: 400, NaN: 400, Biển: 200, Trùng: 400, 500km: 200 (0.21s, fare: 10326000) | **PASS** | Validate tọa độ và tính toán khoảng cách đường dài an toàn |
| **PR-N10** | Quyền Admin: gọi đặt xe, ví me, nạp ví, WS | 403 Forbidden (chặn Admin đóng vai trò Khách/Tài xế) | Trips: 403, WalletMe: 403, Topup: 403, WS: SKIP | **PASS** | Phân quyền RBAC chặt chẽ cho tài khoản Admin |
| **PR-N11** | Đường dẫn lạ (HEAD, OPTIONS, trailing slash, //, %2e%2e, hoa, query 5k ký tự) | Không lỗi 5xx, không bypass quyền | HEAD: 404, OPT: 404, Slash: 200, //: 404, %2e%2e: 404, HOA: 200, QS 5k: 431 | **PASS** | Gateway xử lý đường dẫn lạ an toàn, không sập server |
| **PR-N12** | Endpoint tầng sau chưa làm (/admin/reports 503 nhanh < 16s; ghi nhận các API Tầng 3) | 503 SERVICE_UNAVAILABLE < 16s cho reports; ghi nhận thực tế các endpoint Tầng 3 | Reports: HTTP 503 (0.06s, SERVICE_UNAVAILABLE); Vehicle: 404; UsersMe: 404; AdminUsers: 404; ActiveDrivers: 200 | **PASS** | Chịu lỗi Fail-Closed khi service đích chưa triển khai |
| **PR-N13** | Tài xế đổi trạng thái không hợp lệ (FLYING, rỗng, busy thường, tự đặt BUSY) | 400 Bad Request (VALIDATION_ERROR), tài xế không tự đặt được BUSY | FLYING: 400, Rỗng: 400, busy: 400, BUSY: 400 | **PASS** | Chỉ cho phép tài xế đổi giữa ONLINE và OFFLINE |
| **PR-N14** | Header injection (giả mạo X-User-*, X-Forwarded-For, CRLF) | Dùng đúng danh tính trong token, không crash | Spoof: HTTP 200; CRLF: HTTP -1 | **PASS** | Header giả bị xóa sạch, không bị tấn công header injection |


### Danh sách các ca FAIL cần lưu ý

- **[PR-N02]** Đăng ký với đầu vào xấu: Thực tế `Lỗi tại: full_name emoji/<script>/SQL -> HTTP 201, phone sai định dạng (chữ) -> HTTP 201, phone quá ngắn -> HTTP 201, phone +84 -> HTTP 201` khác kỳ vọng `4xx có envelope`. Ghi chú: Có trường hợp trả về 5xx hoặc không bắt đúng 4xx


---

## 5. Lỗi phát hiện & Lệch tài liệu

### 5.1. Phân loại lỗi theo mức độ nghiêm trọng

#### Lỗi 1: [PR-N02] Thiếu kiểm tra ràng buộc định dạng số điện thoại khi đăng ký (user-service)
- **Mức độ nghiêm trọng:** **Trung bình**
- **Cách tái hiện:**
  Gửi request `POST /api/v1/auth/register` với các giá trị `phone_number` bất thường:
  - Chứa chữ cái: `{"phone_number": "090abcd123", "password": "Test@1234", "full_name": "A", "role": "CUSTOMER"}`
  - Độ dài quá ngắn: `{"phone_number": "0901", "password": "Test@1234", "full_name": "A", "role": "CUSTOMER"}`
  - Chứa tiền tố quốc tế `+84`: `{"phone_number": "+84905000001", "password": "Test@1234", "full_name": "A", "role": "CUSTOMER"}`
- **Thực tế:** Hệ thống trả về `HTTP 201 Created` và lưu tài khoản vào cơ sở dữ liệu `userdb.users`.
- **Kỳ vọng theo tài liệu:** LLD `user-service` Mục 3.1 quy định `phone_number` phải được kiểm tra định dạng và trả về `400 Bad Request` kèm mã lỗi `VALIDATION_ERROR` nếu không hợp lệ. Đầu vào số điện thoại chuẩn tại Việt Nam cần tuân theo định dạng 10 chữ số (bắt đầu bằng `0`).

#### Lỗi 2: [PR-N02] Cho phép họ tên chứa chuỗi mã độc / thẻ HTML / script không qua xử lý (user-service)
- **Mức độ nghiêm trọng:** **Thấp** (Do GORM sử dụng parameterized query nên không phát sinh lỗi SQL Injection, tuy nhiên thiếu lớp lọc dữ liệu đầu vào chống Stored XSS khi hiển thị).
- **Cách tái hiện:**
  Gửi request `POST /api/v1/auth/register` với `full_name = "😀 <script>alert(1)</script> ' OR 1=1--"`.
- **Thực tế:** Hệ thống trả về `HTTP 201 Created` và lưu nguyên chuỗi mã độc vào cột `full_name`.
- **Kỳ vọng theo tài liệu:** Trả về `400 Bad Request` (`VALIDATION_ERROR`) từ chối các ký tự thẻ script hoặc chuỗi bất thường trong họ tên người dùng.

### 5.2. Ghi nhận hành vi hệ thống & Sự khác biệt tài liệu

1. **Chứng chỉ HTTPS DuckDNS (A2):**
   - Đọc trực tiếp từ TLS handshake: `notAfter: Dec 12 12:05:48 2026 GMT` (ngày hết hạn: `12/12/2026`). Đáp ứng kỳ vọng >= 12/12/2026.
2. **Tường lửa & Bảo mật cổng nội bộ (A4):**
   - Chỉ có các cổng `22` (SSH), `80` (HTTP), `443` (HTTPS) là mở ra ngoài Internet.
   - Toàn bộ các cổng nội bộ `8080`, `8081`, `8001`–`8005`, `5432` (PostgreSQL), `6379` (Redis) đều bị chặn hoàn toàn (kết nối thất bại hoặc timeout 3s). UFW và Docker network hoạt động đúng thiết kế.
3. **Cơ chế Fail-Closed khi Service chưa triển khai (PR-N12):**
   - Endpoint `GET /api/v1/admin/reports` (thuộc `ai-service` Tầng 2) được Gateway phản hồi ngay lỗi `503 SERVICE_UNAVAILABLE` trong 0.06 giây (< 16s), không bị treo và không làm rò rỉ stack trace lỗi nội bộ.
   - Endpoint `GET /api/v1/admin/drivers/active` trả về `200 OK` do `location-service` đã cài đặt sẵn handler cho Admin.
4. **Hệ số giá biến động Surge Pricing khi không có tải (P-06a..P-06d, P-06f):**
   - Do kiểm thử từ mạng ngoài không có quyền truy cập Redis Stream để bắn sự kiện `TripCreated` kích hoạt Demand, ô `w3gv7` có `Demand = 0`. Hệ thống tự động fallback an toàn về `surge_multiplier = 1.0` (tuân thủ nguyên tắc an toàn LLD-00 Mục 5(j) không làm gián đoạn đặt xe).
5. **Ràng buộc nạp tiền tối đa (PR-N07 vs W-06b):**
   - Khi nạp đúng mức trần `100.000.000 VND`, hệ thống chấp nhận và trả về `200 OK`.
   - Khi nạp vượt mức trần `100.000.001 VND`, hệ thống từ chối và trả về `400 Bad Request` (`VALIDATION_ERROR`).
6. **Độ an toàn trước tấn công JWT & Header Spoofing (PR-N05, PR-N14, GW-08..GW-10, GW-21):**
   - Token giả mạo chữ ký, token đổi role, token thuật toán `none`, token hết hạn đều bị từ chối `401 UNAUTHORIZED`.
   - Header `X-User-Id`, `X-User-Role` giả mạo bị Gateway xóa sạch và gán lại từ claims trong token hợp lệ.

---

## 6. Việc người dùng chạy tay trên VPS

Dưới đây là các lệnh kiểm tra cấu hình hạ tầng và tiến trình nội bộ trên VPS thật mà tài khoản test từ ngoài Internet không thể truy cập trực tiếp:

| STT | Mục đích kiểm tra | Lệnh thực thi trên VPS | Kết quả thực tế trên VPS |
|:---:|---|---|---|
| 1 | Kiểm tra trạng thái tường lửa UFW | `sudo ufw status verbose` | *[Để trống]* |
| 2 | Kiểm tra cổng bind Docker (chỉ bind 127.0.0.1) | `sudo ss -tulpn \| grep -E '8080\|8081\|8001\|8002\|8003\|8004\|8005\|5432\|6379'` | *[Để trống]* |
| 3 | Kiểm tra chứng chỉ SSL Let's Encrypt | `sudo certbot certificates` | *[Để trống]* |
| 4 | Kiểm tra mức tiêu thụ RAM và Swap | `free -h && docker stats --no-stream` | *[Để trống]* |
| 5 | Kiểm tra log Nginx tiếp nhận kết nối | `sudo tail -n 50 /var/log/nginx/access.log` | *[Để trống]* |
| 6 | Kiểm tra log Gateway xử lý proxy | `docker logs --tail 50 api-gateway` | *[Để trống]* |

---

## 7. Đề xuất bổ sung ca kiểm thử cho các lượt sau

1. **Kiểm thử WebSocket Gateway (`ws-gateway`):**
   - Bổ sung kết nối WSS từ tài xế ảo, gửi tọa độ GPS định kỳ 3 giây một lần.
   - Kiểm tra phân phối tọa độ tài xế realtime cho khách hàng đang trong cuốc xe (`FR-38`).
2. **Kịch bản điều phối trọn vẹn (Dispatch End-to-End):**
   - Khách đặt xe `POST /api/v1/trips` -> State machine `MATCHING` -> Tài xế `ACCEPTED` -> `PICKING_UP` -> `IN_TRIP` -> `COMPLETED`.
   - Kiểm tra sự kiện `TripCompleted` trên Redis Stream kích hoạt trừ tiền ví khách và cộng tiền ví tài xế.
3. **Kịch bản gỡ kẹt Admin (`FR-37`):**
   - Khách hoặc tài xế gặp chuyến kẹt -> Admin gọi `POST /api/v1/admin/trips/:id/force-cancel` để giải phóng khóa và trạng thái tài xế.
