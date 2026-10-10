# SỐ LIỆU ĐO KIỂM THỰC TẾ (BENCHMARK & RESOURCE USAGE)

> **Mã tài liệu:** `DEP-01` | **Phiên bản:** `1.1` | **Cập nhật:** 10/10/2026

---

## 1. Tầng Hạ tầng cơ bản (PostgreSQL + Redis)

Đo kiểm khi khởi chạy 2 container hạ tầng ở trạng thái rảnh rỗi (idle), đã khởi tạo xong 5 database và xác thực kết nối.

### Kết quả `docker stats --no-stream` (Lượt 0b)

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `postgres` | 0.01% | 36.82 MiB | 200 MiB | 18.41% | 1.82 kB / 126 B | 8.19 kB / 119 MB | 6 | Healthy |
| `redis` | 0.60% | 5.53 MiB | 60 MiB | 9.22% | 1.82 kB / 126 B | 0 B / 8.19 kB | 6 | Healthy |
| **Tổng cộng** | - | **42.35 MiB** | **260 MiB** | - | - | - | 12 | - |

---

## 2. Triển khai `user-service` (Lượt 1)

Đo kiểm sau khi chạy toàn bộ chuỗi kiểm thử nghiệp vụ (register, login, refresh, update status, filter-online, logout, restart container).

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `user-service` | 1.89% | 6.47 MiB | 30 MiB | 21.58% | 14.6 kB / 17.6 kB | 0 B / 0 B | 12 | Healthy |
| `postgres` | 2.33% | 28.51 MiB | 200 MiB | 14.25% | 41 kB / 24.5 kB | 3.74 MB / 1.9 MB | 7 | Healthy |
| `redis` | 0.71% | 14.23 MiB | 60 MiB | 23.72% | 14.2 kB / 5.19 kB | 8.76 MB / 41 kB | 7 | Healthy |
| **Tổng cộng** | - | **49.21 MiB** | **290 MiB** | - | - | - | 26 | - |

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (30 MB):** Hoàn toàn đủ và an toàn. `user-service` chỉ tiêu thụ ~6.47 MiB (~21.6% định mức 30 MB) kể cả sau các thao tác băm mật khẩu `bcrypt` và sinh/giải mã token JWT.
- **Cấu hình `GOMEMLIMIT` (26MiB):** Hoạt động hiệu quả, kích hoạt dọn rác sớm giúp duy trì mức RAM ổn định. Giữ nguyên mức 30 MB hiện tại.
- **Ngân sách RAM toàn hệ thống:** 49.21 MiB / 850 MiB (~5.8% ngân sách tối đa của VPS 1 GB).

---

## 3. Nhật ký gỡ lỗi (Debugging Log)

- **Lỗi escape ký tự JSON trong PowerShell khi gọi `curlimages/curl`:** Khi chạy câu lệnh PowerShell với cờ `-d '{...}'`, PowerShell tự động bóc tách dấu nháy kép khiến parser JSON của Go nhận payload bị lỗi cú pháp `VALIDATION_ERROR`.  
  *Cách khắc phục:* Chuyển sang kỹ thuật truyền dữ liệu qua stdin (`<json> | docker run -i ... curlimages/curl --data-binary "@-"`) giúp payload JSON được bảo toàn nguyên vẹn 100%.
