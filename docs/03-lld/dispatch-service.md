# THIẾT KẾ CHI TIẾT DỊCH VỤ ĐIỀU PHỐI (DISPATCH-SERVICE LLD)

> **Mã tài liệu:** `LLD-DISPATCH` | **Service:** `dispatch-service` | **Database:** `dispatchdb` (PostgreSQL)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [SRS v1.5](../01-srs/srs.md), [Hợp đồng liên service](../03-lld/00-hop-dong-lien-service.md), [UC-03](../02-use-cases/uc-03-dat-xe-va-ghep-chuyen.md), [UC-04](../02-use-cases/uc-04-chuyen-di-va-huy.md), [UC-30](../02-use-cases/uc-07-admin.md)

---

## 1. TRÁCH NHIỆM

- Quản lý toàn bộ vòng đời chuyến xe theo máy trạng thái: `CREATED` $\rightarrow$ `MATCHING` $\rightarrow$ `ACCEPTED` $\rightarrow$ `PICKING_UP` $\rightarrow$ `IN_TRIP` $\rightarrow$ `COMPLETED` (nhánh hủy: `CANCELLED`; hết hạn: `EXPIRED`).
- Điều phối ghép chuyến: kiểm tra ví, tính cước, tìm ứng viên gần nhất, lọc ONLINE một lần, chọn Top 3–5, broadcast lời mời cuốc xe.
- Quản lý tranh cuốc cạnh tranh đa tài xế bằng khóa phân tán Redis nguyên tử và cập nhật điều kiện DB.
- Vận hành worker quét chuyến quá hạn định kỳ và dọn dẹp liên kết điều hướng realtime (FR-38).

---

## 2. BẢNG DB (DISPATCHDB)

