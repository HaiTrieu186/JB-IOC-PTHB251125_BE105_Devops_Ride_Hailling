# DỮ LIỆU MẪU KIỂM THỬ HỆ THỐNG (TEST DATA SPECIFICATION)

> **Mã tài liệu:** `TEST-00` | **Phiên bản:** `1.0`  
> **Tài liệu tham chiếu:** [LLD-00](../03-lld/00-hop-dong-lien-service.md), [LLD user-service](../03-lld/user-service.md), [.env.example](../../.env.example)  
> **Nguyên tắc cốt lõi:** Đây là tài liệu nguồn **DUY NHẤT** quy định thông tin tài khoản, tọa độ và dữ liệu mẫu dùng cho kiểm thử tự động và thủ công bằng Postman/cURL. Mọi kịch bản kiểm thử ở các giai đoạn sau chỉ bổ sung, không thay đổi thông tin đã ban hành tại đây.

---

## 1. QUY ƯỚC MẬT KHẨU & PHẠM VI DỮ LIỆU DEMO

- **Mật khẩu demo chung:** `Test@1234` (áp dụng cho toàn bộ tài khoản Khách hàng `CUSTOMER` và Tài xế `DRIVER`, độ dài $\ge 6$ ký tự, đáp ứng kiểm tra tính hợp lệ của `user-service`).
- **Ghi chú an toàn:** Toàn bộ thông tin tài khoản, số điện thoại, email và tọa độ trong tài liệu này là dữ liệu giả lập phục vụ kiểm thử môn học DevOps.

---

## 2. TÀI KHOẢN QUẢN TRỊ VIÊN (ADMIN)

Khởi tạo tự động lúc container `user-service` khởi động (FR-30, Idempotent):

| Vai trò | Email đăng nhập | Mật khẩu demo | Role DB | Driver Status | Mục đích sử dụng |
|---|---|---|---|---|---|
| **Admin** | `admin@ridehailing.local` | `Admin@123456` | `ADMIN` | `NULL` | Quản trị cấu hình giá cước (FR-31), can thiệp hủy cưỡng bức chuyến kẹt (FR-37). |

> [!NOTE]
> Thông tin Admin lấy từ `ADMIN_EMAIL` và `ADMIN_PASSWORD` trong file `.env.example`. Trong môi trường thực tế, mật khẩu trong file `.env` thật có thể được cấu hình khác.

---

## 3. TÀI KHOẢN KHÁCH HÀNG (CUSTOMERS)

Mật khẩu đăng nhập chung: `Test@1234`.

| Ký hiệu | Họ và tên | Số điện thoại | Email | Số dư ví ban đầu (Demo) | Mục đích kịch bản |
|:---:|---|---|---|---|---|
| **C1** | Nguyen Van An | `0901000001` | `customer1@demo.local` | 200,000 VND | Luồng chính (Happy Path): Đặt xe, theo dõi tài xế, hoàn thành cuốc và thanh toán. |
| **C2** | Le Thi Binh | `0901000002` | `customer2@demo.local` | 150,000 VND | Luồng biên / ngoại lệ: Tranh chấp cuốc xe, hủy chuyến chủ động. |
| **C3** | Pham Van Cuong | `0901000003` | `customer3@demo.local` | **0 VND (Không nạp)** | Luồng lỗi nghiệp vụ: Đặt xe khi ví không đủ số dư (`INSUFFICIENT_BALANCE` 400). |

---

## 4. TÀI KHOẢN TÀI XẾ (DRIVERS)

Mật khẩu đăng nhập chung: `Test@1234`.

| Ký hiệu | Họ và tên | Số điện thoại | Email | Biển số xe (Tham khảo) | Loại xe | Trạng thái ban đầu |
|:---:|---|---|---|---|---|---|
| **D1** | Tran Van Dung | `0902000001` | `driver1@demo.local` | `59A-100.01` | MOTORBIKE | `OFFLINE` |
| **D2** | Nguyen Van Em | `0902000002` | `driver2@demo.local` | `59A-100.02` | MOTORBIKE | `OFFLINE` |
| **D3** | Hoang Van Phuc | `0902000003` | `driver3@demo.local` | `59A-100.03` | MOTORBIKE | `OFFLINE` |
| **D4** | Vo Van Giang | `0902000004` | `driver4@demo.local` | `59A-100.04` | MOTORBIKE | `OFFLINE` |
| **D5** | Dang Van Hai | `0902000005` | `driver5@demo.local` | `59A-100.05` | MOTORBIKE | `OFFLINE` |

> [!IMPORTANT]
> **Về thông tin phương tiện:** Biển số xe `59A-100.01` đến `59A-100.05` chỉ ghi nhận để làm dữ liệu chuẩn cho các lượt sau. Ở Tầng 1, `user-service` không nhận thông tin xe khi đăng ký (DTO `RegisterRequest` không có trường xe; API `PUT /driver/vehicle` thuộc Tầng 3 và chưa triển khai). Tuyệt đối **không** gửi thông tin xe trong request đăng ký tài xế.

