# SỐ LIỆU ĐO KIỂM THỰC TẾ (BENCHMARK & RESOURCE USAGE)

> **Mã tài liệu:** `DEP-01` | **Phiên bản:** `1.2` | **Cập nhật:** 11/10/2026

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

### Kết quả `docker stats --no-stream` (Sau tái cấu trúc phân lớp)

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `user-service` | 0.05% | 7.04 MiB | 30 MiB | 23.48% | 43 kB / 27.7 kB | 0 B / 0 B | 12 | Healthy |
| `postgres` | 0.00% | 38.82 MiB | 200 MiB | 19.41% | 44.1 kB / 25.3 kB | 31.7 MB / 130 MB | 7 | Healthy |
| `redis` | 0.84% | 5.93 MiB | 60 MiB | 9.88% | 16.8 kB / 5.46 kB | 5.42 MB / 65.5 kB | 7 | Healthy |
| **Tổng cộng** | - | **51.79 MiB** | **290 MiB** | - | - | - | 26 | - |

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (30 MB):** Sau khi tái cấu trúc sang kiến trúc phân lớp (Clean Architecture / Layered), `user-service` hoạt động ổn định ở mức ~7.04 MiB (~23.5% định mức 30 MB).
- **Ghi chú tải trọng:** Đây là mức RAM lúc nhàn rỗi (idle); `mem_limit: 30MB` sẽ được đánh giá lại khi chạy kịch bản tải 100 tài xế ảo ở Lượt 10 vì thao tác băm mật khẩu `bcrypt` khi đăng nhập dồn dập có thể gây tăng đột biến CPU & RAM spike.
- **Cấu hình `GOMEMLIMIT` (26MiB):** Hoạt động hiệu quả, kích hoạt dọn rác sớm giúp duy trì mức RAM ổn định. Giữ nguyên mức 30 MB hiện tại.
- **Ngân sách RAM toàn hệ thống:** 51.79 MiB / 850 MiB (~6.1% ngân sách tối đa của VPS 1 GB).

---

## 3. Triển khai `location-service` (Lượt 3)

Đo kiểm sau khi chạy toàn bộ chuỗi kiểm thử nghiệp vụ (record 5 tài xế, quét candidates quanh P1, truy vấn geohash-drivers `w3gv7`, admin active drivers, kiểm tra TTL 15s và cơ chế ZREM lười 30s).  
*Ghi chú môi trường:* Số đo thực tế dưới đây được lấy từ Docker Desktop trên máy dev nội bộ, không phải trên máy chủ VPS production.

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `location-service` | 0.08% | 4.51 MiB | 25 MiB | 18.03% | 35.3 kB / 36.8 kB | 4.1 kB / 0 B | 10 | Healthy |
| `user-service` | 0.01% | 17.44 MiB | 30 MiB | 58.13% | 22.2 kB / 21.7 kB | 68.5 MB / 4.47 MB | 8 | Healthy |
| `postgres` | 0.00% | 39.15 MiB | 200 MiB | 19.57% | 22.6 kB / 14.7 kB | 40.5 MB / 471 kB | 7 | Healthy |
| `redis` | 0.35% | 8.63 MiB | 60 MiB | 14.38% | 23.7 kB / 12.6 kB | 31.1 MB / 90.1 kB | 6 | Healthy |
| **Tổng cộng** | - | **69.73 MiB** | **315 MiB** | - | - | - | 31 | - |

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (25 MB):** `location-service` chỉ tiêu thụ ~4.51 MiB RAM (~18% định mức), bộ nhớ rất nhẹ do không có DB quan hệ (chỉ thao tác trực tiếp với Redis GEO). Mức `mem_limit: 25m` và `GOMEMLIMIT: 22MiB` hoàn toàn an toàn và đáp ứng tốt tiêu chuẩn demo.
- **user-service:** 17.44 MiB (58% của 30m), tăng từ 7.04 MiB; cần xem lại ở Lượt 10 vì bcrypt đăng nhập dồn có thể chạm trần.
- **Ngân sách RAM toàn hệ thống:** 69.73 MiB / 850 MiB (~8.2% ngân sách tối đa của VPS 1 GB).

---

## 4. Nhật ký gỡ lỗi (Debugging Log)

- **Lỗi escape ký tự JSON trong PowerShell khi gọi `curlimages/curl`:** Khi chạy câu lệnh PowerShell với cờ `-d '{...}'`, PowerShell tự động bóc tách dấu nháy kép khiến parser JSON của Go nhận payload bị lỗi cú pháp `VALIDATION_ERROR`.  
  *Cách khắc phục:* Chuyển sang kỹ thuật truyền dữ liệu qua stdin (`<json> | docker run -i ... curlimages/curl --data-binary "@-"`) giúp payload JSON được bảo toàn nguyên vẹn 100%. (Lưu ý cờ `@-` phải đặt trong dấu nháy kép `"@-"` trên PowerShell để không bị lỗi token).
- **Lỗi tràn dung lượng ổ đĩa C trong quá trình Docker build (`There is not enough space on the disk`):** Ổ C máy host cạn kiệt dung lượng (chỉ còn ~100 MB free) khiến tiến trình biên dịch Go/BuildKit thất bại và ngắt kết nối daemon WSL2 (`unexpected EOF`).  
  *Cách khắc phục:* Chạy `go clean -cache -modcache` và dọn sạch thư mục tạm `%TEMP%`, giải phóng thành công ~2 GB trên ổ C, khởi động lại Docker Desktop ổn định.