### 2.1. Bảng `trips` (Bản ghi chuyến xe)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `UUID` | PK, DEFAULT `gen_random_uuid()` | Định danh chuyến xe duy nhất toàn hệ thống. |
| `customer_id` | `UUID` | NOT NULL | ID khách hàng đặt xe. |
| `driver_id` | `UUID` | NULLABLE | ID tài xế nhận cuốc (gán khi `ACCEPTED`). |
| `status` | `VARCHAR(20)` | NOT NULL | `CREATED`, `MATCHING`, `ACCEPTED`, `PICKING_UP`, `IN_TRIP`, `COMPLETED`, `CANCELLED`, `EXPIRED`. |
| `pickup_lat` / `pickup_lng` | `DOUBLE PRECISION` | NOT NULL | Tọa độ điểm đón khách. |
| `pickup_address` | `TEXT` | NOT NULL | Địa chỉ điểm đón dạng chuỗi. |
| `dropoff_lat` / `dropoff_lng`| `DOUBLE PRECISION` | NOT NULL | Tọa độ điểm trả khách. |
| `dropoff_address` | `TEXT` | NOT NULL | Địa chỉ điểm trả dạng chuỗi. |
| `fare` | `BIGINT` | NOT NULL | Giá cước cố định đã chốt (VND, số nguyên). |
| `distance_m` | `INTEGER` | NOT NULL | Khoảng cách lộ trình tính bằng mét. |
| `surge_multiplier` | `NUMERIC(3,2)` | NOT NULL, DEFAULT `1.0` | Hệ số nhu cầu áp dụng lúc tạo cuốc. |
| `pickup_geohash5` | `VARCHAR(5)` | NOT NULL | Geohash độ dài 5 tính từ tọa độ đón (tính Demand). |
| `invited_drivers` | `JSONB` | NULLABLE | Danh sách mời Top 3–5: `[{"driver_id": "...", "distance_m": 120}]`. |
| `matching_expires_at` | `TIMESTAMPTZ` | NOT NULL | Hạn chót tìm xe lưu bền: `created_at + MATCHING_TIMEOUT_SECONDS` (mặc định 30s). |
| `completed_at` | `TIMESTAMPTZ` | NULLABLE | Mốc thời gian hoàn tất chuyến xe. |
| `cancelled_by` | `VARCHAR(10)` | NULLABLE | `CUSTOMER`, `DRIVER`, `ADMIN`. |
| `cancel_reason` | `TEXT` | NULLABLE | Lý do hủy cuốc hoặc lý do force-cancel. |
| `created_at` / `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian tạo và cập nhật bản ghi UTC. |

- **Chỉ mục & Ràng buộc toàn vẹn:** Các partial index tạo bằng SQL migration thô hoặc GORM tag where.
  - `idx_trips_active_customer`: `CREATE UNIQUE INDEX ... ON trips (customer_id) WHERE status IN ('CREATED', 'MATCHING', 'ACCEPTED', 'PICKING_UP', 'IN_TRIP')` (Ngăn chặn triệt để 2 chuyến active đồng thời của 1 khách).
  - `idx_trips_matching_timeout`: `CREATE INDEX ... ON trips (matching_expires_at) WHERE status = 'MATCHING'` (Tối ưu worker quét quá hạn 2s/lần).
  - `idx_trips_customer_lookup`: `CREATE INDEX ... ON trips (customer_id, status)` (Tra cứu chuyến hiện tại của khách).
  - `idx_trips_driver_active`: `CREATE INDEX ... ON trips (driver_id, status) WHERE status IN ('ACCEPTED', 'PICKING_UP', 'IN_TRIP')`.
  - `idx_trips_admin_list`: `CREATE INDEX ... ON trips (created_at DESC)` (Phân trang danh sách chuyến cho Admin).

### 2.2. Bảng `status_timeline` (Lịch sử trạng thái chuyến xe)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `BIGINT` | PK, GENERATED ALWAYS AS IDENTITY | Định danh dòng lịch sử tăng tự động. |
| `trip_id` | `UUID` | NOT NULL, FK `trips(id)` ON DELETE CASCADE | Chuyến xe tương ứng. |
| `status` | `VARCHAR(20)` | NOT NULL | Trạng thái ghi nhận tại mốc thời gian. |
| `note` | `TEXT` | NULLABLE | Ghi chú chuyển trạng thái (ví dụ `ADMIN_FORCE_CANCEL`, `TIMEOUT_30S`). |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Mốc thời gian chuyển trạng thái UTC. |

- **Chỉ mục:** `idx_timeline_trip_id`: `CREATE INDEX ... ON status_timeline (trip_id, created_at ASC)`.

---

## 3. API SPECIFICATION

Toàn bộ request/response sử dụng định dạng JSON thống nhất theo AGENTS.md. Header `X-User-Id` và `X-User-Role` do `api-gateway` xác thực và chuyển tiếp.

*Ghi chú đối tượng Trip:* Gồm các trường tương ứng bảng `trips` (trừ `invited_drivers`): `id` (uuid), `customer_id` (uuid), `driver_id` (uuid|null), `status` (string), `pickup_lat` (float), `pickup_lng` (float), `pickup_address` (string), `dropoff_lat` (float), `dropoff_lng` (float), `dropoff_address` (string), `fare` (int), `distance_m` (int), `surge_multiplier` (float), `pickup_geohash5` (string), `matching_expires_at` (iso8601), `completed_at` (iso8601|null), `cancelled_by` (string|null), `cancel_reason` (string|null), `created_at` (iso8601), `updated_at` (iso8601).

### 3.1. API Công khai (Public Endpoints)

| Method & Path | Tầng | Actor | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `POST /api/v1/trips` | 1 | `CUSTOMER` | `pickup_lat` (float, B)<br>`pickup_lng` (float, B)<br>`pickup_address` (string, B)<br>`dropoff_lat` (float, B)<br>`dropoff_lng` (float, B)<br>`dropoff_address` (string, B) | `id` (uuid)<br>`status` (string)<br>`fare` (int)<br>`pickup_address` (string)<br>`dropoff_address` (string)<br>`matching_expires_at` (iso8601) | `FORBIDDEN` (403: role != CUSTOMER)<br>`VALIDATION_ERROR` (400: tọa độ sai)<br>`INSUFFICIENT_BALANCE` (400: ví < fare)<br>`ACTIVE_TRIP_EXISTS` (409: đã có cuốc)<br>`SERVICE_UNAVAILABLE` (503: lỗi pricing/pay/location/user) |
| `POST /api/v1/trips/:id/accept` | 1 | `DRIVER` | Path: `id` (uuid, B) | `id` (uuid)<br>`status` ("ACCEPTED")<br>`driver_id` (uuid) | `FORBIDDEN` (403: role != DRIVER)<br>`NOT_OFFERED` (403: không trong Top 3-5)<br>`VALIDATION_ERROR` (400: id sai)<br>`DRIVER_NOT_AVAILABLE` (400: tài xế bận)<br>`TRIP_ALREADY_TAKEN` (409: thua lock)<br>`INVALID_TRIP_STATUS` (400: cuốc đã xong hoặc không MATCHING)<br>`SERVICE_UNAVAILABLE` (503: lỗi gọi user-service) |
| `POST /api/v1/trips/:id/picking-up` | 1 | `DRIVER` | Path: `id` (uuid, B) | `id` (uuid)<br>`status` ("PICKING_UP") | `FORBIDDEN` (403: role != DRIVER hoặc không phải tài xế cuốc)<br>`TRIP_NOT_FOUND` (404: không tìm thấy)<br>`INVALID_TRIP_STATUS` (400: status != ACCEPTED) |
| `POST /api/v1/trips/:id/start-trip` | 1 | `DRIVER` | Path: `id` (uuid, B) | `id` (uuid)<br>`status` ("IN_TRIP") | `FORBIDDEN` (403: role != DRIVER hoặc không phải tài xế cuốc)<br>`TRIP_NOT_FOUND` (404: không tìm thấy)<br>`INVALID_TRIP_STATUS` (400: status != PICKING_UP) |
| `POST /api/v1/trips/:id/complete` | 1 | `DRIVER` | Path: `id` (uuid, B) | `id` (uuid)<br>`status` ("COMPLETED")<br>`completed_at` (iso8601) | `FORBIDDEN` (403: role != DRIVER hoặc không phải tài xế cuốc)<br>`TRIP_NOT_FOUND` (404: không tìm thấy)<br>`INVALID_TRIP_STATUS` (400: status != IN_TRIP) |
| `POST /api/v1/trips/:id/cancel` | 1 | `CUSTOMER`<br>`DRIVER` | Path: `id` (uuid, B)<br>`reason` (string, T) | `id` (uuid)<br>`status` ("CANCELLED") | `FORBIDDEN` (403: không thuộc chuyến)<br>`TRIP_NOT_FOUND` (404: không tìm thấy)<br>`INVALID_TRIP_STATUS` (400: sai điều kiện hủy) |
| `GET /api/v1/trips/current` | 1 | `CUSTOMER`<br>`DRIVER` | *(None)* | `trip` (`Trip` \| null) | *(None - trả data: null nếu rỗng)* |
| `GET /api/v1/trips/:id` | 1 | `CUSTOMER`<br>`DRIVER` | Path: `id` (uuid, B) | `trip` (`Trip`)<br>`status_timeline` (array) | `FORBIDDEN` (403: không thuộc chuyến)<br>`TRIP_NOT_FOUND` (404: không tồn tại) |
| `POST /api/v1/admin/trips/:id/force-cancel` | 3 | `ADMIN` | Path: `id` (uuid, B)<br>`reason` (string, B) | `id` (uuid)<br>`status` ("CANCELLED")<br>`cancelled_by` ("ADMIN") | `FORBIDDEN` (403: role != ADMIN)<br>`TRIP_NOT_FOUND` (404: id không tồn tại)<br>`INVALID_TRIP_STATUS` (400: cuốc đã kết thúc) |
| `GET /api/v1/admin/trips` | 3 | `ADMIN` | Query: `status` (T), `page` (T), `limit` (T) | `trips` (array of `Trip`)<br>`total` (int)<br>`page` (int) | `FORBIDDEN` (403: role != ADMIN) |
| `GET /health` *(do LLD đặt)* | 1 | Tất cả | *(None)* | `status` ("ok") | *(None)* |

*(B: Bắt buộc, T: Tùy chọn)*

### 3.2. API Nội bộ (Internal Endpoints)
`dispatch-service` **không** cung cấp endpoint nội bộ nào cho service khác gọi. Service đóng vai trò Client gọi REST nội bộ sang 4 services phụ thuộc theo đúng Hợp đồng LLD-00.

---

## 4. LUỒNG XỬ LÝ TỪNG BƯỚC

### 4.1. Luồng Tạo chuyến & Phát sóng mời xe (UC-17, UC-18)
1. **Kiểm tra dữ liệu & Role:** Kiểm tra header `X-User-Role == 'CUSTOMER'` (sai $\rightarrow$ `FORBIDDEN` 403). Validate tọa độ hợp lệ.
2. **Lấy báo giá:** Gọi nội bộ `POST /internal/v1/pricing/estimate` lấy `fare`, `distance_m`, `surge_multiplier`. Nếu lỗi/timeout $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503).
3. **Kiểm tra ví:** Gọi nội bộ `POST /internal/v1/wallets/check-balance` với `user_id` và `required_amount = fare`. Nếu lỗi/timeout $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503). Nếu `sufficient = false` $\rightarrow$ Trả `INSUFFICIENT_BALANCE` (400).
4. **Tính Geohash:** Tính chuỗi `pickup_geohash5` chuẩn độ dài 5 từ tọa độ đón.
5. **Giao dịch DB INSERT chuyến:** Mở transaction DB:
   - Chèn bản ghi vào `trips` với `status = 'MATCHING'`, `matching_expires_at = now() + (MATCHING_TIMEOUT_SECONDS * interval '1 second')`, `fare` cố định, `pickup_geohash5`.
   - Nếu vi phạm partial index `idx_trips_active_customer` $\rightarrow$ Rollback DB, trả lỗi `ACTIVE_TRIP_EXISTS` (409).
   - Chèn 2 dòng vào `status_timeline` trong cùng transaction: dòng 1 `status = 'CREATED'`, tiếp theo dòng 2 `status = 'MATCHING'`. Commit transaction.
6. **Phát sự kiện tạo chuyến:** Ghi vào Redis Stream:
   `XADD stream:trip_events MAXLEN ~ 5000 * type TripCreated version "1.0" trip_id <id> customer_id <cid> pickup_geohash5 <geo5> created_at <utc>`.
   Nếu `XADD` lỗi $\rightarrow$ chỉ log `WARN` và tiếp tục luồng.
7. **Tìm kiếm & Ghép tài xế:**
   - Gọi `POST /internal/v1/locations/candidates` với `radius_km = OFFER_RADIUS_KM` (mặc định 5 km) lấy ~20 tài xế có GPS $\le 15\text{s}$.
   - Nếu `candidates` rỗng $\rightarrow$ coi như 0 tài xế (không gọi `filter-online`).
   - Nếu `candidates` không rỗng: trích xuất mảng `driver_ids`, gọi MỘT lần `POST /internal/v1/users/filter-online` để lọc tài xế đang `ONLINE`. Khớp lại với candidates để lấy `distance_m`, sắp xếp tăng dần theo `distance_m`, cắt lấy Top 3–5 ứng viên gần nhất.
   - **Xử lý lỗi phụ thuộc:** Nếu gọi `location-service` hoặc `user-service` bị lỗi/timeout (sau khi đã INSERT):
     - `UPDATE trips SET status = 'EXPIRED', updated_at = now() WHERE id = <trip_id> AND status = 'MATCHING'`.
     - Ghi `status_timeline` (`status = 'EXPIRED'`, `note = 'NO_DRIVERS_AVAILABLE'`).
     - `XADD stream:trip_events MAXLEN ~ 5000 * type TripExpired version "1.0" trip_id <id> reason "NO_DRIVERS_AVAILABLE" expired_at <utc>`.
     - `PUBLISH ride:trip_updates {"trip_id": "<id>", "customer_id": "<cid>", "driver_id": null, "status": "EXPIRED", "updated_at": "<utc>"}`.
     - Trả HTTP 503 `SERVICE_UNAVAILABLE`.
8. **Phân nhánh kết quả:**
   - **Nhánh không có tài xế (0 ONLINE, không lỗi):**
     - Cập nhật DB: `UPDATE trips SET status = 'EXPIRED', updated_at = now() WHERE id = <trip_id> AND status = 'MATCHING'`.
     - Ghi `status_timeline` (`status = 'EXPIRED'`, `note = 'NO_DRIVERS_AVAILABLE'`).
     - `XADD stream:trip_events MAXLEN ~ 5000 * type TripExpired version "1.0" trip_id <id> reason "NO_DRIVERS_AVAILABLE" expired_at <utc>`.
     - `PUBLISH ride:trip_updates {"trip_id": "<id>", "customer_id": "<cid>", "driver_id": null, "status": "EXPIRED", "updated_at": "<utc>"}`.
     - Trả về `201 Created` với `status = "EXPIRED"`.
   - **Nhánh có tài xế (Top 1..5):**
     - Lưu `invited_drivers` (dạng JSON `[{"driver_id": "...", "distance_m": ...}]`) vào `trips`.
     - Tính `commission = (fare * COMMISSION_RATE + 50) / 100`, suy ra `driver_fare = fare - commission` (chia số nguyên, khớp Mục 4 Hợp đồng LLD-00, dùng hiển thị).
     - Phát Redis Pub/Sub: `PUBLISH ride:trip_offers {"trip_id": "<id>", "targets": [{"driver_id":"...", "distance_m": ...}], "pickup_lat": ..., "pickup_lng": ..., "fare": ..., "driver_fare": ..., "expire_at": "<matching_expires_at>"}` (với `expire_at = matching_expires_at` của chuyến).
     - Trả về `201 Created` với `status = "MATCHING"`.

### 4.2. Luồng Nhận chuyến 4 bước cạnh tranh (UC-19)

```mermaid
sequenceDiagram
    autonumber
    actor Driver as 🚗 Driver
    participant Dispatch as dispatch-service
    participant Redis as Redis
    participant UserSvc as user-service
    participant DB as dispatchdb

    Driver->>Dispatch: POST /api/v1/trips/:id/accept
    Note over Dispatch: Kiểm tra vai trò: role == DRIVER (sai: 403 FORBIDDEN)
    Note over Dispatch: (1) Kiểm tra nhanh: driver trong invited_drivers? status == MATCHING?
    alt Không được mời / status sai
        Dispatch-->>Driver: 403 NOT_OFFERED / 409 TRIP_ALREADY_TAKEN / 400 INVALID_TRIP_STATUS
    end

    Note over Dispatch,Redis: (2) Khóa phân tán: SET ride:lock:<trip_id> <driver_id> NX EX 10
    Dispatch->>Redis: SET ride:lock:<trip_id> <driver_id> NX EX 10
    alt Thua lock (đã bị lock trước)
        Redis-->>Dispatch: nil
        Dispatch-->>Driver: 409 TRIP_ALREADY_TAKEN
    else Thắng lock
        Redis-->>Dispatch: OK

        Note over Dispatch,UserSvc: (3) Gọi nội bộ: POST /internal/v1/drivers/:id/status {from: ONLINE, to: BUSY}
        Dispatch->>UserSvc: Chuyển tài xế ONLINE -> BUSY
        alt Gọi user-service timeout / lỗi mạng
            Dispatch->>Redis: Lua compare-and-delete ride:lock:<trip_id>
            Dispatch-->>Driver: 503 SERVICE_UNAVAILABLE
        else Thất bại / Tài xế không ONLINE (400)
            UserSvc-->>Dispatch: 400 DRIVER_NOT_AVAILABLE
            Dispatch->>Redis: Lua compare-and-delete ride:lock:<trip_id>
            Dispatch-->>Driver: 400 DRIVER_NOT_AVAILABLE
        else Thành công chuyển BUSY (200)
            UserSvc-->>Dispatch: 200 OK

            Note over Dispatch,DB: (4) UPDATE trips SET status='ACCEPTED', driver_id=A WHERE id=trip_id AND status='MATCHING'
            Dispatch->>DB: UPDATE trips SET status='ACCEPTED', driver_id=A...
            alt 0 dòng affected (xung đột trạng thái)
                DB-->>Dispatch: 0 rows affected
                Dispatch->>UserSvc: Rollback: Chuyển tài xế BUSY -> ONLINE
                Dispatch->>Redis: Lua compare-and-delete ride:lock:<trip_id>
                Dispatch->>DB: SELECT status FROM trips WHERE id=trip_id
                Dispatch-->>Driver: 409 TRIP_ALREADY_TAKEN (nếu ACCEPTED) / 400 INVALID_TRIP_STATUS
            else 1 dòng affected (thành công)
                DB-->>Dispatch: 1 row affected
                Note over Dispatch: Best-effort sau commit: lỗi chỉ log ERROR
                Dispatch->>DB: INSERT status_timeline {trip_id, status: 'ACCEPTED'}
                Dispatch->>Redis: SET trip:link:<driver_id> '{"trip_id":"...","customer_id":"..."}' EX 10800
                Dispatch->>Redis: PUBLISH ride:trip_updates {"trip_id":"...","customer_id":"...","driver_id":"...","status":"ACCEPTED","updated_at":"<utc>"}
                Dispatch-->>Driver: 200 OK
            end
        end
    end
