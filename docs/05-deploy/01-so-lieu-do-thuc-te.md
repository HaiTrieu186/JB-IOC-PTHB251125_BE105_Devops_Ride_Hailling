# SỐ LIỆU ĐO KIỂM THỰC TẾ (BENCHMARK & RESOURCE USAGE)

> **Mã tài liệu:** `DEP-01` | **Phiên bản:** `1.3` | **Cập nhật:** 11/10/2026

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

## 4. Triển khai `pricing-service` (Lượt 4)

Đo kiểm sau khi chạy toàn bộ chuỗi kiểm thử nghiệp vụ (ước tính cước lộ trình $P1 \rightarrow P2$, GET/PUT cấu hình bảng giá Admin, kiểm tra RBAC 403, kiểm tra các ca lỗi 400/401, chuỗi kiểm thử Surge Pricing với $T=1.2, 1.4, 5.0, 0.05$, Supply=0, Demand=0, và fallback khi location-service tắt).  
*Ghi chú môi trường:* Số đo thực tế dưới đây được lấy từ Docker Desktop trên máy dev nội bộ sau 2 lượt chạy full test suite.

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `pricing-service` | 0.03% | 10.11 MiB | 30 MiB | 33.71% | 139 kB / 159 kB | 0 B / 0 B | 13 | Healthy |
| `location-service` | 0.05% | 4.35 MiB | 25 MiB | 17.39% | 28.9 kB / 32.8 kB | 0 B / 0 B | 8 | Healthy |
| `user-service` | 0.03% | 19.23 MiB | 30 MiB | 64.09% | 68.8 kB / 62.1 kB | 167 MB / 3.42 MB | 15 | Healthy |
| `postgres` | 0.00% | 51.34 MiB | 200 MiB | 25.67% | 113 kB / 78.4 kB | 49.1 MB / 1.68 MB | 8 | Healthy |
| `redis` | 2.41% | 16.00 MiB | 60 MiB | 26.67% | 278 kB / 185 kB | 34.0 MB / 246 kB | 6 | Healthy |
| **Tổng cộng** | - | **101.03 MiB** | **345 MiB** | - | - | - | 50 | - |

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (30 MB):** `pricing-service` tiêu thụ ~10.11 MiB RAM (~33.7% định mức 30 MB), hoạt động ổn định bao gồm cả goroutine background tiêu thụ Redis Streams PEL. Định mức `mem_limit: 30m` và `GOMEMLIMIT: 26MiB` là hoàn toàn đủ, an toàn chống OOM và kích hoạt GC đúng lúc.
- **user-service:** 19.23 MiB (64% của 30m), tăng dần qua các lần đo 7 -> 17 -> 19 MiB (Docker Desktop, máy dev); cần xem lại ở Lượt 10 vì bcrypt đăng nhập dồn có thể chạm trần.
- **Đo kiểm độ trễ OSRM thực tế từ máy dev:** Gọi trực tiếp `https://router.project-osrm.org` qua cURL cho kết quả thời gian phản hồi từ ~440 ms đến ~630 ms (thường xuyên vượt ngưỡng 400 ms do độ trễ đường truyền quốc tế). Khi timeout $> 400\text{ ms}$, cơ chế Fallback Haversine $\times 1.35$ kích hoạt mượt mà, trả HTTP 200 kèm `source = "HAVERSINE"`. Khi mạng nhanh $< 400\text{ ms}$, service trả kết quả lộ trình thực tế từ OSRM với `source = "OSRM"`.
- **Đo kiểm Redis Streams XPENDING delivery count:** Lệnh `XREADGROUP ... 0` đọc lại tin nhắn tồn đọng trong PEL không tự động tăng `DeliveredTimes` của Redis Stream (giá trị giữ nguyên 1). Đúng theo quy định Mục 5(m) LLD-00, `pricing-service` sử dụng bộ đếm trong RAM `map[msg_id]int` (kèm Mutex lock) để theo dõi số lần thử lại; khi vượt quá `MAX_DELIVERY_ATTEMPTS` (5 lần) thì ghi log `ERROR` và tự động `XACK` giải phóng message kẹt.
- **Ngân sách RAM toàn hệ thống:** 101.03 MiB / 850 MiB (~11.9% ngân sách tối đa của VPS 1 GB).

---

## 5. Triển khai `payment-service` (Lượt 6)