---

## 5. DẢI SỐ DÀNH RIÊNG CHO SCRIPT TÀI XẾ ẢO (LƯỢT 10)

Để tránh xung đột dữ liệu với 5 tài xế mẫu D1–D5:
- Dải số điện thoại `0903xxxxxx` (100 tài xế ảo tải trọng).
- Dải số điện thoại `0904xxxxxx` (Dự phòng mở rộng).
- **Quy tắc:** Tuyệt đối không dùng 2 dải số này trong các kịch bản kiểm thử API thủ công / Postman.

---

## 6. TỌA ĐỘ MẪU, KHOẢNG CÁCH HAVERSINE & GEOHASH5

Các điểm mốc và vị trí tài xế phục vụ kiểm thử định tuyến, điều phối và tính giá cước động (Surge Pricing):

### 6.1. Điểm đón (P1) và Điểm đến (P2)
- **P1 (Điểm đón - Chợ Bến Thành):**
  - Latitude: `10.7725`, Longitude: `106.6980`
  - Geohash5: `w3gv7`
- **P2 (Điểm đến - Landmark 81):**
  - Latitude: `10.7951`, Longitude: `106.7218`
  - Geohash5: `w3gvk`
  - Khoảng cách đường chim bay Haversine ($P1 \rightarrow P2$): **3,615.8 m** (~3.62 km).

### 6.2. Vị trí phân bố 5 Tài xế (D1 đến D5) so với P1

| Tài xế | Vĩ độ (Lat) | Kinh độ (Lng) | Khoảng cách tới P1 (Haversine) | Geohash5 | Mô tả phân vùng kiểm thử |
|:---:|:---:|:---:|:---:|:---:|---|
| **D1** | `10.7738` | `106.6992` | **195.1 m** (~200 m) | `w3gv7` | Rất gần P1, cùng ô Geohash5 với điểm đón (Top 1 ưu tiên điều phối). |
| **D2** | `10.7775` | `106.7032` | **794.8 m** (~800 m) | `w3gvk` | Gần P1 (Top 2 ưu tiên điều phối). |
| **D3** | `10.7855` | `106.7105` | **1,988.4 m** (~2.0 km) | `w3gvk` | Trong bán kính 2 km (Top 3 ưu tiên điều phối). |
| **D4** | `10.7985` | `106.7240` | **4,052.6 m** (~4.0 km) | `w3gvk` | Trong bán kính điều phối quét rộng ($R \le 5\text{ km}$). |
| **D5** | `10.8185` | `106.6588` | **6,670.5 m** (~6.67 km) | `w3gve` | Khu vực Tân Sơn Nhất, ngoài bán kính 5 km (Loại khỏi danh sách ứng viên điều phối). |

*Ghi chú Surge Pricing:* Chỉ tài xế D1 nằm trong ô `w3gv7` của P1 $\rightarrow$ Supply tại ô P1 ban đầu = 1. Tài xế D2, D3, D4 thuộc ô `w3gvk` (ô của P2).

---

## 7. BẢNG TRẠNG THÁI TIỀN ĐIỀU KIỆN (PRE-CONDITIONS MATRIX)

Bảng chuẩn bị trạng thái hệ thống trước khi thực hiện các kịch bản kiểm thử:

| Nhóm kịch bản | Dịch vụ kiểm thử | Tiền điều kiện tài khoản | Tiền điều kiện tọa độ & Redis | Trạng thái dọn dẹp sau kịch bản |
|---|---|---|---|---|
| **U-01..U-15** | `user-service` | Chạy folder "00 Seed" để tạo sẵn C1–C3, D1–D5. | Không yêu cầu GPS. | Đưa trạng thái D1 về `OFFLINE`/`ONLINE` như ban đầu. |
| **L-01..L-05** | `location-service` *(Lượt 2)* | D1–D5 đã đăng ký. | D1–D5 gửi gói tin GPS vào Redis GEO qua REST nội bộ. | Tọa độ lưu trong `drivers:geo`. |
| **P-01..P-06** | `pricing-service` *(Lượt 3)* | C1 tra cứu báo giá. | D1 có vị trí trong ô `w3gv7` để tính Supply. | Cache pricing hết hạn sau 1h hoặc reset. |
| **W-01..W-06** | `payment-service` *(Lượt 4)* | C1, C2 đã nạp ví; C3 số dư = 0. | Ví khởi tạo thành công trong `paymentdb`. | Kiểm tra log số dư trước/sau cuốc. |
| **DP-01..DP-10**| `dispatch-service` *(Lượt 5)* | C1 đặt xe, D1 nhận cuốc. | D1 trạng thái `ONLINE`, GPS $\le 15\text{s}$. | Hủy chuyến hoặc hoàn tất để nhả lock và đưa D1 về `ONLINE`. |
