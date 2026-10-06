# PHÂN HỆ VÍ & THANH TOÁN - USE CASE ĐẶC TẢ

> **Mã tài liệu:** `UC-24` đến `UC-26`  
> **Dịch vụ chịu trách nhiệm:** `payment-service`  
> **Tài liệu tham chiếu:** [SRS v1.4 (Mục 3.5)](../01-srs/srs.md), [quyet-dinh.md](../00-brainstorm/quyet-dinh.md)

---

## 1. SƠ ĐỒ TUẦN TỰ THANH TOÁN TỰ ĐỘNG QUA REDIS STREAMS (SEQUENCE DIAGRAM)

```mermaid
sequenceDiagram
    autonumber
    participant RedisStreams as Redis Streams (stream:trip_events)
    participant PaymentWorker as payment-service (Consumer Group)
    participant DB as paymentdb (PostgreSQL)

    Note over RedisStreams,PaymentWorker: Nhận sự kiện TripCompleted bất đồng bộ
    RedisStreams->>PaymentWorker: XREADGROUP GROUP payment-group {type: "TripCompleted", trip_id, customer_id, driver_id, fare}
    
    PaymentWorker->>DB: Kiểm tra tính lũy thừa (Idempotency): Đã tồn tại giao dịch thành công cho trip_id chưa?
    alt Đã tồn tại giao dịch cho trip_id (Event bị giao lại)
        PaymentWorker->>RedisStreams: XACK stream:trip_events payment-group <message_id>
        Note over PaymentWorker: Bỏ qua xử lý để chống trừ tiền 2 lần
    else Chưa xử lý trip_id này
        PaymentWorker->>DB: Kiểm tra số dư ví khách hàng (SELECT balance FROM wallets WHERE user_id = customer_id FOR UPDATE)
        
        alt Bất thường: Số dư ví < fare (Không đủ tiền)
            PaymentWorker->>DB: INSERT transaction {trip_id, user_id: customer_id, amount: 0, status: 'FAILED', note: 'INSUFFICIENT_FUNDS_ON_COMPLETE'}
            Note over PaymentWorker,DB: TUYỆT ĐỐI KHÔNG TRỪ ÂM VÍ. Ghi log cảnh báo can thiệp thủ công!
            PaymentWorker->>RedisStreams: XACK stream:trip_events payment-group <message_id>
        else Số dư ví đủ chi trả (Bình thường)
            PaymentWorker->>DB: Trừ ví khách: balance = balance - fare
            PaymentWorker->>DB: Tính hoa hồng 15%: Commission = fare * 0.15; Tài xế nhận: fare - Commission
            PaymentWorker->>DB: Cộng ví tài xế: balance = balance + (fare - Commission)
            
            PaymentWorker->>DB: Ghi 3 dòng giao dịch riêng biệt:
            Note over PaymentWorker,DB: 1. Trừ ví khách (-fare, status: SUCCESS, trip_id)<br>2. Cộng ví tài xế (+85%, status: SUCCESS, trip_id)<br>3. Hoa hồng hệ thống (+15%, status: SUCCESS, trip_id)
            
            PaymentWorker->>RedisStreams: XACK stream:trip_events payment-group <message_id>
            Note over PaymentWorker: Xác nhận hoàn tất xử lý tin nhắn an toàn
        end
    end
```

---

## 2. CHI TIẾT TỪNG USE CASE

### UC-24: Xem số dư ví & Lịch sử giao dịch
* **FR liên quan:** `FR-22`
* **Actor:** Khách hàng (Customer), Tài xế (Driver).
* **Tiền điều kiện:** Người dùng đã đăng nhập tài khoản.
* **Luồng chính:**
  1. Người dùng gửi request tra cứu ví cá nhân (`GET /api/v1/wallets/me`).
  2. `payment-service` truy vấn bản ghi ví của người dùng trong `paymentdb` dựa trên header `X-User-Id`.
  3. Trả về thông tin: ID ví, số dư khả dụng (`balance`), đơn vị tiền tệ (`VND`).
  4. Người dùng gửi request xem lịch sử giao dịch (`GET /api/v1/wallets/transactions`).
  5. `payment-service` truy vấn bảng `transactions` và trả về danh sách các dòng giao dịch được ghi nhận riêng biệt:
     - **Dòng trừ tiền khách:** Loại giao dịch `TRIP_PAYMENT`, số tiền âm `(-fare)`, mã `trip_id`, trạng thái `SUCCESS`.
     - **Dòng thu nhập tài xế:** Loại giao dịch `TRIP_INCOME`, số tiền dương `(+85% fare)`, mã `trip_id`, trạng thái `SUCCESS`.
     - **Dòng nạp tiền:** Loại giao dịch `TOPUP`, số tiền nạp, trạng thái `SUCCESS`.
* **Hậu điều kiện:** Số dư và chi tiết lịch sử từng dòng giao dịch được hiển thị minh bạch.
* **Dữ liệu demo cần thấy trên Postman:**
  - Thông tin ví: `wallet_id`, `balance` (ví dụ: `150,000 VND`).
  - Danh sách giao dịch: Mỗi dòng gồm `id`, `trip_id`, `amount`, `type` (`TRIP_PAYMENT`/`TRIP_INCOME`/`COMMISSION`/`TOPUP`), `status` (`SUCCESS`/`FAILED`), thời gian `created_at`.

