# PHÂN HỆ TIẾN TRÌNH CHUYẾN ĐI & HỦY CHUYẾN - USE CASE ĐẶC TẢ

> **Mã tài liệu:** `UC-20` đến `UC-23`  
> **Dịch vụ chịu trách nhiệm:** `dispatch-service`  
> **Tài liệu tham chiếu:** [SRS v1.4 (Mục 3.4, Mục 4)](../01-srs/srs.md), [quyet-dinh.md](../00-brainstorm/quyet-dinh.md)

---

## 1. SƠ ĐỒ TUẦN TỰ HOÀN THÀNH CHUYẾN ĐI (SEQUENCE DIAGRAM)

```mermaid
sequenceDiagram
    autonumber
    actor Driver as 🚗 Tài xế
    participant Gateway as api-gateway
    participant DispatchSvc as dispatch-service
    participant UserSvc as user-service
    participant RedisStreams as Redis Streams (stream:trip_events)
    participant RedisPubSub as Redis Pub/Sub (ride:trip_updates)
    participant DB as dispatchdb

    Driver->>Gateway: POST /api/v1/trips/:id/complete
    Gateway->>DispatchSvc: Forward request kèm X-User-Id
    
    DispatchSvc->>DB: Truy vấn chuyến xe & kiểm tra trạng thái
    alt Trạng thái chuyến khác IN_TRIP hoặc không phải tài xế của chuyến
        DispatchSvc-->>Gateway: Lỗi 400 INVALID_TRIP_STATUS / 403 FORBIDDEN
        Gateway-->>Driver: Lỗi trạng thái không hợp lệ
    else Hợp lệ (Status == IN_TRIP)
        DispatchSvc->>DB: UPDATE trips SET status = 'COMPLETED', completed_at = NOW()
        DispatchSvc->>DB: INSERT status_timeline {trip_id, status: 'COMPLETED', timestamp: NOW()}
        
        DispatchSvc->>UserSvc: Cập nhật tài xế từ BUSY quay lại ONLINE
        
        DispatchSvc->>RedisStreams: XADD stream:trip_events {type: "TripCompleted", trip_id, customer_id, driver_id, fare, timestamp}
        DispatchSvc->>RedisPubSub: PUBLISH ride:trip_updates {trip_id, status: "COMPLETED"}
        
        DispatchSvc-->>Gateway: 200 OK (Chuyến đi hoàn tất)
        Gateway-->>Driver: 200 OK (Chúc mừng bạn đã hoàn thành chuyến!)
    end
```

---

## 2. CHI TIẾT TỪNG USE CASE

### UC-20: Cập nhật tiến trình chuyến đi (Trip State Progression)
* **FR liên quan:** `FR-19`
* **Actor:** Tài xế (Driver).
* **Tiền điều kiện:** Chuyến xe đã được tài xế nhận (`ACCEPTED`) và tài xế đang thực hiện cuốc xe.
* **Luồng chính:**
  1. Tài xế di chuyển tới điểm đón và bấm cập nhật: `ACCEPTED` $\rightarrow$ `PICKING_UP` (`POST /api/v1/trips/:id/picking-up`).
  2. Hệ thống ghi nhận trạng thái `PICKING_UP` và lưu mốc thời gian vào bảng lịch sử `status_timeline`. Phát thông báo qua WebSocket cho khách hàng.
  3. Khi đón được khách lên xe, tài xế bấm bắt đầu chuyến: `PICKING_UP` $\rightarrow$ `IN_TRIP` (`POST /api/v1/trips/:id/start-trip`).
  4. Hệ thống cập nhật trạng thái `IN_TRIP`, lưu mốc thời gian vào `status_timeline`. Phát thông báo qua WebSocket.
  5. Khi tới điểm trả, tài xế bấm hoàn thành cuốc: `IN_TRIP` $\rightarrow$ `COMPLETED` (`POST /api/v1/trips/:id/complete`).
  6. Hệ thống thực hiện:
     - Đổi trạng thái sang `COMPLETED`.
     - Ghi mốc thời gian hoàn thành vào `status_timeline`.
     - Gọi `user-service` đưa tài xế từ `BUSY` quay trở lại trạng thái `ONLINE` để tiếp tục đón khách mới.
     - Phát sự kiện `TripCompleted` vào **Redis Streams** (`stream:trip_events`) mang theo đầy đủ thông tin: `trip_id`, `customer_id`, `driver_id`, giá cước đã chốt (`fare`).
     - Phát event qua Redis Pub/Sub để `ws-gateway` đẩy thông báo hoàn tất xuống cho cả khách hàng và tài xế.
