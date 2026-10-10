# HƯỚNG DẪN KIỂM THỬ API HỆ THỐNG RIDE-HAILING

> **Mã tài liệu:** `TEST-GUIDE` | **Phiên bản:** `1.0`  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. KHỞI ĐỘNG HỆ THỐNG TRÊN MÁY DEV

Mặc định trên VPS, các microservice chỉ bind trong mạng nội bộ Docker `backend-net` và không publish cổng ra ngoài. Để kiểm thử API trực tiếp từ máy dev bằng Postman hoặc cURL, sử dụng file override `docker-compose.dev.yml`:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

> [!WARNING]
> File `docker-compose.dev.yml` **CHỈ DÙNG TRÊN MÁY DEV**, tuyệt đối không đưa lên VPS. Pipeline CI/CD triển khai chỉ scp `docker-compose.yml` và thư mục `infra/`.

---

## 2. HƯỚNG DẪN SỬ DỤNG POSTMAN

### 2.1. Import Collection & Environment
1. Mở Postman $\rightarrow$ Chọn **Import**.
2. Chọn 2 file trong thư mục `docs/04-api-test/postman/`:
   - Collection: `ride-hailing.postman_collection.json` (Chuẩn v2.1)
   - Environment: `local.postman_environment.json`
3. Ở góc trên bên phải Postman, chọn Active Environment là **Ride-Hailing Local Dev**.

### 2.2. Trình tự chạy kiểm thử (Execution Flow)
- **Bước 1: Chạy folder `00 Seed`**
  - Đăng ký và đăng nhập tự động tài khoản Quản trị viên (`Admin`), Khách hàng (`C1`–`C3`) và Tài xế (`D1`–`D5`).
  - Test script tự động lưu các biến phiên `{{admin_token}}`, `{{c1_id}}`, `{{c1_token}}`, `{{c1_refresh}}`, `{{d1_id}}`, `{{d1_token}}`, `{{d1_refresh}}`... vào collection variables.
  - Yêu cầu đăng ký trong folder Seed chấp nhận cả status `201 Created` và `409 Conflict` (khi dữ liệu đã tồn tại sẵn từ lần chạy trước).
- **Bước 2: Chạy folder kiểm thử của từng service**
  - Chạy folder `01 user-service` để kiểm thử toàn diện 15 kịch bản U-01..U-15.
  - Các service tiếp theo (`location`, `pricing`, `payment`, `dispatch`) sẽ được bổ sung vào collection theo từng lượt triển khai.

---

## 3. CÁCH RESET DỮ LIỆU MÔI TRƯỜNG TEST

Khi cần xóa sạch toàn bộ cơ sở dữ liệu PostgreSQL và Redis để chạy lại từ đầu:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml down -v
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

---

## 4. QUY ƯỚC HEADER TRƯỚC VÀ SAU KHI CÓ API-GATEWAY

- **Giai đoạn trước Lượt 8 (chưa có `api-gateway`):**
  - Gọi **THẲNG** vào port publish dev của từng service (ví dụ `user-service` tại `http://127.0.0.1:8001`).
  - Phải truyền kèm header nội bộ `X-User-Id: {{user_id}}` và `X-User-Role: {{user_role}}` (được cấu hình sẵn qua biến trong Postman collection).
- **Giai đoạn từ Lượt 8 trở đi (đã có `api-gateway` tại cổng `8080`):**
  - Đổi biến môi trường `gateway_url = http://127.0.0.1:8080`.
  - Mọi request công khai đi qua Gateway; client chỉ cần gửi header chuẩn `Authorization: Bearer {{token}}`. Gateway tự động giải mã JWT, bóc tách và gắn `X-User-Id` / `X-User-Role` khi chuyển tiếp nội bộ.

---

## 5. THAM CHIẾU DỮ LIỆU MẪU

Tra cứu toàn bộ thông tin tài khoản demo, số điện thoại, mật khẩu chung `Test@1234`, tọa độ P1/P2 và vị trí tài xế D1–D5 tại tài liệu:  
👉 **[00-du-lieu-mau.md](00-du-lieu-mau.md)**