Đo kiểm sau khi chạy toàn bộ chuỗi kiểm thử nghiệp vụ (Top-up ví khách C1 200.000 VND, Get balance/transactions, Check-balance C1/C3, XADD TripCompleted phân chia cước 50.000 VND, kiểm tra Idempotency chống trừ trùng, xử lý thiếu tiền FAILED trên C3, bảo mật RBAC chặn ADMIN 403, khôi phục tồn đọng PEL sau khi phục hồi DB PostgreSQL).  
*Ghi chú môi trường:* Số đo thực tế dưới đây được lấy từ Docker Desktop trên máy dev nội bộ sau khi hoàn thành 2 lượt chạy test suite Postman (39/39 assertions PASS).

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `payment-service` | 0.04% | 7.28 MiB | 30 MiB | 24.27% | 106 kB / 130 kB | 8.19 kB / 0 B | 11 | Healthy |
| `pricing-service` | 1.87% | 8.69 MiB | 30 MiB | 28.96% | 622 kB / 847 kB | 4.10 kB / 0 B | 12 | Healthy |
| `location-service` | 1.88% | 8.22 MiB | 25 MiB | 32.89% | 71.8 kB / 77.4 kB | 3.37 MB / 0 B | 13 | Healthy |
| `user-service` | 1.97% | 18.65 MiB | 30 MiB | 62.16% | 132 kB / 124 kB | 273 MB / 5.96 MB | 15 | Healthy |
| `postgres` | 0.00% | 19.23 MiB | 200 MiB | 9.62% | 14.1 kB / 11.4 kB | 9.88 MB / 782 kB | 7 | Healthy |
| `redis` | 2.42% | 16.12 MiB | 60 MiB | 26.88% | 1.52 MB / 1.06 MB | 34.9 MB / 418 kB | 6 | Healthy |
| **Tổng cộng** | - | **78.19 MiB** | **375 MiB** | - | - | - | 64 | - |

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (30 MB):** `payment-service` tiêu thụ ~7.28 MiB RAM (~24.3% định mức 30 MB), mức sử dụng rất thấp và ổn định dù có chạy ngầm consumer lắng nghe Redis Streams và thực hiện các transaction khóa dòng database PostgreSQL. Mức `mem_limit: 30m` và `GOMEMLIMIT: 26MiB` hoàn toàn an toàn và tối ưu cho VPS 1 GB.
- **Xử lý tránh Deadlock khi khóa 2 ví đồng thời:** Luồng quyết toán `TripCompleted` khóa cả ví khách và ví tài xế bằng `SELECT ... FOR UPDATE`. Để tránh hiện tượng Deadlock khi có 2 chuyến đi giữa 2 người dùng diễn ra đồng thời theo chiều ngược lại, hệ thống luôn sắp xếp khóa ví theo thứ tự UUID tăng dần (`firstID < secondID`). Thao tác này loại trừ triệt để nguy cơ Circular Wait ở tầng cơ sở dữ liệu.
- **Đo kiểm phục hồi PEL (Consumer Group `payment-group`):** Khi PostgreSQL tạm dừng, tin nhắn `TripCompleted` không thể ghi nhận và nằm lại trong PEL với `XPENDING = 1`. Ngay khi PostgreSQL hoạt động trở lại, background worker quét lại PEL định kỳ 3 giây, hoàn tất ghi 3 dòng sổ quyết toán thành công và tự động gọi `XACK`, đưa `XPENDING` về 0 mà không làm thất thoát giao dịch.
- **Ngân sách RAM toàn hệ thống:** 78.19 MiB / 850 MiB (~9.2% ngân sách tối đa của VPS 1 GB).

---

## 6. Triển khai `dispatch-service` (Lượt 7)

