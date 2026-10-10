# PHÂN HỆ ĐẶT XE & GHÉP CHUYẾN - USE CASE ĐẶC TẢ

> **Mã tài liệu:** `UC-14` đến `UC-19`  
> **Dịch vụ chịu trách nhiệm:** `pricing-service`, `dispatch-service`, `payment-service`, `location-service`  
> **Tài liệu tham chiếu:** [SRS v1.5 (Mục 3.3, 3.4)](../01-srs/srs.md), [quyet-dinh.md](../00-brainstorm/quyet-dinh.md)  
> **Phiên bản:** 1.1

---

## 1. SƠ ĐỒ TUẦN TỰ ĐẶT XE & TRANH CUỐC (SEQUENCE DIAGRAMS)

### 1.1. Luồng Đặt xe, Kiểm tra ví & Khởi tạo chuyến
```mermaid
sequenceDiagram
    autonumber
    actor Customer as 🧑 Khách hàng
    participant Gateway as api-gateway
    participant DispatchSvc as dispatch-service
    participant PaymentSvc as payment-service
    participant PricingSvc as pricing-service
    participant LocationSvc as location-service
    participant UserSvc as user-service
    participant Redis as Redis (Streams & Pub/Sub)
    participant DB as dispatchdb

    Customer->>Gateway: POST /api/v1/trips (pickup_lat, pickup_lng, dropoff_lat, dropoff_lng)
    Gateway->>DispatchSvc: Forward request kèm X-User-Id
    
    DispatchSvc->>DB: Kiểm tra chuyến đang active của khách hàng
    alt Đã có chuyến đang chạy (CREATED, MATCHING, ACCEPTED, PICKING_UP, IN_TRIP)
        DispatchSvc-->>Gateway: Lỗi 409 ACTIVE_TRIP_EXISTS
        Gateway-->>Customer: 409 Conflict (Mỗi khách chỉ được có 1 chuyến active)
    else Chưa có chuyến active
        DispatchSvc->>PricingSvc: Lấy báo giá ước tính (nội bộ, không qua api-gateway) {pickup, dropoff}
        PricingSvc-->>DispatchSvc: Trả về {fare, distance, eta, source: OSRM/HAVERSINE}
        
        DispatchSvc->>PaymentSvc: Kiểm tra số dư ví khách (nội bộ, không qua api-gateway)
        PaymentSvc-->>DispatchSvc: Trả về {balance}
        
        alt Số dư ví < fare tạm tính
            DispatchSvc-->>Gateway: Lỗi 400 INSUFFICIENT_BALANCE
            Gateway-->>Customer: 400 Bad Request (Ví không đủ tiền, chặn đặt xe)
        else Số dư ví đủ
            DispatchSvc->>DB: Giao dịch DB: INSERT chuyến (status: CREATED -> MATCHING, fare cố định, timeout 30s)
            DispatchSvc->>Redis: XADD stream:trip_events {type: TripCreated, trip_id, ...}
            
            DispatchSvc->>LocationSvc: Quét tối đa 50 ứng viên có GPS < 15s (nội bộ, không qua api-gateway)
            LocationSvc-->>DispatchSvc: Trả về tối đa 50 drivers kèm distance_m
            DispatchSvc->>UserSvc: Lọc tài xế ONLINE hàng loạt (nội bộ, không qua api-gateway)
            UserSvc-->>DispatchSvc: Trả về danh sách tài xế ONLINE
            
            alt Quét ra 0 tài xế ONLINE
                DispatchSvc->>DB: Cập nhật status = EXPIRED ngay lập tức
                DispatchSvc->>Redis: XADD stream:trip_events {type: TripExpired, trip_id}
                DispatchSvc-->>Gateway: 201 Created (status: EXPIRED)
                Gateway-->>Customer: 201 Created (status: EXPIRED - Không tìm thấy tài xế)
            else Có tài xế ONLINE (Cắt Top 3-5)
                DispatchSvc->>DB: Lưu danh sách tài xế được mời (Top 3-5) vào chuyến
                DispatchSvc->>Redis: PUBLISH ride:trip_offers {trip_id, target_driver_ids, pickup, fare, distance_m}
                Note over DispatchSvc,Redis: Hạn tìm xe 30 giây đã được lưu bền (UC-17); tiến trình quét nền (UC-22) chuyển chuyến quá hạn sang EXPIRED
                DispatchSvc-->>Gateway: 201 Created (status: MATCHING)
                Gateway-->>Customer: 201 Created (Đang tìm tài xế gần bạn...)
            end
        end
    end
```

