# THIẾT KẾ CHI TIẾT DỊCH VỤ ĐỊNH VỊ (LOCATION-SERVICE LLD)

> **Mã tài liệu:** `LLD-LOCATION` | **Service:** `location-service` | **Database:** Không có DB (Postgres/locationdb bỏ qua theo FR-10)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [SRS v1.5](../01-srs/srs.md), [Hợp đồng liên service](../03-lld/00-hop-dong-lien-service.md), [UC-02 (UC-11, UC-12)](../02-use-cases/uc-02-dinh-vi.md), [UC-03 (UC-15)](../02-use-cases/uc-03-dat-xe-va-ghep-chuyen.md), [UC-07 (UC-29)](../02-use-cases/uc-07-admin.md)

---

## 1. TRÁCH NHIỆM

- Tiếp nhận và lưu trữ vị trí GPS thời gian thực của tài xế vào Redis GEO (`drivers:geo`) và ghi nhận mốc thời gian hoạt động (`driver:last_seen:<driver_id>`).
- Cung cấp API nội bộ tìm kiếm ứng viên tài xế gần điểm đón trong bán kính chỉ định cho `dispatch-service` (lọc tín hiệu mới $\le 15\text{s}$, không lọc trạng thái `ONLINE`).
- Cung cấp API nội bộ truy vấn danh sách tài xế trong một ô Geohash 5 (lọc tín hiệu mới $\le 15\text{s}$) phục vụ tính toán nguồn cung (Supply) cho `pricing-service`.
- Cung cấp API cho Quản trị viên (`ADMIN`) xem danh sách tài xế đang hoạt động kèm vị trí GPS mới nhất.

---

## 2. BẢNG DB (DỮ LIỆU REDIS THAY THẾ)

`location-service` **không có DB** quan hệ (SRS FR-10 đã bỏ qua, không tạo cơ sở dữ liệu `locationdb` trên PostgreSQL nhằm tiết kiệm RAM). Dữ liệu vị trí được lưu trữ hoàn toàn trên Redis:

| Cấu trúc dữ liệu | Phân loại | TTL | Mục đích & Mô tả |
| :--- | :--- | :--- | :--- |
| `drivers:geo` | GEO (ZSET) | Không TTL | Member: `driver_id`, Coordinates: `(longitude, latitude)`. Lưu vị trí không gian mới nhất. |
| `driver:last_seen:<driver_id>` | String | 30 giây | Value: Unix timestamp UTC (giây). Mốc nhận GPS gần nhất; dùng để kiểm tra độ tươi $\le 15\text{s}$. |

- **Chính sách dọn key & Giới hạn RAM (Mục 5(g) Hợp đồng LLD-00):**
  - Cơ chế lọc độ tươi kết hợp ZREM lười (lazy eviction): Không chạy worker quét định kỳ để tiết kiệm CPU; trong các luồng truy vấn (`candidates`, `geohash-drivers`, và `GET /api/v1/admin/drivers/active` mục 4.4), sau khi `MGET driver:last_seen:<id>`, mọi member có key hết hạn (`last_seen = nil`) sẽ được thu gom và xóa lười khỏi `drivers:geo` bằng lệnh `ZREM drivers:geo <id1> <id2>...`.
  - Khi tài xế gửi GPS mới, lệnh `GEOADD` tự động cập nhật tọa độ member sẵn có mà không làm phình bộ nhớ.
  - Redis cấu hình `maxmemory 50mb` và `maxmemory-policy noeviction`. Dữ liệu GEO tiêu thụ $< 1\text{ MB}$ RAM cho hàng ngàn tài xế.

---

## 3. API SPECIFICATION

Định dạng JSON chuẩn: `{"success": bool, "data": ..., "error": {"code": string, "message": string}}`. Header `X-User-Id` và `X-User-Role` do Gateway chuyển tiếp vào cho các endpoint công khai.

### 3.1. API Công khai (Public Endpoints)

| Method & Path | Tầng | Actor | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `GET /api/v1/admin/drivers/active` | 3 | `ADMIN` | *(None)* | `drivers` (array of `{driver_id, latitude, longitude, last_seen}` với `last_seen` định dạng ISO 8601 UTC) | `FORBIDDEN` (403: role != ADMIN)<br>`SERVICE_UNAVAILABLE` (503: lỗi Redis) |
| `GET /health` *(do LLD đặt)* | 1 | Tất cả | *(None)* | `status` ("ok") | *(None)* |