```

### 4.3. Luồng Tiến trình chuyến đi (UC-20)
- **Đón khách (`PICKING_UP`):**
  - Thực thi: `UPDATE trips SET status = 'PICKING_UP', updated_at = now() WHERE id = <trip_id> AND driver_id = <user_id> AND status = 'ACCEPTED'`.
  - Nếu 0 dòng affected: `SELECT` chuyến từ DB để phân biệt:
    - Nếu không tìm thấy chuyến $\rightarrow$ Trả `TRIP_NOT_FOUND` (404).
    - Nếu `driver_id != <user_id>` $\rightarrow$ Trả `FORBIDDEN` (403).
    - Nếu trạng thái khác `ACCEPTED` $\rightarrow$ Trả `INVALID_TRIP_STATUS` (400).
  - Nếu thành công (1 dòng): (1) Ghi `status_timeline`, (2) `PUBLISH ride:trip_updates {"trip_id":"...","customer_id":"...","driver_id":"...","status":"PICKING_UP","updated_at":"<utc>"}`, trả 200 OK (lỗi bước phụ log ERROR).
- **Bắt đầu chở (`IN_TRIP`):**
  - Thực thi: `UPDATE trips SET status = 'IN_TRIP', updated_at = now() WHERE id = <trip_id> AND driver_id = <user_id> AND status = 'PICKING_UP'`.
  - Nếu 0 dòng affected: `SELECT` chuyến từ DB để phân biệt:
    - Nếu không tìm thấy chuyến $\rightarrow$ Trả `TRIP_NOT_FOUND` (404).
    - Nếu `driver_id != <user_id>` $\rightarrow$ Trả `FORBIDDEN` (403).
    - Nếu trạng thái khác `PICKING_UP` $\rightarrow$ Trả `INVALID_TRIP_STATUS` (400).
  - Nếu thành công (1 dòng): (1) Ghi `status_timeline`, (2) `PUBLISH ride:trip_updates {"trip_id":"...","customer_id":"...","driver_id":"...","status":"IN_TRIP","updated_at":"<utc>"}`, trả 200 OK (lỗi bước phụ log ERROR).
- **Hoàn thành (`COMPLETED`):**
  - Thực thi: `UPDATE trips SET status = 'COMPLETED', completed_at = now(), updated_at = now() WHERE id = <trip_id> AND driver_id = <user_id> AND status = 'IN_TRIP'`.
  - Nếu 0 dòng affected: `SELECT` chuyến từ DB để phân biệt:
    - Nếu không tìm thấy chuyến $\rightarrow$ Trả `TRIP_NOT_FOUND` (404).
    - Nếu `driver_id != <user_id>` $\rightarrow$ Trả `FORBIDDEN` (403).
    - Nếu trạng thái khác `IN_TRIP` $\rightarrow$ Trả `INVALID_TRIP_STATUS` (400).
  - Nếu thành công (1 dòng), thực hiện tuần tự theo quy tắc best-effort sau commit (bước phụ lỗi chỉ log `ERROR` và vẫn trả 200 OK):
    1. Ghi dòng `COMPLETED` vào `status_timeline`.
    2. `XADD stream:trip_events MAXLEN ~ 5000 * type TripCompleted version "1.0" trip_id <id> customer_id <cid> driver_id <did> fare <fare> completed_at <utc>` (*tuyệt đối không gửi commission*).
    3. `DEL trip:link:<driver_id>`.
    4. Gọi nội bộ `user-service`: `POST /internal/v1/drivers/:id/status` với `from_status: "BUSY", to_status: "ONLINE"` để giải phóng tài xế.
    5. `PUBLISH ride:trip_updates {"trip_id":"...","customer_id":"...","driver_id":"...","status":"COMPLETED","updated_at":"<utc>"}`.
    6. Trả 200 OK.

### 4.4. Luồng Hủy chuyến & Admin Force-Cancel (UC-21, UC-30)
- **Khách hàng hủy:** `UPDATE trips SET status = 'CANCELLED', cancelled_by = 'CUSTOMER', cancel_reason = <reason>, updated_at = now() WHERE id = <trip_id> AND customer_id = <user_id> AND status IN ('MATCHING', 'ACCEPTED') RETURNING driver_id, customer_id;`
- **Tài xế hủy:** `UPDATE trips SET status = 'CANCELLED', cancelled_by = 'DRIVER', cancel_reason = <reason>, updated_at = now() WHERE id = <trip_id> AND driver_id = <user_id> AND status IN ('ACCEPTED', 'PICKING_UP') RETURNING driver_id, customer_id;`
- **Admin Force-Cancel:** `UPDATE trips SET status = 'CANCELLED', cancelled_by = 'ADMIN', cancel_reason = <reason>, updated_at = now() WHERE id = <trip_id> AND status IN ('CREATED', 'MATCHING', 'ACCEPTED', 'PICKING_UP', 'IN_TRIP') RETURNING driver_id, customer_id;`
- **Nguyên tắc thực thi:** Câu UPDATE được chọn theo `X-User-Role` (`CUSTOMER` dùng câu khách hủy, `DRIVER` dùng câu tài xế hủy, `ADMIN` dùng câu force-cancel). Dùng `driver_id` trả về để biết chuyến có tài xế (khác null thì `DEL trip:link:<driver_id>` và gọi `user-service` đổi `BUSY → ONLINE`), dùng `customer_id` trả về cho bản tin `PUBLISH ride:trip_updates`.
- **Xử lý khi UPDATE trả về 0 dòng affected:**
  Thực hiện `SELECT` chuyến từ DB để phân biệt:
  - Nếu không tìm thấy chuyến $\rightarrow$ Trả `TRIP_NOT_FOUND` (404).
  - Nếu người gọi không thuộc chuyến $\rightarrow$ Trả `FORBIDDEN` (403) (chỉ áp dụng cho Khách hàng / Tài xế; KHÔNG áp dụng cho Admin force-cancel vì Admin chỉ có `TRIP_NOT_FOUND` hoặc `INVALID_TRIP_STATUS`).
  - Nếu sai trạng thái hủy (chuyến đã kết thúc `COMPLETED`, `CANCELLED`, `EXPIRED`,...) $\rightarrow$ Trả `INVALID_TRIP_STATUS` (400).
- **Xử lý sau hủy thành công (1 dòng affected, best-effort sau commit):**
  1. Ghi `status_timeline` (`status = 'CANCELLED'`).
  2. Dựa vào `driver_id` trả về: nếu khác null $\rightarrow$ `DEL trip:link:<driver_id>`, và gọi `user-service`: `POST /internal/v1/drivers/:id/status` (`from_status: "BUSY", to_status: "ONLINE"`).
  3. `XADD stream:trip_events MAXLEN ~ 5000 * type TripCancelled version "1.0" trip_id <id> cancelled_at <utc>` (*tuyệt đối không gửi cancelled_by*).
  4. `PUBLISH ride:trip_updates {"trip_id":"...","customer_id":<customer_id>,"driver_id":<driver_id_hoặc_null>,"status":"CANCELLED","updated_at":"<utc>"}`.
  5. Trả 200 OK.

### 4.5. Tiến trình Quét chuyến Quá hạn (Background Sweeper Worker)
- **Khởi chạy:** Chạy trong 1 goroutine nền độc lập với `time.NewTicker(2 * time.Second)`, lắng nghe `ctx.Done()` để shutdown êm.
- **Logic xử lý mỗi chu kỳ (tái sử dụng hàm quét quá hạn):**
  1. Thực thi duy nhất 1 câu SQL nguyên tử:
     `UPDATE trips SET status = 'EXPIRED', updated_at = now() WHERE status = 'MATCHING' AND matching_expires_at <= now() RETURNING id, customer_id;`
  2. Duyệt từng dòng trả về:
     - Ghi nhận `status_timeline` (`status = 'EXPIRED'`, `note = 'TIMEOUT_30S'`).
     - `XADD stream:trip_events MAXLEN ~ 5000 * type TripExpired version "1.0" trip_id <id> reason "TIMEOUT_30S" expired_at <utc>`.
     - `PUBLISH ride:trip_updates {"trip_id": "<id>", "customer_id": "<cid>", "driver_id": null, "status": "EXPIRED", "updated_at": "<utc>"}`.

---

## 5. XỬ LÝ NGOẠI LỆ (EXCEPTION HANDLING)

| Tình huống ngoại lệ | Ngữ cảnh phát sinh | Hành vi xử lý & Mã lỗi |
| :--- | :--- | :--- |
| Đặt trùng chuyến | Khách bấm đặt xe 2 lần đồng thời | Vi phạm Partial Unique Index `idx_trips_active_customer` $\rightarrow$ Rollback DB, trả `ACTIVE_TRIP_EXISTS` (409). |
| Thiếu tiền ví | Khách đặt xe khi ví < fare | `check-balance` trả `sufficient = false` $\rightarrow$ Chặn đặt, trả `INSUFFICIENT_BALANCE` (400). |
| Service phụ thuộc lỗi | `pricing`, `payment`, `location`, `user` timeout/mất mạng | Không retry vô hạn $\rightarrow$ Trả `SERVICE_UNAVAILABLE` (503). Khi accept gọi user-service lỗi/timeout $\rightarrow$ nhả lock Redis và trả 503. Khi tạo chuyến lỗi location/user sau khi INSERT $\rightarrow$ chuyển EXPIRED, phát event/WS và trả 503. |
| Thua cuộc đua nhận cuốc | 2 tài xế cùng bấm Accept 1 chuyến | Tài xế đến sau nhận `nil` từ Redis `SET NX` $\rightarrow$ Trả `TRIP_ALREADY_TAKEN` (409). |
| Tài xế không được mời | Tài xế bấm accept chuyến mình không có trong invited_drivers | `NOT_OFFERED` (403). |
| Tài xế bấm nhận khi đã bận | Tài xế nhận cuốc khác trước đó | `user-service` trả lỗi chuyển trạng thái $\rightarrow$ Nhả lock Redis, trả `DRIVER_NOT_AVAILABLE` (400). |
| Trạng thái nhảy cóc | Gọi complete khi đang PICKING_UP | Lệnh `UPDATE ... WHERE status = ...` trả về 0 rows $\rightarrow$ SELECT phân loại và trả `INVALID_TRIP_STATUS` (400). |

### 5.1. Hạn chế đã biết
- **Crash giữa UPDATE DB và XADD Stream:** Có thể làm mất sự kiện `TripCompleted` dẫn đến luồng thanh toán không tự động chạy. Đây là trade-off chấp nhận được cho đồ án demo nhằm giữ kiến trúc tinh gọn, không triển khai Transactional Outbox pattern.
- **Tài xế kẹt BUSY (Orphaned BUSY Driver):** Nếu dispatch sập nguồn giữa lúc đổi tài xế `BUSY` và `UPDATE` DB hoặc nhả lock, tài xế kẹt xử lý bằng can thiệp thủ công theo quy trình vận hành tại Mục 5(f) Hợp đồng LLD-00.

---

## 6. SỰ KIỆN & KHÓA REDIS

### 6.1. Khóa phân tán & Dữ liệu tạm (Keys)
- `ride:lock:<trip_id>`: String, TTL 10s. Khóa nguyên tử tranh cuốc `SET ride:lock:<trip_id> <driver_id> NX EX 10`. Giải phóng bằng Lua Script compare-and-delete:
  `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`.
- `trip:link:<driver_id>`: String, TTL 3h (`10800s`). Giá trị: `{"trip_id": "<id>", "customer_id": "<cid>"}`. Ghi khi `ACCEPTED`; xóa bằng lệnh `DEL trip:link:<driver_id>` khi cuốc kết thúc (`COMPLETED`, `CANCELLED`, Admin force-cancel).

### 6.2. Kênh Pub/Sub phát đi
- `ride:trip_offers`: Phát lời mời cuốc Top 3–5. Payload:
  `{"trip_id": "<id>", "targets": [{"driver_id":"...", "distance_m": 150}], "pickup_lat": 10.7, "pickup_lng": 106.6, "fare": 30000, "driver_fare": 25500, "expire_at": "<matching_expires_at>"}` (với `expire_at = matching_expires_at` của chuyến).
- `ride:trip_updates`: Phát cập nhật tiến trình cuốc. Payload:
  `{"trip_id": "<id>", "customer_id": "<cid>", "driver_id": "<did_hoặc_null>", "status": "<STATUS>", "updated_at": "<utc>"}`.

### 6.3. Sự kiện Redis Stream phát đi (`stream:trip_events`)
- `TripCreated`: `type` ("TripCreated"), `version="1.0"`, `trip_id`, `customer_id`, `pickup_geohash5`, `created_at`.
- `TripCompleted`: `type` ("TripCompleted"), `version="1.0"`, `trip_id`, `customer_id`, `driver_id`, `fare`, `completed_at` *(không có commission)*.
- `TripCancelled`: `type` ("TripCancelled"), `version="1.0"`, `trip_id`, `cancelled_at` *(không có cancelled_by)*.
- `TripExpired`: `type` ("TripExpired"), `version="1.0"`, `trip_id`, `reason` (`NO_DRIVERS_AVAILABLE` | `TIMEOUT_30S`), `expired_at`.

---

## 7. CẤU HÌNH (ENV) VÀ TÀI NGUYÊN

### 7.1. Bảng biến môi trường
| Tên biến ENV | Mặc định | Ý nghĩa & Mục đích sử dụng |
| :--- | :--- | :--- |
| `PORT` | `8003` | Cổng HTTP nội bộ của `dispatch-service` trong mạng Docker. |
| `DATABASE_URL` | `postgres://dispatch_user:dispatch_pass@postgres:5432/dispatchdb?sslmode=disable` | Chuỗi kết nối cơ sở dữ liệu `dispatchdb`. |
| `REDIS_ADDR` | `redis:6379` | Địa chỉ Redis container. |
| `REDIS_PASSWORD` | `redis_secret_pass` | Mật khẩu Redis container (demo, ghi đè trong .env). |
| `COMMISSION_RATE` | `15` | Phần trăm hoa hồng (nguyên) dùng để tính hiển thị `driver_fare` trong `ride:trip_offers`. |
| `OFFER_RADIUS_KM` | `5` | Bán kính tìm tài xế lân cận (km). |
| `MATCHING_TIMEOUT_SECONDS` | `30` | Thời hạn tìm tài xế ghép chuyến (giây). |
| `INTERNAL_HTTP_TIMEOUT_MS` | `2000` | Thời gian chờ tối đa khi gọi REST nội bộ giữa các service (ms). |
| `PRICING_SERVICE_URL` | `http://pricing-service:8004` | Base URL gọi nội bộ `pricing-service`. |
| `PAYMENT_SERVICE_URL` | `http://payment-service:8005` | Base URL gọi nội bộ `payment-service`. |
| `LOCATION_SERVICE_URL`| `http://location-service:8002`| Base URL gọi nội bộ `location-service`. |
| `USER_SERVICE_URL` | `http://user-service:8001` | Base URL gọi nội bộ `user-service`. |

### 7.2. Tài nguyên & Khởi động hệ thống
- **Connection Pool PostgreSQL (Mục 5(h) LLD-00):** `MaxOpenConns = 10`, `MaxIdleConns = 3`, `ConnMaxLifetime = 30m`.
- **Ràng buộc bộ nhớ VPS 1GB:** `mem_limit` Docker đề xuất: 30 MB; cấu hình `GOMEMLIMIT=26MiB` (kích hoạt GC sớm chống OOM).
- **Thứ tự khởi động dịch vụ (Startup Sequence):**
  1. Kết nối PostgreSQL `dispatchdb` và thực thi migration bảng `trips`, `status_timeline`.
  2. Kết nối Redis, kiểm tra ping `PONG`.
  3. Khởi chạy quét dọn quá hạn lần đầu: Tái sử dụng cùng hàm xử lý với Mục 4.5: thực thi `UPDATE ... RETURNING`, ghi `status_timeline`, `XADD TripExpired` và `PUBLISH ride:trip_updates` cho toàn bộ cuốc `MATCHING` đã quá hạn.
  4. Khởi chạy goroutine background sweeper worker lặp lại chu kỳ 2 giây.
  5. Khởi động HTTP web server (Fiber) nhận request từ Gateway.