### 1.2. Luồng Tranh chấp nhận chuyến cạnh tranh (Redis Atomic Lock)
```mermaid
sequenceDiagram
    autonumber
    actor DriverA as 🚗 Tài xế A (Thắng lock)
    actor DriverB as 🚗 Tài xế B (Thua lock)
    participant Gateway as api-gateway
    participant DispatchSvc as dispatch-service
    participant Redis as Redis (Atomic Lock & Pub/Sub)
    participant UserSvc as user-service
    participant DB as dispatchdb

    par Cả 2 tài xế cùng bấm Accept
        DriverA->>Gateway: POST /api/v1/trips/:id/accept
        DriverB->>Gateway: POST /api/v1/trips/:id/accept
    end

    Gateway->>DispatchSvc: Forward request Driver A kèm X-User-Id
    Gateway->>DispatchSvc: Forward request Driver B kèm X-User-Id

    Note over DispatchSvc: (1) Kiểm tra nhanh: Tài xế có trong danh sách mời? Chuyến còn MATCHING?

    critical (2) Khóa nguyên tử phân tán Redis
        DispatchSvc->>Redis: SET ride:lock:<trip_id> <driver_A_id> NX EX 10
        Redis-->>DispatchSvc: OK (Driver A thắng lock!)

        Note over DispatchSvc,UserSvc: (3) Chuyển tài xế ONLINE -> BUSY có điều kiện (nội bộ, không qua api-gateway)
        DispatchSvc->>UserSvc: Chuyển tài xế ONLINE -> BUSY có điều kiện (nội bộ, không qua api-gateway)
        UserSvc-->>DispatchSvc: 200 OK (Thành công)

        Note over DispatchSvc,DB: (4) UPDATE trips SET status = 'ACCEPTED' WHERE status = 'MATCHING'
        DispatchSvc->>DB: UPDATE trips SET status = 'ACCEPTED', driver_id = A WHERE id = trip_id AND status = 'MATCHING'
        DB-->>DispatchSvc: 1 row affected (Thành công)

        DispatchSvc->>Redis: PUBLISH ride:trip_updates {trip_id, status: ACCEPTED, driver_id: A}
        DispatchSvc-->>Gateway: 200 OK (Nhận chuyến thành công)
        Gateway-->>DriverA: 200 OK (Chuyến đã được gán cho bạn)
    option Driver B thực hiện lock sau đó (Thua cuộc)
        DispatchSvc->>Redis: SET ride:lock:<trip_id> <driver_B_id> NX EX 10
        Redis-->>DispatchSvc: nil (Lock đã tồn tại)
        DispatchSvc-->>Gateway: Lỗi 409 TRIP_ALREADY_TAKEN
        Gateway-->>DriverB: 409 Conflict (Chuyến đã có người nhận)
    end
    Note over DispatchSvc,UserSvc: Hoàn tác nếu lỗi: Bước 3 lỗi -> Nhả lock Redis (DRIVER_NOT_AVAILABLE);\nBước 4 lỗi (0 dòng) -> Rollback BUSY->ONLINE, nhả lock, đọc lại trạng thái chuyến: ACCEPTED -> TRIP_ALREADY_TAKEN; EXPIRED/CANCELLED -> INVALID_TRIP_STATUS
```

---

## 2. CHI TIẾT TỪNG USE CASE