* **Luồng ngoại lệ:**
  - *Chuyển trạng thái nhảy cóc (ví dụ từ `ACCEPTED` nhảy thẳng lên `IN_TRIP` hoặc `COMPLETED`):* Bị chặn với mã lỗi `INVALID_TRIP_STATUS` (HTTP 400).
  - *Tài xế khác không phải người nhận chuyến cố tình cập nhật:* Bị từ chối với lỗi `FORBIDDEN` (HTTP 403).
* **Hậu điều kiện:** Máy trạng thái chuyến xe được tuân thủ nghiêm ngặt; sự kiện `TripCompleted` sẵn sàng cho hệ thống thanh toán và phân tích xử lý tiếp.
* **Dữ liệu demo cần thấy trên Postman:** `trip_id`, trạng thái hiện tại (`PICKING_UP`, `IN_TRIP`, `COMPLETED`), thời gian cập nhật.

---

### UC-21: Hủy chuyến xe hợp lệ
* **FR liên quan:** `FR-20`, `FR-25`
* **Actor:** Khách hàng (Customer), Tài xế (Driver).
* **Tiền điều kiện:** Chuyến xe đang ở một trong các trạng thái có thể hủy hợp lệ.
* **Luồng chính:**
  1. Người dùng gửi yêu cầu hủy chuyến (`POST /api/v1/trips/:id/cancel`).
  2. `dispatch-service` kiểm tra vai trò người gọi và trạng thái hiện tại của chuyến:
     - **Nếu là Khách hàng:** Được phép hủy khi chuyến đang ở trạng thái `MATCHING` (đang tìm tài xế) hoặc `ACCEPTED` (đã có tài xế nhận nhưng chưa bắt đầu di chuyển đón).
     - **Nếu là Tài xế:** Được phép hủy khi chuyến đang ở trạng thái `ACCEPTED` hoặc `PICKING_UP` (gặp sự cố phương tiện hoặc không thể tiếp cận điểm đón).
  3. Nếu điều kiện hợp lệ:
     - Cập nhật trạng thái chuyến xe thành `CANCELLED`.
     - Ghi nhận mốc thời gian hủy vào `status_timeline`.
     - Nếu chuyến đã có tài xế gán vào (`ACCEPTED` hoặc `PICKING_UP`): Gọi `user-service` đưa tài xế từ `BUSY` quay trở lại `ONLINE`.
     - Giải phóng khóa tài xế nếu có.
     - **Chính sách không phạt (FR-25):** Không trừ bất kỳ khoản phí phạt nào từ ví của khách hàng hay tài xế.
     - Bắn sự kiện `TripCancelled` vào Redis Streams (`stream:trip_events`) và Redis Pub/Sub (`ride:trip_updates`).
     - Trả về thông báo hủy chuyến thành công.
* **Luồng ngoại lệ (Cố tình hủy sai trạng thái):**
  - *Khách hàng cố tình hủy khi tài xế đang đến đón (`PICKING_UP`):* Từ chối với lỗi `INVALID_TRIP_STATUS` (HTTP 400).
  - *Khách hàng hoặc Tài xế cố tình hủy khi xe đang chạy (`IN_TRIP`):* Bị chặn tuyệt đối với mã lỗi `INVALID_TRIP_STATUS` (HTTP 400).
* **Hậu điều kiện:** Chuyến xe kết thúc ở trạng thái `CANCELLED`; tài xế được giải phóng sang `ONLINE`; không phát sinh trừ tiền ví.
* **Dữ liệu demo cần thấy trên Postman:** `trip_id`, trạng thái mới `CANCELLED`, người thực hiện hủy (`cancelled_by`), thời gian hủy.

---

