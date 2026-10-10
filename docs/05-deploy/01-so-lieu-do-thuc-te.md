# SỐ LIỆU ĐO KIỂM THỰC TẾ (BENCHMARK & RESOURCE USAGE)

> **Mã tài liệu:** `DEP-01` | **Phiên bản:** `1.0` | **Ngày đo:** 10/10/2026

---

## 1. Tầng Hạ tầng cơ bản (PostgreSQL + Redis)

Đo kiểm khi khởi chạy 2 container hạ tầng ở trạng thái rảnh rỗi (idle), đã khởi tạo xong 5 database và xác thực kết nối.

### Kết quả `docker stats --no-stream`

| Container | CPU % | RAM Hiện tại (MEM USAGE) | Giới hạn (LIMIT) | Tỷ lệ RAM (MEM %) | NET I/O | BLOCK I/O | PIDs | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| `postgres` | 0.01% | 36.82 MiB | 200 MiB | 18.41% | 1.82 kB / 126 B | 8.19 kB / 119 MB | 6 | Healthy |
| `redis` | 0.60% | 5.53 MiB | 60 MiB | 9.22% | 1.82 kB / 126 B | 0 B / 8.19 kB | 6 | Healthy |
| **Tổng cộng** | - | **42.35 MiB** | **260 MiB** | - | - | - | 12 | - |

---

## 2. Đánh giá & Đối chiếu ngân sách RAM

- **Ngân sách RAM tối đa toàn hệ thống:** $\le 850\text{ MB}$ (trên tổng 1 GB RAM VPS).
- **Mức tiêu thụ hạ tầng thực tế:** $\approx 42.35\text{ MiB}$ (chỉ chiếm ~5% ngân sách cho phép).
- **Dung lượng RAM còn lại cho 8 microservices (Go) & Nginx:** $\approx 807\text{ MiB}$.
- **Kết luận:** Hạ tầng chạy ổn định, cấu hình bộ nhớ `shared_buffers=32MB`, `work_mem=2MB`, `maxmemory 50mb` đáp ứng tối ưu yêu cầu tài nguyên trên VPS 1 GB RAM.
