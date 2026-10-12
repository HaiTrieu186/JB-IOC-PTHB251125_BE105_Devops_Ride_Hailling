# SCRIPT MÔ PHỎNG TÀI XẾ VÀ CHUYẾN XE (SIM-DRIVERS)

Script giả lập tải và kiểm thử luồng nghiệp vụ end-to-end với tối đa 100 tài xế ảo và các chuyến xe tự động.

## 1. Yêu cầu môi trường
- Python 3.10+
- Thư viện `aiohttp`:
  ```bash
  pip install aiohttp
  ```

---

## 2. Cách chạy script

### 2.1. Chạy trên môi trường Local Dev (không qua Nginx)
Do môi trường local không có Nginx định tuyến proxy `/ws/` về cổng 8081, tham số `--ws-url` cần được truyền tường minh trỏ trực tiếp tới cổng của `ws-gateway` (127.0.0.1:8081/ws/):

- **Kịch bản 1: Giữ 100 tài xế gửi GPS định kỳ 300 giây (trips = 0):**
  ```bash
  python scripts/sim-drivers/sim.py --api-url http://127.0.0.1:8080 --ws-url ws://127.0.0.1:8081/ws/ --drivers 100 --duration 300 --trips 0
  ```

- **Kịch bản 2: Mô phỏng 100 tài xế và chạy 25 chuyến xe thực tế:**
  ```bash
  python scripts/sim-drivers/sim.py --api-url http://127.0.0.1:8080 --ws-url ws://127.0.0.1:8081/ws/ --drivers 100 --trips 25
  ```

### 2.2. Chạy trên VPS Production (qua HTTPS/WSS và Nginx)
Trên VPS có Nginx quản lý cổng 443 và reverse proxy tự động `/ws/` sang ws-gateway:
```bash
python scripts/sim-drivers/sim.py --api-url https://<domain-vps> --drivers 100 --trips 25
```
*(Tham số `--ws-url` mặc định tự động suy ra `wss://<domain-vps>/ws/` có dấu `/` ở cuối).*

---

## 3. Khả năng chạy lại (Idempotency) & Phục hồi

- Script có thể chạy lại nhiều lần mà không cần xóa dữ liệu: API đăng ký gặp lỗi 409 (`USER_ALREADY_EXISTS`) sẽ tự động bỏ qua và chuyển sang bước đăng nhập.
- Mọi thao tác register/login đều đi qua `asyncio.Semaphore(2)` để bảo vệ CPU tránh quá tải băm bcrypt trên VPS 1 vCPU.
- Nếu tài xế bị kẹt trạng thái `DRIVER_BUSY` từ lần chạy trước khi gọi `PATCH /api/v1/driver/status`, script sẽ bỏ qua lỗi và vẫn kết nối WebSocket gửi tọa độ GPS bình thường.
- Nếu khách gặp `ACTIVE_TRIP_EXISTS` (409), script tự tra cứu `GET /api/v1/trips/current` và gửi lệnh hủy chuyến để tiếp tục luồng.
- **Xử lý chuyến kẹt thủ công:** Tham khảo runbook force-cancel tại [docs/05-deploy/00-ke-hoach-trien-khai.md](../../docs/05-deploy/00-ke-hoach-trien-khai.md) Mục 3:
  1. Tra cứu `GET /api/v1/admin/trips?status=...` và gọi `POST /api/v1/admin/trips/:id/force-cancel`.
  2. Hoặc giải phóng trực tiếp DB nếu cần:
     ```bash
     docker exec -i postgres psql -U user_user -d userdb \
       -c "UPDATE users SET driver_status = 'ONLINE', updated_at = now() WHERE id = '<driver_id>' AND driver_status = 'BUSY';"
     ```

---

## 4. Xóa SẠCH toàn bộ dữ liệu demo (Nếu cần dọn dẹp môi trường)

> [!WARNING]
> **CẢNH BÁO:** Lệnh `TRUNCATE` dưới đây sẽ xóa ví và chuyến xe của **MỌI** tài khoản trong hệ thống (kể cả các tài khoản thử nghiệm thủ công Postman/cURL C1–C3, D1–D5). Hãy cân nhắc trước khi chạy.

```bash
# 1. Dọn dẹp dispatchdb (bảng status_timeline và trips)
docker exec -i postgres psql -U dispatch_user -d dispatchdb -c "
TRUNCATE TABLE status_timeline, trips RESTART IDENTITY CASCADE;
"

# 2. Dọn dẹp paymentdb (bảng transactions và wallets)
docker exec -i postgres psql -U payment_user -d paymentdb -c "
TRUNCATE TABLE transactions, wallets RESTART IDENTITY CASCADE;
"

# 3. Dọn dẹp userdb (bảng vehicles và users dải 0903/0904)
docker exec -i postgres psql -U user_user -d userdb -c "
DELETE FROM vehicles WHERE driver_id IN (SELECT id FROM users WHERE phone_number LIKE '0903%' OR phone_number LIKE '0904%');
DELETE FROM users WHERE phone_number LIKE '0903%' OR phone_number LIKE '0904%';
"
```

---

## 5. Ghi chú về đo độ trễ (Latency Measurement)

- Độ trễ chuyển tiếp GPS (`DRIVER_LOCATION_UPDATED`) được đo bằng cách so khớp chính xác bộ khóa `(driver_id, latitude, longitude)` giữa mốc thời gian `time.perf_counter()` lúc tài xế gửi frame lên ws-gateway và lúc khách hàng nhận được frame qua kết nối WebSocket của chuyến xe.
- Chỉ số $p50$, $p95$, và $\max$ được tính toán thuần túy bằng hàm `sorted()` của thư viện chuẩn Python (không phụ thuộc `numpy`).
- **Lưu ý:** Số đo ghi nhận bao gồm toàn bộ thời gian di chuyển qua mạng, thời gian xử lý của `ws-gateway`, truy vấn Redis `trip:link`, và độ trễ vòng lặp sự kiện (event loop) của chính tiến trình script Python.