*(B: Bắt buộc, T: Tùy chọn)*

### 3.2. API Nội bộ (Internal Endpoints)

| Method & Path | Tầng | Caller | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `POST /internal/v1/locations/record` | 1 | `ws-gateway` | `driver_id` (uuid, B: không rỗng, đúng UUID)<br>`latitude` (*float, B: [-85.05112878, 85.05112878])<br>`longitude` (*float, B: [-180, 180])<br>*(latitude/longitude dùng kiểu con trỏ để phân biệt thiếu trường với 0)* | `success` (bool: true) | `VALIDATION_ERROR` (400: driver_id không hợp lệ, hoặc vĩ độ ngoài [-85.05112878, 85.05112878], hoặc kinh độ ngoài [-180, 180])<br>`SERVICE_UNAVAILABLE` (503: lỗi Redis) |
| `POST /internal/v1/locations/candidates` | 1 | `dispatch-service` | `pickup_lat` (*float, B: [-85.05112878, 85.05112878])<br>`pickup_lng` (*float, B: [-180, 180])<br>`radius_km` (float, T, mặc định 5.0; yêu cầu > 0 và <= MAX_RADIUS_KM) | `candidates` (array of `{driver_id, distance_m}`) | `VALIDATION_ERROR` (400: thiếu hoặc sai tọa độ, hoặc radius_km <= 0 hoặc > MAX_RADIUS_KM)<br>`SERVICE_UNAVAILABLE` (503: lỗi Redis) |
| `POST /internal/v1/locations/geohash-drivers` | 1 | `pricing-service` | `geohash5` (string, B, độ dài 5) | `driver_ids` (array of string) | `VALIDATION_ERROR` (400: geohash5 sai độ dài hoặc rỗng)<br>`SERVICE_UNAVAILABLE` (503: lỗi Redis) |

*(Ghi chú: ws-gateway đã lọc role DRIVER trước khi gọi record, location-service tin tưởng dữ liệu này).*

---

## 4. LUỒNG XỬ LÝ TỪNG BƯỚC

### 4.1. Ghi nhận tọa độ GPS tài xế (`POST /internal/v1/locations/record`)
1. Validate dữ liệu: `driver_id` không rỗng và đúng định dạng UUID; `latitude` và `longitude` dùng kiểu con trỏ `*float64` để phân biệt thiếu trường với giá trị 0; kiểm tra `latitude` $\in [-85.05112878, 85.05112878]$ và `longitude` $\in [-180, 180]$ theo giới hạn của Redis GEO (sai $\rightarrow$ `VALIDATION_ERROR` 400).
2. Thứ tự ghi nhận vào Redis:
   - Thực thi `SET driver:last_seen:<driver_id> <unix_now> EX 30` TRƯỚC `GEOADD drivers:geo <longitude> <latitude> <driver_id>` (thứ tự này đảm bảo key `last_seen` luôn tồn tại trước khi vị trí xuất hiện trong ZSET GEO, ngăn cơ chế ZREM lười ở các luồng truy vấn đồng thời xóa oan tài xế vừa gửi GPS).
   - Nếu bất kỳ lệnh Redis nào gặp sự cố (`SET`, `GEOADD`) $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503).
3. Trả về 200 OK với `{"success": true}`.

### 4.2. Quét tìm ứng viên tài xế lân cận (`POST /internal/v1/locations/candidates`)
1. Validate `(pickup_lat, pickup_lng)` với vĩ độ $\in [-85.05112878, 85.05112878]$, kinh độ $\in [-180, 180]$ (sai $\rightarrow$ `VALIDATION_ERROR` 400). Lấy `radius_km` (nếu thiếu dùng mặc định `DEFAULT_RADIUS_KM` = 5.0). Kiểm tra `radius_km > 0` và `radius_km <= MAX_RADIUS_KM` (vượt ngưỡng hoặc $\le 0 \rightarrow$ `VALIDATION_ERROR` 400, tuyệt đối không âm thầm cắt giảm).
2. Truy vấn thô trên Redis:
   `GEOSEARCH drivers:geo FROMLONLAT <pickup_lng> <pickup_lat> BYRADIUS <radius_km * 1000> m WITHDIST ASC COUNT <GEO_SCAN_COUNT>`.
   *(GEOSEARCH dùng đơn vị mét `m` tương đương `radius_km * 1000`; số lượng quét thô cấu hình qua ENV `GEO_SCAN_COUNT`, mặc định 500).* Nếu Redis lỗi $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503).