Đo kiểm sau khi chạy toàn bộ chuỗi 15 kịch bản kiểm thử nghiệp vụ (luồng đầy đủ C1 + D1 tạo chuyến -> matching -> accept -> picking-up -> start-trip -> complete -> quyết toán ví khách/tài xế, 2 tài xế tranh cuốc đồng thời với Redis SET NX, ví không đủ INSUFFICIENT_BALANCE, đặt trùng ACTIVE_TRIP_EXISTS, 0 tài xế EXPIRED, hết 30s TIMEOUT_30S với background sweeper worker 2s, các nhánh hủy của khách/tài xế, admin force-cancel và phân trang, accept idempotent, RBAC và validation dải tọa độ/phân trang).  
*Ghi chú môi trường:* Số đo thực tế dưới đây được lấy từ Docker Desktop trên máy dev nội bộ sau khi hoàn thành chuỗi kiểm thử tự động.

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `dispatch-service` | 0.08% | 8.58 MiB | 30 MiB | 28.59% | 169 kB / 260 kB | 0 B / 0 B | 13 | Healthy |
| `payment-service` | 1.91% | 9.19 MiB | 30 MiB | 30.63% | 473 kB / 681 kB | 2.42 MB / 0 B | 11 | Healthy |
| `pricing-service` | 0.07% | 15.54 MiB | 30 MiB | 51.80% | 752 kB / 1.05 MB | 332 MB / 51.3 MB | 17 | Healthy |
| `location-service` | 0.04% | 12.86 MiB | 25 MiB | 51.45% | 110 kB / 114 kB | 21.5 MB / 0 B | 16 | Healthy |
| `user-service` | 0.01% | 16.07 MiB | 30 MiB | 53.57% | 91.5 kB / 102 kB | 338 MB / 8.34 MB | 17 | Healthy |
| `postgres` | 0.03% | 53.81 MiB | 200 MiB | 26.91% | 303 kB / 208 kB | 49.9 MB / 3.73 MB | 11 | Healthy |
| `redis` | 0.51% | 9.88 MiB | 60 MiB | 16.47% | 1.31 MB / 860 kB | 5.55 MB / 197 kB | 6 | Healthy |
| **Tổng cộng** | - | **125.93 MiB** | **405 MiB** | - | - | - | 91 | - |

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (30 MB):** `dispatch-service` tiêu thụ ~8.58 MiB RAM (~28.6% định mức 30 MB), mức tiêu thụ rất thấp dù tích hợp đồng thời kết nối PostgreSQL connection pool (10 open, 3 idle), Redis client, background sweeper ticker 2s/lần quét quá hạn, và HTTP client gọi 4 microservice phụ thuộc. Định mức `mem_limit: 30m` và `GOMEMLIMIT: 26MiB` hoàn toàn đủ và tối ưu cho VPS 1 GB.
- **Xác minh Distributed Lock & Hoàn tác:** Khóa phân tán `ride:lock:<trip_id>` kết hợp Lua Script `compare-and-delete` ngăn chặn thành công hiện tượng race condition khi 2 tài xế cùng bấm accept; cơ chế rollback BUSY -> ONLINE retry 3 lần đảm bảo không làm kẹt trạng thái tài xế khi nhận cuốc thất bại.
- **Xác minh Worker quét quá hạn:** Background worker quét định kỳ mỗi 2 giây giải phóng chính xác các chuyến quá hạn 30 giây sang `EXPIRED`, ghi log timeline `TIMEOUT_30S` và phát sự kiện `TripExpired` ra Redis Stream.
- **Ngân sách RAM toàn hệ thống:** 125.93 MiB / 850 MiB (~14.8% ngân sách tối đa của VPS 1 GB).

---

## 7. Triển khai `api-gateway` (Lượt 8)