### UC-14: Ước tính cước phí & khoảng cách (OSRM Fallback Haversine)
* **FR liên quan:** `FR-11`, `FR-12`
* **Actor:** Khách hàng (hoặc người dùng đã đăng nhập).
* **Tiền điều kiện:** Đã đăng nhập hệ thống (bắt buộc có JWT hợp lệ, chấp nhận mọi vai trò `CUSTOMER`, `DRIVER`, `ADMIN`). Cung cấp tọa độ điểm đón và điểm trả hợp lệ.
* **Luồng chính:**
  1. Người dùng gửi yêu cầu ước tính cước qua API kèm JWT: tọa độ đón `(pickup_lat, pickup_lng)` và trả `(dropoff_lat, dropoff_lng)` (`POST /api/v1/pricing/estimate`).
  2. `pricing-service` gửi request HTTP tới OSRM Public API với timeout tối đa **400 ms** để lấy khoảng cách đường bộ và thời gian di chuyển.
  3. Lấy hệ số nhân nhu cầu hiện tại (`surge_multiplier`) và bảng giá (`BaseFare`, `PricePerKm`).
  4. Tính tổng cước phí theo công thức:
     $$\text{TotalFare} = (\text{BaseFare} + \text{Distance} \times \text{PricePerKm}) \times \text{SurgeMultiplier}$$
     Cước phí là số nguyên VND, được làm tròn tới hàng nghìn gần nhất (ví dụ: 25.400 $\rightarrow$ 25.000, 25.600 $\rightarrow$ 26.000).
  5. Trả về kết quả chi tiết kèm nguồn tính khoảng cách là `OSRM`.
* **Luồng ngoại lệ (Fallback cốt lõi & Lỗi hợp lệ):**
  - *OSRM Public API bị timeout (> 400ms) hoặc mất mạng ngoài:* `pricing-service` ngay lập tức chuyển hướng sang công thức tính toán toán học nội bộ:
    $$\text{Distance} = \text{Haversine}(\text{Pickup}, \text{Dropoff}) \times 1.35$$
    Thời gian dự kiến $\text{ETA} = \frac{\text{Distance}}{30\text{ km/h}} \times 60 \text{ (phút)}$. Tổng cước cũng được làm tròn tới hàng nghìn gần nhất. Nguồn tính khoảng cách được đánh dấu là `HAVERSINE`. Request hoàn thành trong $< 1\text{ ms}$, không trả lỗi cho người dùng.
  - *Tọa độ không hợp lệ (ngoài phạm vi vĩ độ [-85.05112878, 85.05112878] theo tham chiếu LLD-00, kinh độ [-180, 180], hoặc điểm đón trùng điểm trả):* Trả về mã lỗi `VALIDATION_ERROR` (HTTP 400).
  - *Chưa đăng nhập / thiếu JWT:* `api-gateway` từ chối với mã lỗi `UNAUTHORIZED` (HTTP 401).
* **Hậu điều kiện:** Giá cước được tính toán chính xác để hiển thị cho người dùng tham khảo hoặc làm căn cứ tạo chuyến.
* **Dữ liệu demo cần thấy trên Postman:**
  - `distance` (km), `eta` (phút), `source` (`OSRM` hoặc `HAVERSINE`), `surge_multiplier`, `base_fare`, `price_per_km`, `total_fare` (VNĐ - số nguyên làm tròn tới hàng nghìn).

---

### UC-15: Tính toán hệ số nhu cầu Surge Pricing
* **FR liên quan:** `FR-13`
* **Actor:** Hệ thống (`pricing-service`).
* **Tiền điều kiện:** Có yêu cầu tính cước từ người dùng hoặc hệ thống cho một tọa độ điểm đón cụ thể.
* **Luồng chính:**
  1. `pricing-service` xác định ô lưới theo mã Geohash độ dài 5 ký tự (khoảng 4.9 km × 4.9 km) dựa trên tọa độ điểm đón. Ô Geohash này được dùng làm phạm vi địa lý chung cho cả nhu cầu (Demand) và nguồn cung (Supply):
     - **Nhu cầu (Demand):** Số chuyến xe được tạo trong 5 phút gần nhất thuộc cùng ô Geohash. `pricing-service` theo dõi và tổng hợp số liệu này thông qua sự kiện `TripCreated` nhận từ Redis Streams (phát bởi `dispatch-service`).
     - **Nguồn cung (Supply):** Số tài xế đang `ONLINE` có cập nhật GPS còn mới ($\le 15$ giây) trong cùng ô Geohash. `pricing-service` lấy danh sách ID tài xế trong ô từ `location-service`, sau đó gọi sang `user-service` để lọc các tài xế đang ở trạng thái `ONLINE` (đây là cách tính đơn giản có chủ đích, tuân thủ kiến trúc microservices và nguyên tắc Database-per-Service).
  2. Tính toán hệ số nhân nhu cầu theo công thức liên tục tuyến tính:
     $$\text{SurgeMultiplier} = \text{clamp}\left(\frac{\text{Demand} / \text{Supply}}{T},\; 1.0,\; 1.5\right)$$
     Trong đó:
     - $T$ là ngưỡng kích hoạt surge, mặc định đọc từ biến môi trường `SURGE_THRESHOLD` (giá trị mặc định $1.0$), Admin có thể thay đổi lúc runtime qua UC-16.
     - Hàm $\text{clamp}(x, 1.0, 1.5)$ giới hạn giá trị trong đoạn $[1.0, 1.5]$, đảm bảo hệ số tăng giảm liên tục, không có khoảng trống hay bước nhảy gián đoạn giữa các bậc.
  3. Quy tắc biên đặc biệt:
     - Nếu $\text{Supply} = 0$ và $\text{Demand} > 0$: Hệ số đạt mức tối đa $\text{SurgeMultiplier} = 1.5$ (thiếu xe hoàn toàn).
     - Nếu $\text{Demand} = 0$: Hệ số ở mức cơ sở $\text{SurgeMultiplier} = 1.0$ (không có nhu cầu).
  4. Hệ số sau khi tính được áp dụng ngay vào báo giá cước, **tuyệt đối không lưu cache kết quả surge** để phản ánh chính xác tức thời biến động cung - cầu.