### UC-22: Xử lý hết giờ tìm xe / Không có tài xế (Trip Expired)
* **FR liên quan:** `FR-17`, `FR-21`
* **Actor:** Hệ thống (`dispatch-service`).
* **Tiền điều kiện:** Chuyến xe đang ở trạng thái `MATCHING`.
* **Luồng chính:**
  - **Trường hợp A (Quét ra 0 tài xế ngay lúc đặt xe):**
    1. Khi khách vừa tạo chuyến, `location-service` quét bán kính 5km trả về 0 tài xế rảnh.
    2. `dispatch-service` cập nhật ngay trạng thái chuyến xe thành `EXPIRED` mà không cần chờ 30 giây.
  - **Trường hợp B (Hết 30 giây không tài xế nào bấm nhận):**
    1. Bộ đếm thời gian 30 giây của chuyến xe hết hạn mà trạng thái vẫn là `MATCHING`.
    2. `dispatch-service` cập nhật trạng thái chuyến xe thành `EXPIRED`.
  - **Hành động chung:**
    3. Ghi mốc thời gian hết giờ vào `status_timeline`.
    4. Xóa các đề nghị mời cuốc liên quan.
    5. Phát sự kiện `TripExpired` vào Redis Streams (`stream:trip_events`) để `ai-service` ghi nhận thống kê tỷ lệ hết giờ.
    6. Phát thông báo qua Redis Pub/Sub để đẩy xuống WebSocket của khách hàng thông báo không tìm thấy tài xế.
* **Hậu điều kiện:** Chuyến xe kết thúc ở trạng thái `EXPIRED`; khách hàng được giải phóng để có thể tạo chuyến mới.
* **Dữ liệu demo cần thấy trên Postman/WS:** `trip_id`, trạng thái `EXPIRED`, lý do hết hạn (`NO_DRIVERS_AVAILABLE` hoặc `TIMEOUT_30S`).

---

### UC-23: Xem chuyến hiện tại & Chi tiết lịch sử chuyến
* **FR liên quan:** `FR-36`
* **Actor:** Khách hàng, Tài xế.
* **Tiền điều kiện:** Người dùng đã đăng nhập.
* **Luồng chính:**
  - **Kịch bản 1: Xem chuyến đang hoạt động (`GET /api/v1/trips/current`):**
    1. Client gửi request tra cứu chuyến xe hiện tại.
    2. `dispatch-service` tìm chuyến xe thuộc sở hữu của người dùng có trạng thái thuộc tập: `CREATED`, `MATCHING`, `ACCEPTED`, `PICKING_UP`, `IN_TRIP`.
    3. Nếu có: Trả về thông tin chuyến xe đang chạy kèm trạng thái hiện tại.
    4. Nếu không có: Trả về `data: null`.
  - **Kịch bản 2: Xem chi tiết một chuyến xe (`GET /api/v1/trips/:id`):**
    1. Client gửi request kèm mã `trip_id`.
    2. `dispatch-service` kiểm tra quyền: Chuyến xe phải thuộc về `customer_id` hoặc `driver_id` của người gọi (hoặc Admin).
    3. Trả về toàn bộ thông tin chi tiết của chuyến xe cùng mảng **`status_timeline`** gồm danh sách các trạng thái mà chuyến đã đi qua kèm mốc thời gian chính xác từng giây.
* **Luồng ngoại lệ:**
  - *Xem chuyến không thuộc quyền sở hữu của mình:* Trả về lỗi `FORBIDDEN` (HTTP 403).
  - *Mã chuyến không tồn tại:* Trả về lỗi `TRIP_NOT_FOUND` (HTTP 404).
* **Hậu điều kiện:** Thông tin chuyến xe và lịch sử diễn biến được hiển thị đầy đủ và minh bạch.
* **Dữ liệu demo cần thấy trên Postman:**
  - Thông tin chuyến: `id`, `status`, `fare`, `pickup_address`, `dropoff_address`.
  - Mảng lịch sử `status_timeline`:
    `[{status: "CREATED", time: "..."}, {status: "MATCHING", time: "..."}, {status: "ACCEPTED", time: "..."}, {status: "PICKING_UP", time: "..."}, {status: "IN_TRIP", time: "..."}, {status: "COMPLETED", time: "..."}]`.
