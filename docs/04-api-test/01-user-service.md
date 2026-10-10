# KỊCH BẢN KIỂM THỬ DỊCH VỤ TÀI KHOẢN (USER-SERVICE TEST SUITE)

> **Mã tài liệu:** `TEST-USER` | **Phiên bản:** `1.0`  
> **Dịch vụ kiểm thử:** `user-service` (Port nội bộ: `8001`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD user-service](../03-lld/user-service.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

Bộ kịch bản kiểm thử bao gồm 15 ca kiểm thử (`U-01` đến `U-15`) bao phủ toàn diện các chức năng nghiệp vụ, phân quyền, xử lý ngoại lệ và API nội bộ của `user-service` theo thiết kế LLD.  
Mọi kịch bản làm thay đổi trạng thái đều có bước dọn dẹp (cleanup) để đảm bảo tính Idempotent (chạy lại nhiều lần mà không bị lỗi phụ thuộc trạng thái).

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Method & Path | Headers | Body / Tham số | Kỳ vọng HTTP Status & Mã lỗi | Ghi chú & Bước dọn dẹp |
|:---:|---|---|---|---|---|---|
| **U-01** | Đăng ký Khách hàng C1 hợp lệ | `POST /api/v1/auth/register` | `Content-Type: application/json` | `{"phone_number": "0901000001", "email": "customer1@demo.local", "password": "Test@1234", "full_name": "Nguyen Van An", "role": "CUSTOMER"}` | **201 Created**<br>*(hoặc 409 nếu chạy lại)* | Trả về `user_id`, `role: "CUSTOMER"`. Chấp nhận 409 khi chạy lại kịch bản. |
| **U-02** | Đăng ký trùng số điện thoại C1 | `POST /api/v1/auth/register` | `Content-Type: application/json` | `{"phone_number": "0901000001", "email": "other@demo.local", "password": "Test@1234", "full_name": "Nguyen Khach Khac", "role": "CUSTOMER"}` | **409 Conflict**<br>`USER_ALREADY_EXISTS` | Thông điệp: *"Số điện thoại hoặc email đã tồn tại trong hệ thống"*. |
| **U-03** | Đăng ký mật khẩu quá ngắn (< 6 ký tự) | `POST /api/v1/auth/register` | `Content-Type: application/json` | `{"phone_number": "0901000099", "password": "123", "full_name": "Test Short Pass", "role": "CUSTOMER"}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Thông điệp: *"Mật khẩu phải từ 6 ký tự trở lên"*. |
| **U-04** | Đăng nhập đúng thông tin C1 | `POST /api/v1/auth/login` | `Content-Type: application/json` | `{"phone_or_email": "0901000001", "password": "Test@1234"}` | **200 OK** | Trả về `access_token`, `refresh_token`, `expires_in: 3600`, `user.role: "CUSTOMER"`. |
| **U-05** | Đăng nhập sai mật khẩu C1 | `POST /api/v1/auth/login` | `Content-Type: application/json` | `{"phone_or_email": "0901000001", "password": "WrongPassword@123"}` | **401 Unauthorized**<br>`UNAUTHORIZED` | Thông điệp: *"Số điện thoại/email hoặc mật khẩu không chính xác"*. |
| **U-06** | Đăng nhập tài khoản không tồn tại | `POST /api/v1/auth/login` | `Content-Type: application/json` | `{"phone_or_email": "0999999999", "password": "WrongPassword@123"}` | **401 Unauthorized**<br>`UNAUTHORIZED` | Chống timing attack: Chạy bcrypt giả cost 10, cùng thông điệp như U-05. |
| **U-07** | Xoay vòng Refresh Token lần 1 (Hợp lệ) | `POST /api/v1/auth/refresh` | `Content-Type: application/json` | `{"refresh_token": "{{c1_refresh_token}}"}` | **200 OK** | Cấp cặp token mới, thu hồi token cũ nguyên tử qua `GETDEL`. |
| **U-08** | Tái sử dụng Refresh Token lần 2 (Đã dùng) | `POST /api/v1/auth/refresh` | `Content-Type: application/json` | `{"refresh_token": "{{c1_refresh_token_old}}"}` | **401 Unauthorized**<br>`INVALID_REFRESH_TOKEN` | Thông điệp: *"Refresh token không tồn tại hoặc đã được sử dụng"*. |
| **U-09** | Tài xế D1 bật ONLINE | `PATCH /api/v1/driver/status` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER`<br>`Content-Type: application/json` | `{"status": "ONLINE"}` | **200 OK** | Trả về `driver_id`, `status: "ONLINE"`. |
| **U-10** | Khách hàng C1 cố đổi trạng thái tài xế | `PATCH /api/v1/driver/status` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER`<br>`Content-Type: application/json` | `{"status": "ONLINE"}` | **403 Forbidden**<br>`FORBIDDEN` | Middleware chặn: *"Chỉ tài xế mới có quyền cập nhật trạng thái"*. |
| **U-11a**| Nội bộ đổi D1 từ ONLINE $\rightarrow$ BUSY lần 1 | `POST /internal/v1/drivers/{{d1_id}}/status` | `Content-Type: application/json` | `{"from_status": "ONLINE", "to_status": "BUSY"}` | **200 OK** | Trả về `driver_id`, `status: "BUSY"`. |
| **U-11b**| Nội bộ đổi D1 từ ONLINE $\rightarrow$ BUSY lần 2 (Đã BUSY) | `POST /internal/v1/drivers/{{d1_id}}/status` | `Content-Type: application/json` | `{"from_status": "ONLINE", "to_status": "BUSY"}` | **400 Bad Request**<br>`DRIVER_NOT_AVAILABLE` | Thông điệp: *"Trạng thái hiện tại khác from_status hoặc không tìm thấy tài xế"*. |
| **U-12** | Tài xế D1 đang BUSY cố chuyển OFFLINE | `PATCH /api/v1/driver/status` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER`<br>`Content-Type: application/json` | `{"status": "OFFLINE"}` | **400 Bad Request**<br>`DRIVER_BUSY` | Chặn tài xế bỏ cuốc: *"Tài xế đang bận chuyến xe, không thể chuyển đổi trạng thái"*.<br>**Bước dọn:** Gọi nội bộ đổi D1 từ `BUSY` về `ONLINE`. |
| **U-13a**| Lọc tài xế ONLINE với mảng rỗng `[]` | `POST /internal/v1/users/filter-online` | `Content-Type: application/json` | `{"driver_ids": []}` | **200 OK** | Trả về `{"online_driver_ids": []}`. |
| **U-13b**| Lọc tài xế ONLINE với phần tử sai UUID | `POST /internal/v1/users/filter-online` | `Content-Type: application/json` | `{"driver_ids": ["not-a-valid-uuid"]}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Bắt lỗi chuỗi sai cú pháp: *"Phần tử không phải UUID hợp lệ: not-a-valid-uuid"*. |
| **U-13c**| Lọc tài xế ONLINE với mảng > 500 phần tử | `POST /internal/v1/users/filter-online` | `Content-Type: application/json` | `{"driver_ids": [501 phần tử UUID]}` | **400 Bad Request**<br>`VALIDATION_ERROR` | Thông điệp: *"Mảng driver_ids vượt quá giới hạn tối đa 500 phần tử"*. |
| **U-13d**| Lọc tài xế ONLINE trả đúng ID | `POST /internal/v1/users/filter-online` | `Content-Type: application/json` | `{"driver_ids": ["{{d1_id}}"]}` | **200 OK** | D1 đang `ONLINE` $\rightarrow$ Trả về `{"online_driver_ids": ["{{d1_id}}"]}`. |
| **U-14a**| Đăng xuất khi tài xế đang BUSY | `POST /api/v1/auth/logout` | `X-User-Id: {{d1_id}}`<br>`X-User-Role: DRIVER`<br>`Authorization: Bearer {{d1_token}}`<br>`Content-Type: application/json` | `{"refresh_token": "{{d1_refresh_token}}"}` | **400 Bad Request**<br>`DRIVER_CANNOT_LOGOUT` | D1 chuyển sang `BUSY` trước khi gọi logout: *"Tài xế đang trong chuyến đi, không thể đăng xuất"*.<br>**Bước dọn:** Đưa D1 về `OFFLINE`. |
| **U-14b**| Đăng xuất bằng Refresh Token của người khác | `POST /api/v1/auth/logout` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER`<br>`Authorization: Bearer {{c1_token}}`<br>`Content-Type: application/json` | `{"refresh_token": "{{d1_refresh_token}}"}` | **401 Unauthorized**<br>`INVALID_REFRESH_TOKEN` | Token không chính chủ: Chặn và không làm thay đổi trạng thái. |
| **U-14c**| Khách hàng C1 đăng xuất hợp lệ | `POST /api/v1/auth/logout` | `X-User-Id: {{c1_id}}`<br>`X-User-Role: CUSTOMER`<br>`Authorization: Bearer {{c1_token}}`<br>`Content-Type: application/json` | `{"refresh_token": "{{c1_refresh_token}}"}` | **200 OK** | Thông báo: *"Đăng xuất thành công"*. Redis tạo `blacklist:<jti>` và phát kênh `ride:ws_control` DISCONNECT. |
| **U-15** | Đăng nhập tài khoản Quản trị viên (Admin) | `POST /api/v1/auth/login` | `Content-Type: application/json` | `{"phone_or_email": "admin@ridehailing.local", "password": "Admin@123456"}` | **200 OK** | Trả về JWT claim `role: "ADMIN"`, `user.driver_status: null`. |

---

## 3. BƯỚC DỌN DẸP TRẠNG THÁI (TEARDOWN / CLEANUP)

Sau khi hoàn tất toàn bộ kịch bản, để bảo đảm D1 sẵn sàng cho các lượt kiểm thử kế tiếp (như `location-service` và `dispatch-service`):
1. Chuyển trạng thái D1 về `ONLINE` hoặc `OFFLINE` (không để kẹt ở `BUSY`).
2. Nếu các token bị thu hồi bởi logout, chạy lại folder `00 Seed` trên Postman để tái cấp phát cặp token mới.
