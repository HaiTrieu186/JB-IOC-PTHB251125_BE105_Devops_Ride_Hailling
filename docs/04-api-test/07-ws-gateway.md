# KỊCH BẢN KIỂM THỬ CỔNG WEBSOCKET (WS-GATEWAY TEST SUITE)

> **Mã tài liệu:** `TEST-WS` | **Phiên bản:** `1.0`  
> **Dịch vụ kiểm thử:** `ws-gateway` (Port tiếp nhận: `127.0.0.1:8081`, Path: `/ws/`, Health: `/health`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD ws-gateway](../03-lld/ws-gateway.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

`ws-gateway` là cổng tiếp nhận kết nối WebSocket duy nhất của hệ thống, phục vụ các mục đích:
1. Tiếp nhận tọa độ GPS từ tài xế định kỳ (3-5 giây) và chuyển tiếp bất đồng bộ sang `location-service`.
2. Đẩy thông báo thời gian thực xuống client thông qua Redis Pub/Sub (`ride:trip_offers`, `ride:trip_updates`, `ride:ws_control`).
3. Chuyển tiếp tọa độ trực tiếp của tài xế đến khách hàng đang trong chuyến đi (`DRIVER_LOCATION_UPDATED`).

Toàn bộ các yêu cầu nghiệp vụ (đặt xe, chấp nhận cuốc, cập nhật trạng thái...) đều đi qua REST API tại `api-gateway` (cổng 8080).

Bộ kịch bản kiểm thử bao gồm **16 ca kiểm thử chi tiết**:
- **Kiểm thử Handshake & Bảo mật (WS-01 $\rightarrow$ WS-07):** Health check, từ chối thiếu token, từ chối token query param, token sai/hết hạn/alg none, token trong danh sách đen blacklist, chặn role ADMIN, từ chối request không có header Upgrade.
- **Kiểm thử Nghiệp vụ & Độ trễ (WS-08 $\rightarrow$ WS-09):** Luồng chuyến đi đầy đủ C1 + D1, chuyển tiếp GPS cho khách, đo độ trễ p50/p95/max dưới 500 ms.
- **Kiểm thử Quản lý Phiên & Kết nối (WS-10 $\rightarrow$ WS-13):** Ngắt kết nối khi đăng xuất (Close 1000), từ chối khi vượt ngưỡng MAX_CONNECTIONS (503), timeout Heartbeat Ping/Pong (Close 1001), xử lý frame rác và đóng frame vượt kích thước 4096B (Close 1009).
- **Kiểm thử Ranh giới Quyền hạn & Khả năng Chịu lỗi (WS-14 $\rightarrow$ WS-16):** Bỏ qua GPS từ khách hàng, chịu lỗi khi location-service và Redis dừng, đo kiểm RAM ở 0 và 100 kết nối đồng thời.

---

## 2. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT

| Mã KT | Tên kịch bản | Giao thức & Endpoint | Headers / Tham số | Payload gửi lên | Kỳ vọng Kết quả & Mã đóng | Ghi chú & Dữ liệu tham chiếu |
|:---:|---|---|---|---|---|---|
| **WS-01** | Kiểm tra sức khỏe dịch vụ | `GET /health` | *(None)* | *(None)* | **200 OK**<br>`{"status":"ok"}` | Endpoint kiểm tra liveness của container. |
| **WS-02** | Bắt lỗi thiếu Token xác thực | `GET /ws/` (Upgrade) | `Connection: Upgrade`<br>`Upgrade: websocket`<br>`Sec-WebSocket-Key: ...` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-03** | Chặn truyền Token qua Query Param | `GET /ws/?token={{token}}` (Upgrade) | `Connection: Upgrade`<br>`Upgrade: websocket` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Token chỉ được chấp nhận qua header `Authorization`. |
| **WS-04** | Chặn Token sai chữ ký / alg none / hết hạn | `GET /ws/` (Upgrade) | `Authorization: Bearer <invalid_jwt>` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-05** | Chặn Token đã đăng xuất (Blacklist) | `GET /ws/` (Upgrade) | `Authorization: Bearer {{logged_out_jwt}}` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Tra cứu Redis `blacklist:<jti>` thấy tồn tại key. |
| **WS-06** | Chặn Handshake từ Quản trị viên (ADMIN) | `GET /ws/` (Upgrade) | `Authorization: Bearer {{admin_jwt}}` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-07** | Từ chối HTTP Request thường không có Upgrade | `GET /ws/` | `Authorization: Bearer {{d1_jwt}}` | *(None)* | **426 Upgrade Required**<br>`UPGRADE_REQUIRED` | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-08** | Luồng nghiệp vụ End-to-End qua WebSocket | `WS /ws/` | `Authorization: Bearer {{d1_jwt}}`<br>`Authorization: Bearer {{c1_jwt}}` | D1: `{"latitude":10.7738,"longitude":106.6992}` | D1 nhận `TRIP_OFFERED`<br>C1 & D1 nhận `TRIP_STATUS_UPDATED`<br>C1 nhận `DRIVER_LOCATION_UPDATED` | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-09** | Đo độ trễ chuyển tiếp vị trí tài xế -> khách | `WS /ws/` | C1 & D1 kết nối trong cùng cuốc xe | D1 gửi tọa độ liên tục | $p50 < 10\text{ ms}$, $p95 < 500\text{ ms}$ | 1 tài xế, loopback, 35 mẫu; kịch bản 100 tài xế đo ở Lượt 10. |
| **WS-10** | Đăng xuất ngắt kết nối WebSocket | `WS /ws/` | D1 đang kết nối | D1 gọi `POST /api/v1/auth/logout` | Kết nối WS của D1 bị đóng với **Mã 1000 (Normal)** | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-11** | Chặn quá tải MAX_CONNECTIONS | `GET /ws/` (Upgrade) | Kết nối thứ 3 khi cấu hình `MAX_CONNECTIONS=2` | *(None)* | **503 Service Unavailable**<br>`SERVICE_UNAVAILABLE` | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-12** | Heartbeat Ping/Pong quá hạn đóng kết nối | `WS /ws/` | Client cấu hình `autoping=False` | Không gửi lại Pong | Bị đóng kết nối với **Mã 1001 (Going Away)** sau ~5 giây | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-13** | Xử lý frame rác và frame quá cỡ | `WS /ws/` | D1 gửi text rác / tọa độ 999 / thiếu trường / frame > 4096B | Frame sai: bỏ qua, giữ kết nối<br>Frame > 4096B: **Đóng 1009** | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-14** | Bỏ qua tọa độ gửi từ khách hàng (CUSTOMER) | `WS /ws/` | C1 kết nối và gửi payload GPS | `{"latitude":10.77,"longitude":106.69}` | Bỏ qua im lặng, không ghi vào Redis `drivers:geo` | Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-15** | Khả năng chịu lỗi (Resilience) | `WS /ws/` | Dừng `location-service` hoặc `redis` | D1 gửi GPS / Tạo cuốc mới | location tắt: C1 vẫn nhận GPS<br>redis tắt: 503<br>redis bật lại: auto-resubscribe nhận cuốc | Redis dừng/bật đã kiểm lại ở Lượt 9x (Ca 12: 503 sau 1.02s, auto-resubscribe cuốc mới). Nhánh dừng location-service: Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập. |
| **WS-16** | Đo kiểm định mức bộ nhớ RAM | `WS /ws/` | 100 kết nối đồng thời với token C1 | Duy trì kết nối | RAM 0 conn: **4.60 MiB**<br>RAM 100 conn: **8.28 MiB** (< 24 MiB) | Đo kiểm thực tế trên máy dev (Docker Desktop). |

---

## 3. CÁC LỆNH KIỂM THỬ MẪU (COMMAND EXAMPLES)

### 3.1. Kiểm tra Health Check
```powershell
curl.exe -i http://127.0.0.1:8081/health
```
**Kỳ vọng phản hồi:**
```http
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 15

{"status":"ok"}
```

---

### 3.2. Kiểm tra từ chối Handshake (cURL Upgrade)

#### A. Không có Token (WS-02)
```powershell
curl.exe -i -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" http://127.0.0.1:8081/ws/
```
**Kỳ vọng:** `HTTP/1.1 401 Unauthorized`

#### B. Token qua Query Param (WS-03)
```powershell
curl.exe -i -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" "http://127.0.0.1:8081/ws/?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```
**Kỳ vọng:** `HTTP/1.1 401 Unauthorized`

#### C. Role ADMIN bị chặn (WS-06)
```powershell
curl.exe -i -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" -H "Authorization: Bearer <admin_token>" http://127.0.0.1:8081/ws/
```
**Kỳ vọng:** `HTTP/1.1 403 Forbidden`

#### D. Request HTTP thường không có header Upgrade (WS-07)
```powershell
curl.exe -i -H "Authorization: Bearer <driver_token>" http://127.0.0.1:8081/ws/
```
**Kỳ vọng:** `HTTP/1.1 426 Upgrade Required`

---

### 3.3. Kiểm thử kết nối WebSocket bằng công cụ tạm thời (`websocat`)

Kiểm thử bằng container `solsson/websocat` trong mạng Docker `backend-net` (không dùng cờ TTY `-t` / `-it`):

```powershell
# Chạy websocat với token tài xế D1 (giữ stdin mở 5 giây):
(echo '{"latitude":10.7738,"longitude":106.6992}'; sleep 5) | docker run -i --rm --network backend-net solsson/websocat -v -H="Authorization: Bearer <d1_token>" ws://ws-gateway:8081/ws/

# Ca đối chứng: Chạy KHÔNG header Authorization (kỳ vọng bị từ chối):
(echo '{"latitude":10.7738,"longitude":106.6992}'; sleep 5) | docker run -i --rm --network backend-net solsson/websocat -v ws://ws-gateway:8081/ws/
```

---

## 4. KẾT QUẢ ĐO KIỂM THỰC TẾ

### 4.1. Các ca đã kiểm tra độc lập và ghi nhận output thật (Lượt 9x)

1. **Ca 12 - Khả năng chịu lỗi Redis (Dừng/Bật & Auto-Resubscribe) (`scratch/ws_redis_failover.py`):**
   - Lệnh `docker stop redis`: thực thi xong trong 0.31s.
   - Thử nghiệm handshake mới khi Redis dừng: `HTTP/1.1 503 Service Unavailable`, body `{"data":null,"error":{"code":"SERVICE_UNAVAILABLE","message":"Dịch vụ xác thực tạm thời không khả dụng"},"success":false}`, thời gian đo 1.02s (đúng timeout 1.0s của Handshake Guard).
   - Lệnh `docker start redis`: Redis khởi động và đạt trạng thái `healthy`.
   - Kết nối WebSocket cũ của tài xế D1 và khách hàng C1 (mở trước khi dừng Redis) tự động nhận được các sự kiện sau khi Redis hoạt động lại:
     - D1 nhận JSON frame `TRIP_OFFERED`:
       ```json
       {
         "event": "TRIP_OFFERED",
         "trip_id": "f6953a79-68f3-4861-910a-d6cdee677138",
         "pickup_lat": 10.7725,
         "pickup_lng": 106.698,
         "fare": 69000,
         "driver_fare": 58650,
         "distance_m": 195,
         "expire_at": "2026-10-12T09:21:19Z"
       }
       ```
     - Sau khi D1 chấp nhận cuốc qua REST `POST /trips/{id}/accept`, C1 nhận JSON frame `TRIP_STATUS_UPDATED`:
       ```json
       {
         "event": "TRIP_STATUS_UPDATED",
         "trip_id": "f6953a79-68f3-4861-910a-d6cdee677138",
         "status": "ACCEPTED",
         "updated_at": "2026-10-12T09:20:49Z"
       }
       ```

2. **Ca kiểm tra dọn session ("Dọn session không rò") (`scratch/ws_session_cleanup.py`):**
   - Kịch bản: 1 token mở 100 kết nối đồng thời, đóng toàn bộ, lặp 4 lần liên tiếp (tổng 400 kết nối > ngưỡng `MAX_CONNECTIONS=300`).
   - Lần 1: Mở thành công 100/100 kết nối (Lỗi: 0).
   - Lần 2: Mở thành công 100/100 kết nối (Lỗi: 0).
   - Lần 3: Mở thành công 100/100 kết nối (Lỗi: 0).
   - Lần 4: Mở thành công 100/100 kết nối (Lỗi: 0).
   - Kết quả `docker stats --no-stream ws-gateway` sau lần đóng cuối (chờ 5 giây):
     - `MEM USAGE / LIMIT`: `14.2MiB / 30MiB` (`MEM %`: `47.34%`), PIDs: `18`.

3. **Ca kiểm thử bằng `solsson/websocat` (`scratch/test_websocat.py`):**
   - Chạy với container `solsson/websocat`, không dùng cờ TTY, truyền header `-H="Authorization: Bearer <d1_token>"`:
     - Log `-v`: `[INFO  websocat::ws_client_peer] Connected to ws`, exit code: 0.
   - Ca đối chứng không truyền header:
     - Log `-v`: `websocat: WebSocketError: WebSocket response error`, exit code: 1.

4. **Đo độ trễ chuyển tiếp vị trí tài xế -> khách hàng (DRIVER_LOCATION_UPDATED):**
   - Cấu hình: 1 tài xế, loopback, 35 mẫu; kịch bản 100 tài xế đo ở Lượt 10.
   - $p50 = 1.96\text{ ms}$, $p95 = 2.98\text{ ms}$, $\max = 3.52\text{ ms}$.

5. **Tiêu thụ tài nguyên bộ nhớ:**
   - 0 kết nối: **4.60 MiB** / 30 MiB.
   - 100 kết nối: **8.28 MiB** / 30 MiB.

### 4.2. Các ca không chạy lại trong lượt này (Ca 2, 4, 6, 7, 8, 9, 10, 11 của chuỗi Lượt 9)
- Ghi nhận từ báo cáo lượt 9; output thô không lưu, chưa kiểm lại độc lập.

### 4.3. Chưa kiểm
- `SLOW_CLIENT` (mã đóng 1008 khi buffer client đầy): Chưa có ca kiểm thử tự động.