* **Ghi chú demo:**
  - *Mẹo thử nghiệm demo:* Khi chạy script giả lập với 100 tài xế ảo, nguồn cung tài xế rất lớn khiến tỷ lệ $\text{Demand} / \text{Supply}$ ở mức thấp. Người điều hành có thể sử dụng API UC-16 để hạ ngưỡng $T$ xuống mức rất thấp (khoảng $0.01$ – $0.05$) nhằm dễ dàng kích hoạt surge hiển thị hệ số $> 1.0$ trên Postman/curl.
  - *Tính toán minh họa:* Với 100 tài xế ảo đặt trong cùng một ô geohash ($\text{Supply} = 100$):
    - Khi $T = 0.05$: Cần khoảng 6 chuyến đã tạo trong 5 phút để hệ số lên $1.2$ ($\frac{6/100}{0.05} = 1.2$).
    - Khi $T = 0.01$: Cần 2 chuyến đã tạo trong 5 phút để hệ số đạt mức trần $1.5$ ($\frac{2/100}{0.01} = 2.0 > 1.5$, kẹp tại $1.5$).
  - *Lưu ý về Demand và Script:* $\text{Demand}$ đếm tổng số chuyến được tạo (đặt rồi hủy rồi đặt lại vẫn được tính); script tài xế ảo phải đặt các tài xế trong cùng ô geohash của điểm đón.
* **Hậu điều kiện:** Hệ số surge được trả về cho công thức tính cước (UC-14).

---

### UC-16: Quản lý bảng giá cước (Admin)
* **FR liên quan:** `FR-31`
* **Actor:** Quản trị viên (Admin).
* **Tiền điều kiện:** Đã đăng nhập với quyền `ADMIN` (kèm JWT có role `ADMIN`).
* **Luồng chính:**
  1. Admin gửi request xem bảng giá cước hiện tại (`GET /api/v1/admin/pricing/config`).
  2. `pricing-service` trả về các tham số hiện hành: `BaseFare`, `PricePerKm`, ngưỡng kích hoạt surge $T$.
  3. Admin gửi request cập nhật (`PUT /api/v1/admin/pricing/config`): truyền giá trị mới (`BaseFare`, `PricePerKm`, `SurgeThreshold`).
  4. `pricing-service` ghi nhận dữ liệu vào `pricingdb` và cập nhật cache Redis.
  5. **Quy tắc hiệu lực:** Bảng giá mới **chỉ áp dụng cho các chuyến xe tạo mới sau thời điểm cập nhật**; các chuyến xe đã tạo trước đó giữ nguyên giá đã chốt.
* **Hậu điều kiện:** Bảng giá toàn hệ thống được cập nhật thành công.
* **Dữ liệu demo cần thấy trên Postman:** Cấu hình mới gồm `base_fare`, `price_per_km`, `surge_threshold`, thời gian cập nhật `updated_at`.

---