3. Nếu kết quả thô rỗng $\rightarrow$ Trả về 200 OK với `candidates: []`.
4. Kiểm tra độ tươi bằng MGET và ZREM lười:
   - Lập mảng key `driver:last_seen:<id>` của các ứng viên và thực thi lệnh `MGET`. Nếu Redis lỗi $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503).
   - Với các member có `last_seen = nil` (key hết hạn), gom ID và thực thi `ZREM drivers:geo <id1> <id2>...` ngay sau `MGET` (cơ chế ZREM lười dọn sạch tài xế mất sóng).
5. Lọc ứng viên: Chỉ giữ lại tài xế có key tồn tại và $(now - \text{last\_seen}) \le 15\text{s}$. Làm tròn khoảng cách `distance_m` thành số nguyên.
6. Sắp xếp tăng dần theo khoảng cách `distance_m`, cắt lấy tối đa 50 ứng viên đầu tiên. Trả về 200 OK. *(Lưu ý: Không lọc trạng thái ONLINE; tài xế đang BUSY vẫn gửi GPS định kỳ nên vẫn nằm trong danh sách candidates trả về; dispatch-service sẽ chịu trách nhiệm lọc trạng thái ONLINE qua filter-online với tối đa 500 ID).*

### 4.3. Quét tài xế trong một ô Geohash 5 (`POST /internal/v1/locations/geohash-drivers`)
1. Validate `geohash5` có đúng 5 ký tự (sai $\rightarrow$ `VALIDATION_ERROR` 400).
2. Giải mã `geohash5` xác định tâm ô `(center_lat, center_lng)`.
3. Truy vấn không gian bao phủ ô:
   `GEOSEARCH drivers:geo FROMLONLAT <center_lng> <center_lat> BYBOX 5.5 5.5 km WITHCOORD COUNT <GEO_SCAN_COUNT>`.
   *(Số lượng quét thô cấu hình qua ENV `GEO_SCAN_COUNT`, mặc định 500).* Nếu Redis lỗi $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503).
4. Nếu kết quả rỗng $\rightarrow$ Trả về 200 OK với `driver_ids: []`.
5. Lọc chính xác ô, kiểm tra độ tươi và ZREM lười:
   - Mã hóa lại Geohash độ dài 5 từ `WITHCOORD` của từng tài xế, chỉ giữ tài xế có geohash đúng bằng `geohash5` yêu cầu (loại bỏ tài xế lấn sang ô lân cận theo Mục 5(e) Hợp đồng LLD-00).
   - Gọi `MGET driver:last_seen:<id>` cho các ứng viên. Nếu Redis lỗi $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503).
   - Với các member có `last_seen = nil`, gom ID và thực thi `ZREM drivers:geo <id1> <id2>...` ngay sau `MGET` (ZREM lười).
   - Chỉ giữ tài xế có mốc thời gian $(now - \text{last\_seen}) \le 15\text{s}$.
6. Trả về 200 OK với danh sách `driver_ids: []`.

### 4.4. Giám sát tài xế đang hoạt động (`GET /api/v1/admin/drivers/active`)
1. Kiểm tra header `X-User-Role == 'ADMIN'` (sai $\rightarrow$ `FORBIDDEN` 403).
2. Lấy danh sách toàn bộ ID từ `drivers:geo` bằng `ZRANGE drivers:geo 0 -1`. Nếu Redis lỗi $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503).
3. Gọi `MGET` các key `driver:last_seen:<id>`, gom các member có `last_seen = nil` để `ZREM drivers:geo`, lọc lấy danh sách có mốc thời gian $\le 15\text{s}$.
4. Gọi `GEOPOS drivers:geo <id1> <id2>...` để lấy tọa độ của các tài xế hợp lệ.
5. Trả về 200 OK với danh sách `[{driver_id, latitude, longitude, last_seen}]` trong đó `last_seen` được định dạng chuỗi ISO 8601 UTC (dữ liệu lưu trên Redis vẫn là Unix timestamp tính bằng giây).

---

## 5. XỬ LÝ NGOẠI LỆ (EXCEPTION HANDLING)

