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
| **WS-02** | Bắt lỗi thiếu Token xác thực | `GET /ws/` (Upgrade) | `Connection: Upgrade`<br>`Upgrade: websocket`<br>`Sec-WebSocket-Key: ...` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Bị từ chối TRƯỚC khi upgrade WebSocket. |
| **WS-03** | Chặn truyền Token qua Query Param | `GET /ws/?token={{token}}` (Upgrade) | `Connection: Upgrade`<br>`Upgrade: websocket` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Token chỉ được chấp nhận qua header `Authorization`. |
| **WS-04** | Chặn Token sai chữ ký / alg none / hết hạn | `GET /ws/` (Upgrade) | `Authorization: Bearer <invalid_jwt>` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Thư viện JWT xác thực thuật toán HS256 và hạn dùng `exp`. |
| **WS-05** | Chặn Token đã đăng xuất (Blacklist) | `GET /ws/` (Upgrade) | `Authorization: Bearer {{logged_out_jwt}}` | *(None)* | **401 Unauthorized**<br>`UNAUTHORIZED` | Tra cứu Redis `blacklist:<jti>` thấy tồn tại key. |
| **WS-06** | Chặn Handshake từ Quản trị viên (ADMIN) | `GET /ws/` (Upgrade) | `Authorization: Bearer {{admin_jwt}}` | *(None)* | **403 Forbidden**<br>`FORBIDDEN` | Admin không sử dụng WebSocket. |
| **WS-07** | Từ chối HTTP Request thường không có Upgrade | `GET /ws/` | `Authorization: Bearer {{d1_jwt}}` | *(None)* | **426 Upgrade Required**<br>`UPGRADE_REQUIRED` | Không phải yêu cầu nâng cấp giao thức WebSocket. |
| **WS-08** | Luồng nghiệp vụ End-to-End qua WebSocket | `WS /ws/` | `Authorization: Bearer {{d1_jwt}}`<br>`Authorization: Bearer {{c1_jwt}}` | D1: `{"latitude":10.7738,"longitude":106.6992}` | D1 nhận `TRIP_OFFERED`<br>C1 & D1 nhận `TRIP_STATUS_UPDATED`<br>C1 nhận `DRIVER_LOCATION_UPDATED` | C1 chỉ nhận GPS của D1 sau khi D1 đã ACCEPT và trước COMPLETED. |
| **WS-09** | Đo độ trễ chuyển tiếp vị trí tài xế -> khách | `WS /ws/` | C1 & D1 kết nối trong cùng cuốc xe | D1 gửi tọa độ liên tục | $p50 < 10\text{ ms}$, $p95 < 500\text{ ms}$ | Kết quả thực tế: $p50 = 1.96\text{ ms}$, $p95 = 2.98\text{ ms}$. |
| **WS-10** | Đăng xuất ngắt kết nối WebSocket | `WS /ws/` | D1 đang kết nối | D1 gọi `POST /api/v1/auth/logout` | Kết nối WS của D1 bị đóng với **Mã 1000 (Normal)** | `user-service` phát Pub/Sub `ride:ws_control` DISCONNECT. |
| **WS-11** | Chặn quá tải MAX_CONNECTIONS | `GET /ws/` (Upgrade) | Kết nối thứ 3 khi cấu hình `MAX_CONNECTIONS=2` | *(None)* | **503 Service Unavailable**<br>`SERVICE_UNAVAILABLE` | Cơ chế bảo vệ tài nguyên chống cạn kiệt bộ nhớ. |
| **WS-12** | Heartbeat Ping/Pong quá hạn đóng kết nối | `WS /ws/` | Client cấu hình `autoping=False` | Không gửi lại Pong | Bị đóng kết nối với **Mã 1001 (Going Away)** sau ~5 giây | Server gửi Ping mỗi 2s, timeout Pong 5s. |
| **WS-13** | Xử lý frame rác và frame quá cỡ | `WS /ws/` | D1 gửi text rác / tọa độ 999 / thiếu trường / frame > 4096B | Frame sai: bỏ qua, giữ kết nối<br>Frame > 4096B: **Đóng 1009** | Server không crash, kết nối hợp lệ duy trì an toàn. |
| **WS-14** | Bỏ qua tọa độ gửi từ khách hàng (CUSTOMER) | `WS /ws/` | C1 kết nối và gửi payload GPS | `{"latitude":10.77,"longitude":106.69}` | Bỏ qua im lặng, không ghi vào Redis `drivers:geo` | Chỉ tài xế mới được quyền cập nhật vị trí. |
| **WS-15** | Khả năng chịu lỗi (Resilience) | `WS /ws/` | Dừng `location-service` hoặc `redis` | D1 gửi GPS / Tạo cuốc mới | location tắt: C1 vẫn nhận GPS<br>redis tắt: 503<br>redis bật lại: auto-resubscribe nhận cuốc | Hệ thống phục hồi mượt mà không cần can thiệp thủ công. |
| **WS-16** | Đo kiểm định mức bộ nhớ RAM | `WS /ws/` | 100 kết nối đồng thời với token C1 | Duy trì kết nối | RAM 0 conn: **4.60 MiB**<br>RAM 100 conn: **8.28 MiB** (< 24 MiB) | Tiết kiệm RAM vượt chuẩn thiết kế VPS 1 GB. |

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

Kiểm thử tương tác hai chiều bên trong mạng Docker `backend-net`:

```powershell
# Chạy websocat container kết nối tới ws-gateway:8081
docker run --rm -it --network backend-net solsson/websocat -H="Authorization: Bearer <driver_token>" ws://ws-gateway:8081/ws/
```
Sau khi kết nối, gõ tọa độ JSON:
```json
{"latitude": 10.7738, "longitude": 106.6992}
```

---

## 4. KẾT QUẢ ĐO KIỂM THỰC TẾ

1. **Toàn bộ 16 ca kiểm thử đạt kết quả 100%:**
   - Handshake Guard chặn chính xác mọi trường hợp vi phạm bảo mật theo đúng thứ tự ưu tiên.
   - Luồng sự kiện chuyến đi và cập nhật vị trí thời gian thực hoạt động đồng bộ với `dispatch-service` và `api-gateway`.
2. **Độ trễ chuyển tiếp vị trí tài xế -> khách hàng:**
   - $p50 = 1.96\text{ ms}$, $p95 = 2.98\text{ ms}$, $\max = 3.52\text{ ms}$.
   - Đáp ứng vượt xa chỉ tiêu đề tài ($p95 < 500\text{ ms}$).
3. **Tiêu thụ tài nguyên bộ nhớ:**
   - 0 kết nối: **4.60 MiB** / 30 MiB.
   - 100 kết nối: **8.28 MiB** / 30 MiB.
   - Không rò rỉ bộ nhớ, giải phóng kết nối tức thì khi ngắt session.