### UC-17: Đặt xe, kiểm tra ví & Chặn đa chuyến
* **FR liên quan:** `FR-15`, `FR-16`
* **Actor:** Khách hàng (Customer).
* **Tiền điều kiện:** Đã đăng nhập tài khoản khách hàng (JWT có vai trò `CUSTOMER`).
* **Luồng chính:**
  1. Khách hàng gửi tọa độ đón và trả để đặt xe (`POST /api/v1/trips`).
  2. `dispatch-service` kiểm tra xem khách hàng có chuyến nào đang hoạt động (`CREATED`, `MATCHING`, `ACCEPTED`, `PICKING_UP`, `IN_TRIP`) hay không.
  3. Nếu chưa có chuyến active: Gọi `pricing-service` lấy giá cước chốt (`fare`) (nội bộ, không qua api-gateway).
  4. Gọi `payment-service` kiểm tra số dư ví của khách (nội bộ, không qua api-gateway).
  5. Nếu số dư $\ge \text{fare}$:
     - `dispatch-service` thực thi **giao dịch DB duy nhất**: tạo bản ghi chuyến xe trong `dispatchdb` với trạng thái chuyển từ `CREATED` $\rightarrow$ `MATCHING`, lưu cố định giá cước chốt và thời hạn tìm xe 30 giây (matching timeout).
     - **Giá cước chốt được lưu cố định vào bản ghi chuyến và là giá thanh toán cuối cùng**, tuyệt đối không tính toán lại khi kết thúc chuyến.
     - `dispatch-service` phát sự kiện `TripCreated` vào Redis Streams (phục vụ tính toán Demand cho `pricing-service` và thu thập số liệu cho `ai-service`).
  6. Gọi tiếp sang use case phát sóng tìm tài xế (UC-18):
     - *Trường hợp tìm thấy $\ge 1$ tài xế phù hợp:* Phát lời mời cuốc xe, trả về HTTP `201 Created` kèm thông tin chuyến với `status = 'MATCHING'`.
     - *Trường hợp quét ra 0 tài xế:* Chuyến xe lập tức chuyển sang trạng thái `EXPIRED` (lưu DB và phát sự kiện `TripExpired` vào Redis Streams), đồng thời hệ thống vẫn trả về HTTP `201 Created` kèm thông tin chuyến với `status = 'EXPIRED'` (chuyến tạo thành công nhưng kết thúc ngay vì không có tài xế quanh khu vực).
* **Luồng ngoại lệ:**
  - *Khách hàng đã có một chuyến khác đang hoạt động:* Bị chặn ngay lập tức với mã lỗi `ACTIVE_TRIP_EXISTS` (HTTP 409).
  - *Số dư ví không đủ chi trả giá cước tạm tính:* Bị chặn với mã lỗi `INSUFFICIENT_BALANCE` (HTTP 400). Chuyến xe không được tạo.
* **Ghi chú demo:**
  - *Khởi tạo ví kiểu lazy:* Ví người dùng chưa tồn tại thì được tạo kiểu lazy với số dư 0 khi có yêu cầu truy vấn hoặc giao dịch đầu tiên (FR-22). Do đó khách hàng mới đăng ký chưa từng nạp tiền khi đặt xe sẽ nhận mã lỗi `INSUFFICIENT_BALANCE` (HTTP 400).
  - *Hạn chế đã biết của FR-15:* Sau khi một chuyến xe hoàn thành (`COMPLETED`), nghiệp vụ trừ tiền ví khách và cộng ví tài xế diễn ra bất đồng bộ qua Redis Streams. Vì vậy trong kịch bản demo, khách hàng cần kiểm tra ví đã trừ tiền xong trước khi tạo cuốc xe tiếp theo để tránh xung đột số dư hoặc trạng thái ví.
* **Hậu điều kiện:** Chuyến xe được khởi tạo bền vững trong hệ thống (`MATCHING` hoặc `EXPIRED`), giá cước được khóa cố định.
* **Dữ liệu demo cần thấy trên Postman:** `trip_id`, trạng thái (`MATCHING` hoặc `EXPIRED`), `fare`, `pickup_address`, `dropoff_address`.

---