| Tình huống ngoại lệ | Ngữ cảnh phát sinh | Hành vi xử lý & Mã lỗi |
| :--- | :--- | :--- |
| Tọa độ không hợp lệ | Vĩ độ ngoài [-85.05112878, 85.05112878], kinh độ ngoài [-180, 180], thiếu tọa độ hoặc driver_id sai UUID | Trả `VALIDATION_ERROR` (400). |
| Bán kính không hợp lệ | `radius_km <= 0` hoặc `radius_km > MAX_RADIUS_KM` | Trả `VALIDATION_ERROR` (400) (không âm thầm cắt giảm). |
| Mã Geohash5 sai | Chuỗi geohash rỗng hoặc khác 5 ký tự | Trả `VALIDATION_ERROR` (400). |
| Lỗi Redis hạ tầng | Lệnh `GEOADD`, `GEOSEARCH`, `MGET`, `SET`, `ZREM` gặp sự cố kết nối/timeout | Trả `SERVICE_UNAVAILABLE` (503). |
| Không có ứng viên lân cận | Không có tài xế nào có GPS $\le 15\text{s}$ trong bán kính | Trả về 200 OK với `candidates: []`. |
| Không có tài xế trong ô | Không có tài xế nào có GPS $\le 15\text{s}$ trong ô geohash5 | Trả về 200 OK với `driver_ids: []`. |
| Sai vai trò Quản trị viên | Người dùng không phải ADMIN gọi API giám sát | Trả `FORBIDDEN` (403). |

### 5.1. Hạn chế đã biết
- **Tài xế mất sóng và cơ chế dọn dẹp:** Không chạy worker định kỳ quét toàn bộ ZSET để tránh tốn CPU; hệ thống áp dụng cơ chế lọc độ tươi kết hợp ZREM lười (xóa key mất sóng ngay khi `MGET` phát hiện `last_seen = nil`), cùng với việc `GEOADD` tự động ghi đè tọa độ khi tài xế gửi GPS mới.

---

## 6. SỰ KIỆN & KHÓA REDIS

### 6.1. Tài nguyên Redis quản lý
- `drivers:geo`: ZSET/GEO, không TTL. Lưu tọa độ GPS mới nhất của các tài xế. Cập nhật bằng `GEOADD`.
- `driver:last_seen:<driver_id>`: String, TTL 30s. Mốc timestamp UTC lần gửi GPS gần nhất. Ghi bằng `SET EX 30`. Đọc hàng loạt bằng `MGET`.

### 6.2. Kênh Pub/Sub và Stream
`location-service` **không** trực tiếp phát hay đăng ký Pub/Sub hoặc Redis Stream nào (đóng vai trò lưu trữ và tính toán không gian phục vụ các service qua HTTP REST nội bộ).

---

## 7. CẤU HÌNH (ENV) VÀ TÀI NGUYÊN

### 7.1. Bảng biến môi trường
| Tên biến ENV | Mặc định | Ý nghĩa & Mục đích sử dụng |
| :--- | :--- | :--- |
| `PORT` | `8002` | Cổng HTTP nội bộ của `location-service` trong mạng Docker. |
| `REDIS_ADDR` | `redis:6379` | Địa chỉ kết nối Redis container trong Docker. |
| `REDIS_PASSWORD` | `redis_secret_pass` | Mật khẩu xác thực kết nối Redis (demo, ghi đè trong .env). |
| `DEFAULT_RADIUS_KM` | `5.0` | Bán kính tìm kiếm ứng viên tài xế lân cận mặc định (km). |
| `MAX_RADIUS_KM` | `10.0` | Bán kính tìm kiếm ứng viên tài xế tối đa cho phép (km); vượt ngưỡng trả VALIDATION_ERROR. |
| `GPS_FRESHNESS_SECONDS` | `15` | Ngưỡng thời gian tối đa để xác định GPS còn mới (giây). |
| `GEO_SCAN_COUNT` | `500` | Số lượng ứng viên quét thô tối đa từ Redis GEOSEARCH cho `candidates` và `geohash-drivers`. |

### 7.2. Tài nguyên & Thứ tự khởi động
- **Ràng buộc bộ nhớ VPS 1GB:** `mem_limit` Docker đề xuất: 25 MB; `GOMEMLIMIT = 22MiB` (chống OOM).
- **Kết nối Database:** Không có kết nối PostgreSQL (không có DB, FR-10 bỏ qua `locationdb`).
- **Thứ tự khởi động dịch vụ (Startup Sequence):**
  1. Kết nối Redis và kiểm tra ping `PONG`.
  2. Khởi động HTTP web server (Fiber) trên cổng `8002` bắt đầu nhận request.