Đo kiểm sau khi chạy toàn bộ chuỗi kiểm thử nghiệp vụ (luồng demo end-to-end qua cổng 8080 C1 + D1 đăng nhập -> nạp tiền ví C1 -> D1 online -> estimate -> create trip -> accept -> picking-up -> start-trip -> complete -> tra cứu ví C1/D1, kiểm tra header spoofing sanitize, từ chối token alg none / sai secret / hết hạn, chặn API nội bộ /internal/* và method sai về 404, kiểm tra RBAC Admin 403, kiểm tra passthrough lỗi dispatch 403, kiểm tra 503 khi service đích hoặc Redis sập fail-closed, chạy 2 lượt Newman 0 failed).  
*Ghi chú môi trường:* Số đo thực tế dưới đây được lấy từ Docker Desktop trên máy dev nội bộ sau khi hoàn thành 2 lượt chạy full test suite Postman (48/48 assertions PASS).

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `api-gateway` | 0.06% | 4.30 MiB | 25 MiB | 17.19% | 75 kB / 65.4 kB | 0 B / 0 B | 8 | Healthy |
| `dispatch-service` | 0.02% | 7.22 MiB | 30 MiB | 24.08% | 56.6 kB / 96.2 kB | 2.76 MB / 0 B | 12 | Healthy |
| `pricing-service` | 0.04% | 5.29 MiB | 30 MiB | 17.64% | 14.6 kB / 19.7 kB | 0 B / 0 B | 8 | Healthy |
| `payment-service` | 0.04% | 6.60 MiB | 30 MiB | 21.99% | 62.5 kB / 84.9 kB | 1.46 MB / 0 B | 13 | Healthy |
| `location-service` | 0.02% | 3.54 MiB | 25 MiB | 14.16% | 13.1 kB / 10.8 kB | 0 B / 0 B | 8 | Healthy |
| `user-service` | 0.01% | 7.13 MiB | 30 MiB | 23.75% | 48.9 kB / 63.5 kB | 3.9 MB / 0 B | 14 | Healthy |
| `postgres` | 0.10% | 55.80 MiB | 200 MiB | 27.90% | 135 kB / 73.6 kB | 27.7 MB / 122 MB | 10 | Healthy |
| `redis` | 0.36% | 4.73 MiB | 60 MiB | 7.88% | 13 kB / 8.15 kB | 16.4 kB / 0 B | 6 | Healthy |
| **Tổng cộng** | - | **94.61 MiB** | **430 MiB** | - | - | - | 79 | - |

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (25 MB):** 
  - **Mức tiêu thụ lúc rảnh:** `api-gateway` tiêu thụ ~4.30 MiB RAM (~17.2% định mức 25 MB). Do Gateway không có DB quan hệ và không có background worker nặng, mức RAM cực kỳ gọn nhẹ và ổn định.
  - **Mức tiêu thụ dưới tải cao điểm (Ca T6 - Benchmark 200 requests `GET /api/v1/wallets/me`, 50 luồng song song qua 2 đợt):** RAM đỉnh đo được đạt ~9.56 MiB (~38.2% định mức 25 MB), tỷ lệ thành công 200/200 requests (100%), RAM đo 1 mẫu sau 2 đợt x 100 request; chưa đo soak dài hạn.
  - **Kết luận:** Định mức `mem_limit: 25m` và `GOMEMLIMIT: 22MiB` hoàn toàn đủ và an toàn cho VPS 1 GB (đỉnh tải 9.56 MiB còn cách xa ngưỡng cảnh báo 20 MiB).
- **Xác minh bảo mật & Header Sanitization:** Sanitize middleware xóa sạch toàn bộ header giả mạo `X-User-*` client gửi lên, chỉ gán lại danh tính sau khi giải mã JWT hợp lệ.
- **Xác minh Fail-Closed:** Khi Redis gặp sự cố, Gateway ngắt truy cập ngay lập tức với HTTP 503 `SERVICE_UNAVAILABLE` thay vì cho qua, bảo đảm an toàn dữ liệu phiên đăng nhập.
- **Ngân sách RAM toàn hệ thống:** 94.61 MiB / 850 MiB (~11.1% ngân sách tối đa của VPS 1 GB).

---

## 8. Triển khai `ws-gateway` (Lượt 9)

Đo kiểm sau khi chạy toàn bộ chuỗi 13 kịch bản kiểm thử (từ chối 8 ca handshake sai mã lỗi, luồng E2E đẩy GPS và chuyển tiếp vị trí tài xế cho khách, đo độ trễ p50/p95 tài xế->khách, ngắt kết nối WebSocket khi đăng xuất qua Redis Pub/Sub `ride:ws_control`, quá tải MAX_CONNECTIONS=2 trả 503, heartbeat Ping/Pong quá hạn 5s đóng 1001, lọc frame rác và đóng 1009 khi frame > 4096B, duy trì kết nối khi token hết hạn nhưng không kiểm lại exp sau handshake, bỏ qua GPS từ role khách, failover khi location-service dừng và phục hồi, failover khi Redis dừng trả 503 và tự động nối lại/resubscribe khi Redis bật lại, đo RAM lúc 0 và 100 kết nối).  
*Ghi chú môi trường:* Số đo thực tế dưới đây được lấy từ Docker Desktop trên máy dev nội bộ sau khi hoàn thành chuỗi kiểm thử tự động.

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `ws-gateway` (idle / 0 conns) | 0.11% | 4.60 MiB | 30 MiB | 15.34% | 15 kB / 17 kB | 0 B / 0 B | 8 | Healthy |
| `ws-gateway` (100 conns active) | 0.02% | 8.28 MiB | 30 MiB | 27.59% | 102 kB / 68.6 kB | 0 B / 0 B | 12 | Healthy |
| `api-gateway` | 2.11% | 7.95 MiB | 25 MiB | 31.81% | 78.8 kB / 72.3 kB | 14 MB / 0 B | 9 | Healthy |
| `dispatch-service` | 2.67% | 16.71 MiB | 30 MiB | 55.69% | 275 kB / 519 kB | 559 MB / 20.6 MB | 11 | Healthy |
| `payment-service` | 2.46% | 21.62 MiB | 30 MiB | 72.08% | 334 kB / 488 kB | 288 MB / 9.02 MB | 13 | Healthy |
| `pricing-service` | 0.02% | 16.45 MiB | 30 MiB | 54.84% | 411 kB / 553 kB | 550 MB / 33.5 MB | 17 | Healthy |
| `location-service` | 0.01% | 3.54 MiB | 25 MiB | 14.14% | 18.7 kB / 14 kB | 311 kB / 0 B | 8 | Healthy |
| `user-service` | 2.02% | 16.77 MiB | 30 MiB | 55.90% | 79.2 kB / 76.1 kB | 269 MB / 7.23 MB | 14 | Healthy |
| `postgres` | 2.68% | 52.53 MiB | 200 MiB | 26.27% | 584 kB / 307 kB | 50.5 MB / 2.53 MB | 9 | Healthy |
| `redis` | 0.57% | 7.20 MiB | 60 MiB | 12.01% | 75.6 kB / 48.5 kB | 3.07 MB / 0 B | 6 | Healthy |
| **Tổng cộng (lúc 100 conns)** | - | **151.05 MiB** | **460 MiB** | - | - | - | 99 | - |

> **Phép tính tổng cộng (trạng thái 100 kết nối, không cộng dồn dòng idle):**  
> - RAM: $8.28 + 7.95 + 16.71 + 21.62 + 16.45 + 3.54 + 16.77 + 52.53 + 7.20 = 151.05\text{ MiB}$  
> - PIDs: $12 + 9 + 11 + 13 + 17 + 8 + 14 + 9 + 6 = 99\text{ PIDs}$  
> - % Ngân sách VPS: $151.05 / 850 \approx 17.77\%$

### Đánh giá & Khuyến nghị giới hạn tài nguyên

- **Đánh giá `mem_limit` hiện tại (30 MB):** 
  - Lúc 0 kết nối (idle): `ws-gateway` tiêu thụ ~4.60 MiB RAM (~15.3% định mức 30 MB).
  - Lúc 100 kết nối đồng thời: `ws-gateway` tiêu thụ ~8.28 MiB RAM (~27.6% định mức 30 MB), dưới ngưỡng cảnh báo 24 MiB (34.5% ngưỡng cảnh báo).
  - Định mức `mem_limit: 30m` và `GOMEMLIMIT: 26MiB` cho VPS 1 GB.
- **Đo kiểm độ trễ chuyển tiếp GPS tài xế -> khách hàng (DRIVER_LOCATION_UPDATED):**
  - Số lượng mẫu: 35 lần gửi liên tục (1 frame / 200 ms).
  - $p50 = 1.96\text{ ms}$
  - $p95 = 2.98\text{ ms}$
  - $\max = 3.52\text{ ms}$
  - Ghi chú: 1 tài xế, loopback, 35 mẫu; kịch bản 100 tài xế đo ở Lượt 10.
- **Khả năng chịu lỗi (Resilience & Auto-Resubscribe):**
  - Khi `location-service` tạm dừng: `ws-gateway` ghi log `ERROR` nhưng không làm đứt kết nối WebSocket của tài xế và vẫn tra cứu `trip:link:<driver_id>` để chuyển tiếp vị trí cho khách hàng bình thường.
  - Khi `redis` tạm dừng: handshake mới fail-closed trả HTTP 503; khi `redis` chạy lại, thư viện `go-redis` tự động kết nối và tái đăng ký (auto-resubscribe) các kênh `ride:trip_offers`, `ride:trip_updates`, `ride:ws_control`, các kết nối WebSocket cũ vẫn nhận được cuốc mới mà không cần kết nối lại.
- **Ngân sách RAM toàn hệ thống:** 151.05 MiB / 850 MiB (~17.8% ngân sách tối đa của VPS 1 GB).

---

## 9. Mô phỏng tải 100 Tài xế ảo & Chế độ Chuyến xe (Lượt 10)

Đo kiểm khi chạy script giả lập `scripts/sim-drivers/sim.py` với 100 tài xế ảo gửi GPS định kỳ 3–5 giây lên `ws-gateway` trong thời gian 300 giây.  
*Ghi chú môi trường:* Toàn bộ số đo dưới đây là số liệu thực tế đo trên máy local dev (Docker Desktop); số liệu trên máy chủ VPS production 1 GB RAM sẽ được đo và bổ sung sau khi deploy.

### 9.1. Kiểm tra Redis GEO (`ZCARD drivers:geo`)
- Lệnh: `redis-cli -a redis_secret_pass ZCARD drivers:geo`
- Kết quả: **100** tài xế có vị trí hợp lệ trong sorted set GEO của Redis.

### 9.2. Kết quả `docker stats --no-stream` (Giữa chừng lúc 100 tài xế đang stream GPS)

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `api-gateway` | 0.05% | 8.31 MiB | 25 MiB | 33.25% | 544 kB / 563 kB | 14.8 MB / 0 B | 15 | Healthy |
| `ws-gateway` | 2.03% | 22.94 MiB | 30 MiB | 76.46% | 616 kB / 857 kB | 18.4 MB / 0 B | 18 | Healthy |
| `pricing-service` | 5.49% | 16.25 MiB | 30 MiB | 54.18% | 487 kB / 694 kB | 88.2 MB / 680 kB | 17 | Healthy |
| `dispatch-service` | 0.08% | 15.84 MiB | 30 MiB | 52.79% | 402 kB / 713 kB | 635 MB / 35.6 MB | 17 | Healthy |
| `payment-service` | 2.44% | 17.74 MiB | 30 MiB | 59.13% | 425 kB / 630 kB | 94.3 MB / 1.1 MB | 14 | Healthy |
| `location-service` | 1.48% | 18.06 MiB | 25 MiB | 72.25% | 527 kB / 627 kB | 17.7 MB / 0 B | 17 | Healthy |
| `user-service` | 0.02% | 15.86 MiB | 30 MiB | 52.85% | 399 kB / 487 kB | 1.2 GB / 90.1 MB | 17 | Healthy |
| `postgres` | 0.04% | 61.52 MiB | 200 MiB | 30.76% | 917 kB / 548 kB | 51.6 MB / 4.36 MB | 14 | Healthy |
| `redis` | 1.06% | 11.84 MiB | 60 MiB | 19.73% | 2.14 MB / 1.24 MB | 31.6 MB / 721 kB | 6 | Healthy |
| **Tổng cộng** | - | **188.36 MiB** | **460 MiB** | - | - | - | 135 | - |

> **Đánh giá ngân sách RAM:**
> - Tổng RAM tiêu thụ: **188.36 MiB** / mục tiêu $\le 850\text{ MiB}$ (~22.16% ngân sách RAM của VPS 1 GB).
> - Tất cả 9 service đều duy trì mức sử dụng dưới ngưỡng `mem_limit` được cấu hình.
> - `ws-gateway` chịu tải 100 kết nối WebSocket gửi frame mỗi 3-5s tiêu thụ ~22.94 MiB (dưới hạn mức 30 MiB).

### 9.3. Đo kiểm Độ trễ chuyển tiếp GPS (`DRIVER_LOCATION_UPDATED`)
- **Môi trường thực hiện:** Chạy trên máy local dev (tiến trình sim, api-gateway, ws-gateway, Redis chạy chung trên cùng một máy host nên gần như không có độ trễ mạng). Số đo thực tế qua Internet trên VPS sẽ được bổ sung sau khi deploy.
- **Phương pháp đo:** Dùng `time.perf_counter()` ghi nhận mốc thời gian lúc tài xế phát frame và lúc khách hàng nhận frame qua WebSocket (khớp theo bộ khóa `(driver_id, latitude, longitude)`).
- **Kết quả đo:**
  - Số mẫu ghi nhận: **47 mẫu**
  - $p50$: **1.85 ms**
  - $p95$: **2.65 ms**
  - $\max$: **2.95 ms**

---

## 10. Nhật ký gỡ lỗi (Debugging Log)

- **Lỗi DNS lookup timeout kéo dài 53 giây khi dừng container Redis trong lúc kiểm tra handshake (`ws-gateway`):** Khi `redis` bị `docker stop`, hàm `rdb.Get` mặc định không có context timeout độc lập nên thư viện `go-redis` thực hiện retry dial nhiều lần đến DNS nội bộ Docker (`lookup redis on 127.0.0.11:53: no such host`), khiến handshake bị block tới ~53 giây mới trả về lỗi 503.  
  *Cách khắc phục:* Bổ sung `context.WithTimeout(ctx, 1*time.Second)` vào `RedisBlacklistRepository.IsBlacklisted`. Khi Redis dừng hoặc mất kết nối, hàm lập tức hết hạn sau 1 giây và trả về lỗi, giúp middleware fail-closed trả HTTP 503 nhanh chóng.
- **Lỗi mã hóa ký tự UTF-8 tiếng Việt trên console Python Windows (`charmap codec can't encode character`):** Terminal Windows mặc định dùng bảng mã cp1252/cp437 gây lỗi khi in log tiếng Việt có dấu từ script kiểm thử.  
  *Cách khắc phục:* Thêm `sys.stdout.reconfigure(encoding='utf-8')` ở đầu các script Python trong thư mục scratch.
- **Lỗi escape ký tự JSON trong PowerShell khi gọi `curlimages/curl`:** Khi chạy câu lệnh PowerShell với cờ `-d '{...}'`, PowerShell tự động bóc tách dấu nháy kép khiến parser JSON của Go nhận payload bị lỗi cú pháp `VALIDATION_ERROR`.  
  *Cách khắc phục:* Chuyển sang kỹ thuật truyền dữ liệu qua stdin (`<json> | docker run -i ... curlimages/curl --data-binary "@-"`) hoặc dùng test runner tự động giúp payload JSON được bảo toàn nguyên vẹn 100%. (Lưu ý cờ `@-` phải đặt trong dấu nháy kép `"@-"` trên PowerShell để không bị lỗi token).
- **Lỗi tràn dung lượng ổ đĩa C trong quá trình Docker build (`There is not enough space on the disk`):** Ổ C máy host cạn kiệt dung lượng (chỉ còn ~100 MB free) khiến tiến trình biên dịch Go/BuildKit thất bại và ngắt kết nối daemon WSL2 (`unexpected EOF`).  
  *Cách khắc phục:* Chuyển `GOTMPDIR` và `GOCACHE` sang ổ D (`D:\tmp_go`, `D:\tmp_gocache`), giải phóng cache build giúp biên dịch ổn định.
- **Lỗi xung đột multiple default values trên cột `id` bảng `pricing_configs` trong GORM PostgreSQL:** Khi khai báo `ID int `gorm:"primaryKey;default:1"``, GORM tự động tạo sequence auto-increment và gán default là `nextval(...)`, đồng thời tag `default:1` lại gán thêm 1 default nữa, gây lỗi `ERROR: multiple default values specified for column "id" of table "pricing_configs" (SQLSTATE 42601)`.  
  *Cách khắc phục:* Khai báo tường minh `gorm:"primaryKey;type:integer;default:1;autoIncrement:false"` giúp GORM tạo đúng DDL `id integer PRIMARY KEY DEFAULT 1` mà không thêm sequence ngầm định.
- **Xử lý số lần giao lại trong Redis Stream PEL (`payment-service` & `pricing-service`):** Lệnh `XREADGROUP ... 0` đọc lại tin nhắn từ PEL không tự động tăng `DeliveredTimes` trên Redis. Do đó, consumer sử dụng `map[string]int` có mutex lock để tự theo dõi: lần đầu thấy tin trong PEL tính là lần giao thứ 2. Nếu vượt quá `MAX_DELIVERY_ATTEMPTS` (5 lần), hệ thống ghi log `ERROR` và `XACK` giải phóng tin nhắn để không tắc nghẽn đường ống.
- **Lỗi Unused variable & Unused import trong Go vet (dispatch-service):** Trong quá trình triển khai, biến `appErr` và import `fmt` trong một số file impl không sử dụng trực tiếp dẫn tới compiler error.  
  *Cách khắc phục:* Làm sạch toàn bộ biến cục bộ và import thừa trước khi build, đảm bảo `go vet ./...` đạt exit code 0 tuyệt đối.