### UC-18: Broadcast đề nghị nhận chuyến cho tài xế
* **FR liên quan:** `FR-17`
* **Actor:** Hệ thống (`dispatch-service` $\rightarrow$ `location-service` $\rightarrow$ `user-service` $\rightarrow$ `ws-gateway`).
* **Tiền điều kiện:** Chuyến xe vừa được tạo thành công ở trạng thái `MATCHING`.
* **Luồng chính:**
  1. `dispatch-service` gọi `location-service` quét tối đa 50 ứng viên tài xế có cập nhật GPS trong vòng 15 giây gần nhất quanh điểm đón trong bán kính $R$ (nội bộ, không qua api-gateway, UC-12). `location-service` trả về danh sách ứng viên kèm khoảng cách `distance_m`.
  2. `dispatch-service` gọi `user-service` MỘT lần duy nhất để lọc `ONLINE` hàng loạt (nội bộ, không qua api-gateway) kiểm tra và lọc ra các tài xế đang thực sự ở trạng thái `ONLINE`.
  3. Từ danh sách tài xế `ONLINE`, `dispatch-service` cắt lấy **Top 3–5 tài xế gần nhất**, sau đó **lưu danh sách tài xế được mời này vào bản ghi chuyến xe trong `dispatchdb`** (làm căn cứ xác thực khi tài xế bấm nhận cuốc ở UC-19).
  4. **Trường hợp 1 (Có tài xế phù hợp - Top 1..5):**
     - `dispatch-service` phát thông báo mời chuyến vào Redis Pub/Sub kênh `ride:trip_offers` chứa: `trip_id`, danh sách `driver_id` được mời, tọa độ đón, giá cước tài xế nhận ($85\%$), kèm `distance_m` tương ứng của từng tài xế.
     - `ws-gateway` lắng nghe và đẩy thông báo xuống ứng dụng WebSocket của các tài xế nằm trong danh sách được mời.
     - Hạn tìm xe 30 giây đã được lưu bền (UC-17); tiến trình quét nền (UC-22) chuyển chuyến quá hạn sang `EXPIRED`.
  5. **Trường hợp 2 (Quét ra 0 tài xế ONLINE):**
     - Chuyến xe lập tức chuyển sang trạng thái `EXPIRED` mà không cần chờ hết 30 giây (kích hoạt UC-22).
* **Hậu điều kiện:** Danh sách tài xế được mời được lưu bền trong DB; các tài xế mục tiêu nhận được thông báo mời cuốc kèm khoảng cách qua WebSocket.

---