---

### UC-25: Nạp tiền vào ví ảo (Top-up API Demo)
* **FR liên quan:** `FR-23`
* **Actor:** Khách hàng, Tài xế.
* **Tiền điều kiện:** Đã đăng nhập tài khoản.
* **Luồng chính:**
  1. Người dùng gửi yêu cầu nạp tiền (`POST /api/v1/wallets/top-up`) với số tiền mong muốn nạp (ví dụ: 100,000 VNĐ).
  2. `payment-service` kiểm tra số tiền nạp hợp lệ ($> 0$).
  3. Mở database transaction trong `paymentdb`:
     - Cộng số tiền nạp vào số dư ví của người dùng: `balance = balance + amount`.
     - Chèn một bản ghi giao dịch mới vào bảng `transactions`: loại `TOPUP`, trạng thái `SUCCESS`, số tiền `+amount`.
  4. Trả về thông tin số dư ví mới cập nhật.
* **Luồng ngoại lệ:**
  - *Số tiền nạp không hợp lệ ($\le 0$):* Trả về lỗi Validation (HTTP 400).
* **Hậu điều kiện:** Số dư ví của người dùng tăng lên ngay lập tức, phục vụ kịch bản demo đặt xe qua Postman mà không cần cổng thanh toán ngoài.
* **Dữ liệu demo cần thấy trên Postman:** `wallet_id`, `added_amount`, số dư mới `new_balance`.

---

### UC-26: Thanh toán tự động & Trích khấu hoa hồng (Event-Driven Payment)
* **FR liên quan:** `FR-24`, `FR-25`
* **Actor:** Hệ thống (`payment-service` Worker).
* **Tiền điều kiện:** `dispatch-service` đã phát sự kiện `TripCompleted` vào Redis Streams (`stream:trip_events`).
* **Luồng chính:**
  1. Worker của `payment-service` lắng nghe stream `stream:trip_events` thông qua Consumer Group `payment-group`.
  2. Nhận bản tin sự kiện `TripCompleted` mang theo: `trip_id`, `customer_id`, `driver_id`, giá cước chốt `fare`.
  3. **Kiểm tra tính lũy thừa (Idempotency):**
     - Truy vấn bảng `transactions` xem đã tồn tại giao dịch thành công nào có tham chiếu `trip_id` này hay chưa.
     - Nếu đã tồn tại (do bản tin Redis bị gửi lặp lại): Gửi `XACK` ngay lập tức và bỏ qua xử lý để tránh trừ tiền trùng lặp.
  4. **Kiểm tra số dư ví khách hàng:**
     - Truy vấn số dư ví của `customer_id` với khóa hàng `FOR UPDATE`.
     - Nếu số dư $\ge \text{fare}$:
       - Trừ ví khách số tiền `fare`: `balance = balance - fare`.
       - Tính hoa hồng hệ thống **15%** (lấy từ cấu hình ENV):
         $$\text{Commission} = \text{round}(\text{fare} \times 0.15)$$
       - Tính thu nhập ròng của tài xế ($85\%$):
         $$\text{DriverIncome} = \text{fare} - \text{Commission}$$
       - Cộng ví tài xế: `balance = balance + DriverIncome`.
       - Ghi 3 dòng giao dịch riêng biệt vào bảng `transactions`:
         1. Dòng trừ khách: `{user_id: customer_id, trip_id, type: 'TRIP_PAYMENT', amount: -fare, status: 'SUCCESS'}`.
         2. Dòng cộng tài xế: `{user_id: driver_id, trip_id, type: 'TRIP_INCOME', amount: +DriverIncome, status: 'SUCCESS'}`.
         3. Dòng hoa hồng: `{user_id: SYSTEM, trip_id, type: 'COMMISSION', amount: +Commission, status: 'SUCCESS'}`.
       - Gửi lệnh `XACK` xác nhận đã xử lý tin nhắn tới Redis Streams.
* **Luồng ngoại lệ (Trường hợp bất thường thiếu tiền):**
  - *Số dư ví khách hàng $< \text{fare}$ lúc trừ:*
    - **Quy tắc cốt lõi:** Hệ thống **tuyệt đối không bao giờ để số dư ví bị âm**.
    - Không thực hiện trừ tiền ví khách và không cộng tiền cho tài xế.
    - Ghi nhận 1 bản ghi giao dịch thất bại: `{trip_id, user_id: customer_id, amount: 0, status: 'FAILED', note: 'INSUFFICIENT_FUNDS_ON_COMPLETE'}`.
    - Ghi log nghiêm trọng mức `ERROR` để người quản trị xử lý thủ công (ghi chú: đây là tình huống bất thường không mong đợi vì use case UC-17 đã kiểm tra và chặn ngay từ đầu).
    - Vẫn gửi lệnh `XACK` để tránh consumer bị treo vòng lặp vô tận.
* **Hậu điều kiện:** Tiền cước được thanh toán đầy đủ; hoa hồng 15% được khấu trừ chuẩn xác; các dòng giao dịch được lưu vết hoàn chỉnh.
* **Dữ liệu demo cần thấy trên Postman/DB:** 3 dòng giao dịch riêng biệt cùng gắn mã `trip_id` với trạng thái `SUCCESS`.