### UC-19: Nhận chuyến cạnh tranh (Redis Atomic Lock)
* **FR liên quan:** `FR-18`
* **Actor:** Tài xế (Driver).
* **Tiền điều kiện:** Tài xế nhận được đề nghị cuốc xe và gửi yêu cầu nhận chuyến (`POST /api/v1/trips/:id/accept`).
* **Luồng chính (Thứ tự thực hiện bắt buộc):**
  1. **Bước 1 - Kiểm tra nhanh tại `dispatch-service`:**
     - Kiểm tra tài xế có nằm trong danh sách được mời của chuyến xe (đã lưu ở UC-18) hay không. Nếu không thuộc danh sách mời, từ chối ngay với mã lỗi `NOT_OFFERED` (HTTP 403).
     - Kiểm tra trạng thái hiện tại của chuyến xe:
       - Nếu chuyến đã có người nhận (`ACCEPTED`): trả về mã lỗi `TRIP_ALREADY_TAKEN` (HTTP 409).
       - Nếu chuyến đã kết thúc (`EXPIRED` hoặc `CANCELLED`): trả về mã lỗi `INVALID_TRIP_STATUS` (HTTP 400).
       - Nếu chuyến vẫn đang `MATCHING`: tiếp tục bước 2.
  2. **Bước 2 - Khóa nguyên tử phân tán trên Redis:**
     - Thực thi lệnh: `SET ride:lock:<trip_id> <driver_id> NX EX 10`.
     - Nếu thất bại (khóa đã tồn tại do tài xế khác nhanh tay lock trước): trả về mã lỗi `TRIP_ALREADY_TAKEN` (HTTP 409).
     - Nếu thành công (nhận được `OK`): tài xế giành được lock, tiếp tục bước 3.
  3. **Bước 3 - Chuyển trạng thái tài xế có điều kiện tại `user-service`:**
     - `dispatch-service` gọi `user-service` chuyển trạng thái tài xế từ `ONLINE` $\rightarrow$ `BUSY` có điều kiện (nội bộ, không qua api-gateway; chỉ thành công nếu tài xế hiện đang `ONLINE`).
     - *Nếu thất bại* (tài xế đã chuyển `OFFLINE`, hoặc đang `BUSY` - ví dụ trường hợp một tài xế bấm nhận 2 chuyến cùng lúc và đã thắng chuyến kia trước):
       - **Hoàn tác:** `dispatch-service` chủ động giải phóng khóa Redis `ride:lock:<trip_id>` của chính mình.
       - Trả về mã lỗi `DRIVER_NOT_AVAILABLE` (HTTP 400).
     - *Nếu thành công* (tài xế đã chuyển sang `BUSY`): tiếp tục bước 4.
  4. **Bước 4 - Cập nhật trạng thái chuyến xe trong `dispatchdb`:**
     - `dispatch-service` thực thi lệnh cập nhật:  
       `UPDATE trips SET status = 'ACCEPTED', driver_id = <driver_id> WHERE id = <trip_id> AND status = 'MATCHING'`.
     - *Nếu thành công (1 dòng được cập nhật):*
       - `dispatch-service` phát sự kiện chuyển trạng thái qua Redis Pub/Sub kênh `ride:trip_updates` (UC-13).
       - Bắt đầu chuyển tiếp tọa độ GPS của tài xế thời gian thực cho khách hàng (UC-31).
       - Trả về HTTP `200 OK` kèm thông tin chuyến xe đã gán tài xế.
     - *Nếu thất bại (0 dòng được cập nhật* do xung đột chạy đua khiến chuyến đổi trạng thái giữa Bước 1 và Bước 4):
       - **Hoàn tác:** Gọi `user-service` rollback trạng thái tài xế từ `BUSY` $\rightarrow$ `ONLINE` (nội bộ, không qua api-gateway), đồng thời giải phóng khóa Redis `ride:lock:<trip_id>`.
       - Đọc lại trạng thái hiện tại của chuyến xe: nếu chuyến đã `ACCEPTED` thì trả về mã lỗi `TRIP_ALREADY_TAKEN` (HTTP 409); nếu chuyến đã `EXPIRED` hoặc `CANCELLED` thì trả về mã lỗi `INVALID_TRIP_STATUS` (HTTP 400) (khớp quy tắc Bước 1). Kết quả cuối cùng trạng thái tài xế vẫn luôn là `ONLINE`.
* **Luồng ngoại lệ:**
  - *Tài xế không nằm trong danh sách mời của chuyến:* Trả về mã lỗi `NOT_OFFERED` (HTTP 403).
  - *Tài xế không sẵn sàng (đang `OFFLINE`, `BUSY`, hoặc bấm nhận 2 chuyến cùng lúc):* Bị chặn ở Bước 3, `dispatch-service` nhả lock Redis và trả về mã lỗi `DRIVER_NOT_AVAILABLE` (HTTP 400).
  - *Tài xế bấm nhận nhưng đã có tài xế khác nhận trước hoặc thất bại khi lấy lock:* Trả về mã lỗi `TRIP_ALREADY_TAKEN` (HTTP 409).
  - *Khóa Redis hết hạn / tài xế gửi request đến muộn:* Bị chặn ngay ở Bước 1 (chuyến đã ở trạng thái `ACCEPTED` thì trả về `TRIP_ALREADY_TAKEN` - HTTP 409), chưa chạm đến lock Redis hay `user-service`; chỉ khi trạng thái chuyến thay đổi trong khoảng thời gian giữa Bước 1 và Bước 4 mới dẫn tới cơ chế hoàn tác `BUSY` $\rightarrow$ `ONLINE`; kết quả cuối cùng trạng thái của tài xế vẫn luôn là `ONLINE`.
  - *Chuyến xe đã ở trạng thái kết thúc (`EXPIRED` hoặc `CANCELLED`):* Trả về mã lỗi `INVALID_TRIP_STATUS` (HTTP 400).
* **Hậu điều kiện:** Đúng duy nhất 1 tài xế nhận được chuyến; tài xế chuyển sang `BUSY`; chuyến xe bước vào giai đoạn đón khách (`ACCEPTED`).
* **Dữ liệu demo cần thấy trên Postman:** `trip_id`, trạng thái mới `ACCEPTED`, `driver_id` đã được gán.
