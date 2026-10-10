# ĐẶC TẢ YÊU CẦU PHẦN MỀM (SRS) - HỆ THỐNG RIDE-HAILING & LOGISTICS

> **Dự án:** Hệ thống Đặt xe & Điều phối Thời gian thực (Ride-Hailing & Logistics System)  
> **Môn học:** DevOps  
> **Phiên bản:** 1.5  
> **Tài liệu căn cứ:** [AGENTS.md](../../AGENTS.md), [de-bai-goc.md](../00-brainstorm/de-bai-goc.md), [quyet-dinh.md](../00-brainstorm/quyet-dinh.md)

---

## 1. MỤC TIÊU DỰ ÁN

1. **Mục tiêu chính:** Xây dựng hệ thống backend microservices mô phỏng dịch vụ gọi xe công nghệ (mô hình Grab/Gojek), đóng gói container hóa Docker, thiết lập CI/CD tự động bằng GitHub Actions và triển khai (deploy) vận hành ổn định trên hạ tầng Cloud VPS thực tế.
2. **Phạm vi triển khai:** Hệ thống tập trung hoàn toàn vào **Backend và DevOps**. Toàn bộ các luồng nghiệp vụ cốt lõi được kiểm thử và nghiệm thu qua **cURL / Postman** và **Script giả lập 100 tài xế ảo**. **Không phát triển giao diện người dùng (UI Web/Mobile/Admin)**.
3. **Môi trường hạ tầng:** Vận hành tối ưu trên môi trường VPS hạn chế tài nguyên: **1 vCPU, 1 GB RAM, 20 GB Disk (Ubuntu 22.04 LTS)**, có cấu hình Swap 2 GB.

---

## 2. TÁC NHÂN HỆ THỐNG (ACTORS)

* **Khách hàng (Passenger / Customer):**
  * Đăng ký, đăng nhập tài khoản khách hàng; làm mới token phiên truy cập và đăng xuất thu hồi token (tự động đóng kết nối WebSocket).
  * Tra cứu ước tính cước phí và khoảng cách giữa điểm đón và điểm trả (yêu cầu JWT, xem chi tiết khoảng cách, ETA, BaseFare, PricePerKm, hệ số surge và nguồn OSRM/HAVERSINE).
  * Quản lý ví cá nhân (ví được tạo kiểu lazy khi truy cập lần đầu hoặc có giao dịch đầu tiên; kiểm tra số dư, nạp tiền demo, xem lịch sử giao dịch chi tiết theo dòng và trạng thái).
  * Tạo yêu cầu đặt xe (mỗi thời điểm chỉ có tối đa 1 chuyến đang hoạt động).
  * Xem chuyến xe đang hoạt động (`active trip`) và xem chi tiết chuyến của chính mình (kèm lịch sử mốc thời gian chuyển trạng thái).
  * Kết nối WebSocket (`/ws/`) để nhận thông báo đẩy khi chuyến xe thay đổi trạng thái theo thời gian thực.
  * **Nhận tọa độ tài xế thời gian thực qua WebSocket:** Từ khi chuyến `ACCEPTED` đến khi kết thúc chuyến (`COMPLETED` hoặc `CANCELLED`), khách nhận stream tọa độ GPS mới nhất của tài xế.
  * Hủy chuyến khi chuyến đang ở trạng thái `MATCHING` hoặc `ACCEPTED` (không được hủy khi tài xế đang đến đón `PICKING_UP` hoặc xe đang chạy `IN_TRIP`).
* **Tài xế (Driver):**
  * Đăng ký, đăng nhập tài khoản tài xế; làm mới token phiên truy cập và đăng xuất thu hồi token (đăng xuất thành công sẽ tự chuyển `OFFLINE` và đóng kết nối WebSocket).
  * Bật/tắt trạng thái hoạt động (`ONLINE`, `OFFLINE`). Tự động sang `BUSY` khi nhận chuyến và quay lại `ONLINE` khi chuyến kết thúc (`COMPLETED`) hoặc bị hủy (`CANCELLED`).
  * Tài xế đang `BUSY` không được tự ý chuyển sang `OFFLINE` và bị từ chối đăng xuất cho đến khi hết chuyến.
  * Kết nối WebSocket (`/ws/`) để phát stream tọa độ GPS liên tục (chu kỳ 3–5 giây), nhận tín hiệu mời chuyến mới (Broadcast Notification), và nhận thông báo đổi trạng thái chuyến.
  * Chấp nhận chuyến (`ACCEPT`) theo cơ chế cạnh tranh khóa nguyên tử: chỉ tài xế nằm trong danh sách được mời mới được nhận chuyến, chỉ chấp nhận khi đang `ONLINE`, và một tài xế không thể giữ 2 chuyến cùng lúc.
  * Xem chuyến xe đang hoạt động (`active trip`) và xem chi tiết chuyến của chính mình (kèm lịch sử mốc thời gian chuyển trạng thái).
  * Cập nhật tiến trình chuyến: Đang đón khách (`PICKING_UP`) $\rightarrow$ Đang chở khách (`IN_TRIP`) $\rightarrow$ Hoàn thành (`COMPLETED`).
  * Hủy chuyến khi chuyến đang ở trạng thái `ACCEPTED` hoặc `PICKING_UP` (gặp sự cố hợp lệ; không được hủy khi chuyến đang `IN_TRIP`).
  * Nhận tiền cước sau khi hệ thống tự động trừ hoa hồng.
* **Quản trị viên (Admin):**
  * Đăng nhập tài khoản Quản trị viên (được khởi tạo sẵn từ biến môi trường - ENV khi khởi động, không cho đăng ký tự do, không có ví cá nhân); làm mới phiên và đăng xuất.
  * Mọi endpoint của Admin nằm thống nhất dưới tiền tố `/api/v1/admin/*`.
  * Xem và cập nhật cấu hình bảng giá (`BaseFare`, `PricePerKm`, ngưỡng surge $T$) áp dụng cho các chuyến tạo mới.
  * Xem dữ liệu vận hành toàn hệ thống ở chế độ **chỉ đọc** (danh sách chuyến từ `dispatch-service`, danh sách người dùng từ `user-service`, báo cáo từ `ai-service`, danh sách tài xế đang hoạt động có GPS trong 15 giây gần nhất từ `location-service`).
  * **Quyền ghi duy nhất (Ngoại lệ):** Thực hiện lệnh hủy cưỡng bức chuyến kẹt (FR-37) ở bất kỳ trạng thái chưa kết thúc nào để giải phóng tài nguyên.
  * Gọi API sinh nhận xét và báo cáo tổng kết tình hình kinh doanh (AI / Heuristic) có cơ chế tự động fallback sang Rule-based khi LLM lỗi/timeout.

---

## 3. DANH SÁCH YÊU CẦU CHỨC NĂNG (FUNCTIONAL REQUIREMENTS)

> **Quy ước mức độ ưu tiên:**
> * **Bắt buộc (Must-Have):** Nghiệp vụ sống còn của đồ án và bắt buộc để vượt qua kịch bản demo.
> * **Nên có (Should-Have):** Tăng cường tính hoàn thiện của hệ thống nhưng có thể tinh giản.
> * **Bỏ qua (Out of Scope):** Không thực hiện để đảm bảo tiết kiệm RAM và tập trung cho DevOps.

### 3.1. Phân hệ Tài khoản & Xác thực (`user-service`, `api-gateway`)

| Mã FR | Tên chức năng | Mô tả chi tiết | Mức ưu tiên |
| :--- | :--- | :--- | :--- |
| **FR-01** | Đăng ký tài khoản | Hỗ trợ người dùng đăng ký tài khoản với vai trò Khách hàng (`CUSTOMER`) hoặc Tài xế (`DRIVER`). Mật khẩu được băm an toàn (bcrypt). | **Bắt buộc** |
| **FR-02** | Đăng nhập & Cấp JWT | Xác thực thông tin đăng nhập, sinh access token (JWT) và refresh token. JWT được ký bằng secret trong ENV (dùng chung cho `api-gateway` và `ws-gateway`), chứa `user_id`, `role`, `exp`, và `jti` (JWT ID). TTL của access token mặc định là 1 giờ (cấu hình qua ENV). | **Bắt buộc** |
| **FR-03** | Xác thực & Phân quyền Gateway | **Bảo vệ Gateway:** `api-gateway` xóa mọi header `X-User-*` do client gửi lên trước khi tự gắn header `X-User-Id` và `X-User-Role` của chính nó; tuyệt đối không route các endpoint `/internal/*` ra ngoài Internet.<br>**Phân quyền:** Endpoint công khai chỉ gồm: đăng ký, đăng nhập, làm mới phiên (ước tính cước yêu cầu JWT, áp dụng cho mọi vai trò). Mọi endpoint của Admin (kể cả bảng giá và hủy cưỡng bức) đều nằm dưới `/api/v1/admin/*` để Gateway áp dụng một quy tắc kiểm tra role `ADMIN` duy nhất. Kiểm tra chữ ký JWT, thời hạn `exp`, và tra cứu Redis `blacklist:<jti>` cho mỗi request (nằm trong blacklist thì trả 401). | **Bắt buộc** |
| **FR-04** | Quản lý trạng thái Tài xế | Tài xế chuyển đổi trạng thái làm việc: `ONLINE` (sẵn sàng đón khách), `OFFLINE` (nghỉ ngơi), hoặc tự động sang `BUSY` khi đã nhận chuyến. **Tài xế `BUSY` tự động quay lại `ONLINE` khi chuyến `COMPLETED` hoặc `CANCELLED`.**<br>**Ràng buộc:** Tài xế đang `BUSY` không được phép tự chuyển sang `OFFLINE` và bị từ chối đăng xuất cho đến khi hết chuyến. Tài xế `ONLINE` khi đăng xuất (FR-34) sẽ tự động chuyển sang `OFFLINE` và đóng kết nối WebSocket. | **Bắt buộc** |
| **FR-05** | Xem thông tin hồ sơ | Lấy thông tin cá nhân cơ bản (họ tên, email/số điện thoại, vai trò, trạng thái). | **Nên có** |
| **FR-06** | Quản lý phương tiện tài xế | Lưu trữ thông tin xe (biển số, loại xe máy/ô tô) đơn giản hóa trong bảng tài xế. | **Nên có** |
| **FR-30** | Tài khoản Admin | Tài khoản mang vai trò `ADMIN`, được tạo sẵn tự động khi khởi động service từ biến môi trường (ENV), không cho phép đăng ký tự do qua API. Admin không có ví cá nhân. | **Nên có** |
| **FR-33** | Làm mới phiên (Token Refresh) | Refresh token là chuỗi ngẫu nhiên (opaque string), lưu server-side trong Redis (key kèm TTL, mặc định 7 ngày, ENV). **Quy tắc sử dụng 1 lần:** Refresh token chỉ được dùng đúng 1 lần; nếu có 2 request gửi đồng thời dùng cùng một refresh token thì chỉ duy nhất 1 request thành công, request còn lại bị từ chối 401. Cấp cặp token mới và xóa ngay token cũ khỏi Redis (Rotation). Refresh token chỉ trả trong body JSON (không dùng cookie). | **Bắt buộc** |
| **FR-34** | Đăng xuất & Thu hồi token | Endpoint đăng xuất (`POST /api/v1/auth/logout`): Client gửi kèm refresh token trong body JSON và access token trên header; hệ thống kiểm tra refresh token đó phải thuộc đúng người gọi. Xóa refresh token khỏi Redis và đưa `jti` của access token vào blacklist Redis (key `blacklist:<jti>`, TTL bằng thời gian còn lại của token).<br>**Quy tắc kết nối & trạng thái:** Đăng xuất thành công sẽ **đóng kết nối WebSocket của mọi vai trò (cả Khách hàng và Tài xế)**. Đối với tài xế: nếu đang `BUSY` thì bị từ chối đăng xuất cho đến khi hết chuyến; nếu đang `ONLINE` thì khi đăng xuất thành công sẽ tự động chuyển sang trạng thái `OFFLINE`. | **Nên có** |

### 3.2. Phân hệ Định vị & Vị trí Thời gian thực (`ws-gateway`, `location-service`)

| Mã FR | Tên chức năng | Mô tả chi tiết | Mức ưu tiên |
| :--- | :--- | :--- | :--- |
| **FR-07** | Kết nối WebSocket thời gian thực | `ws-gateway` duy trì kết nối WebSocket hai chiều persistent (`/ws/`). **Xác thực token:** Token được gửi qua header `Authorization` (`Bearer <token>`) lúc bắt tay kết nối WebSocket (handshake), **không dùng query string**. Kiểm tra chữ ký, hạn `exp` và blacklist trên Redis lúc handshake. Kết nối WebSocket đang mở không bị ngắt khi access token hết hạn sau đó. | **Bắt buộc** |
| **FR-08** | Tiếp nhận & Lưu tọa độ GPS | Tiếp nhận stream tọa độ `(lat, lng)` từ tài xế gửi lên mỗi 3–5 giây qua WebSocket, đẩy vào `location-service` để cập nhật vào `Redis GEO` (`GEOADD`). | **Bắt buộc** |
| **FR-09** | Quét tài xế lân cận | **Phân tách trách nhiệm (Database-per-Service):**<br>1. `location-service` tìm kiếm các ứng viên có GPS còn mới trong vòng 15 giây, nằm trong bán kính $R$ km (mặc định $R = 5\text{ km}$), sắp xếp theo khoảng cách tăng dần và lấy khoảng 50 ứng viên gần nhất.<br>2. Việc lọc tài xế `ONLINE` do `dispatch-service` đảm nhiệm: gọi sang endpoint nội bộ của `user-service` một lần (truyền danh sách ID ứng viên, nhận về danh sách ID đang `ONLINE`), sau đó cắt lấy Top 3–5 tài xế rảnh gần nhất.<br>*Lý do:* Trạng thái tài xế (`ONLINE`/`OFFLINE`/`BUSY`) do `user-service` sở hữu, `location-service` không truy cập DB của `user-service`. | **Bắt buộc** |
| **FR-10** | Lưu lịch sử tọa độ vào DB | Ghi toàn bộ vết di chuyển GPS vào PostgreSQL (`locationdb`). | **Bỏ qua** *(Tiết kiệm I/O đĩa và RAM VPS, chỉ lưu tọa độ mới nhất trong Redis; không tạo `locationdb`)* |
| **FR-35** | Đẩy thông báo trạng thái chuyến | Khách hàng và tài xế đều kết nối tới `/ws/` qua `ws-gateway` (xác thực token lúc handshake như FR-07). Mỗi lần chuyến xe thay đổi trạng thái, `dispatch-service` phát event qua Redis Pub/Sub, `ws-gateway` đẩy thông báo cập nhật trạng thái thời gian thực tới kết nối WebSocket của khách hàng và tài xế của chuyến đó. | **Nên có** |
| **FR-38** | Khách nhận tọa độ tài xế thời gian thực | Từ thời điểm chuyến xe chuyển sang trạng thái `ACCEPTED` cho đến khi kết thúc chuyến (`COMPLETED` hoặc `CANCELLED`), hệ thống chuyển tiếp tọa độ GPS mới nhất của tài xế tới kết nối WebSocket của khách hàng thuộc chuyến xe đó. Độ trễ đầu-cuối từ lúc tài xế gửi tọa độ đến khi khách nhận được đạt $< 500\text{ ms}$ (quy định tại NFR-01). Cách thức cài đặt chi tiết do LLD quyết định. | **Bắt buộc** |

### 3.3. Phân hệ Tính giá & Định tuyến (`pricing-service`)

| Mã FR | Tên chức năng | Mô tả chi tiết | Mức ưu tiên |
| :--- | :--- | :--- | :--- |
| **FR-11** | Tính khoảng cách & ETA có Fallback | **Cơ chế cốt lõi:** Gọi OSRM Public API với timeout **400 ms** để lấy khoảng cách thực tế và thời gian di chuyển. Nếu OSRM bị lỗi/timeout $\rightarrow$ **Tự động Fallback sang công thức Haversine $\times 1.35$** (hệ số uốn khúc đô thị). | **Bắt buộc** |
| **FR-12** | Ước tính cước phí chuyến xe | Tính giá cước chuyến theo công thức: $\text{Fare} = (\text{BaseFare} + \text{Distance} \times \text{PricePerKm}) \times \text{SurgeMultiplier}$.<br>**Quy tắc làm tròn:** Cước phí được **làm tròn thành số nguyên VNĐ, tới hàng nghìn gần nhất** (ví dụ: 23,400đ $\rightarrow$ 23,000đ; 23,600đ $\rightarrow$ 24,000đ).<br>**Dữ liệu trả về:** Response của API ước tính cước (yêu cầu JWT) trả về đầy đủ: khoảng cách (`distance`), thời gian dự kiến (`eta`), nguồn tính khoảng cách (`source`: `OSRM` hoặc `HAVERSINE`), hệ số nhu cầu (`surge_multiplier`), `base_fare`, `price_per_km` và tổng cước phí (`total_fare`). | **Bắt buộc** |
| **FR-13** | Tính hệ số Surge Pricing (Geohash Grid) | **Tính toán Cung/Cầu đơn giản theo ô lưới:**<br>- "Ô lưới" được xác định là ô **Geohash độ dài 5** của điểm đón khách, áp dụng cho cả Demand và Supply.<br>- `Demand`: Số chuyến tạo trong 5 phút gần nhất trong cùng ô Geohash đó (`pricing-service` lắng nghe event `TripCreated`).<br>- `Supply`: Số tài xế `ONLINE` có cập nhật GPS mới trong 15 giây gần nhất thuộc cùng ô Geohash đó (`pricing-service` gọi `location-service` lấy ID tài xế trong ô, sau đó gọi `user-service` lọc các ID đang `ONLINE` như quy trình FR-09).<br>- Gọi $T$ là ngưỡng surge (mặc định $T = 1.0$, đọc từ ENV, có thể điều chỉnh lúc chạy qua FR-31):<br>$$\text{SurgeMultiplier} = \text{clamp}\left(\frac{\text{Demand} / \text{Supply}}{T}, 1.0, 1.5\right)$$<br>- Trường hợp biên: Nếu $\text{Supply} = 0$ và $\text{Demand} > 0 \rightarrow \text{SurgeMultiplier} = 1.5$; nếu $\text{Demand} = 0 \rightarrow \text{SurgeMultiplier} = 1.0$. Giá trị tăng tuyến tính liên tục, không có khoảng trống giữa các bậc. Không cache kết quả tính surge (bảng giá BaseFare/PricePerKm vẫn cache Redis).<br>*Ghi chú demo:* Khi demo với 100 tài xế ảo, Admin có thể hạ ngưỡng $T$ xuống thấp để kích hoạt hệ số surge tăng lên. | **Bắt buộc** |
| **FR-14** | Áp dụng Voucher / Khuyến mãi | Trừ tiền voucher giảm giá, mã khuyến mãi đa tầng. | **Bỏ qua** *(Đơn giản hóa cho demo)* |
| **FR-31** | Quản lý bảng giá (Admin) | Admin xem và cập nhật `BaseFare`, `PricePerKm`, ngưỡng surge $T$ qua endpoint `/api/v1/admin/pricing/config`. Khi chưa kích hoạt FR-31, các tham số này đọc mặc định từ ENV. Dữ liệu lưu trong `pricingdb` và cache Redis. Giá mới chỉ áp dụng cho các chuyến đi được tạo sau thời điểm cập nhật. | **Nên có** |

### 3.4. Phân hệ Điều phối & Khớp chuyến (`dispatch-service`)

| Mã FR | Tên chức năng | Mô tả chi tiết | Mức ưu tiên |
| :--- | :--- | :--- | :--- |
| **FR-15** | Kiểm tra số dư ví & Chặn đa chuyến | **Kiểm tra số dư:** Kiểm tra số dư ví của khách qua `payment-service`. Nếu số dư ví nhỏ hơn giá tạm tính $\rightarrow$ **Chặn không cho đặt xe**, trả về mã lỗi `INSUFFICIENT_BALANCE`.<br>**Chặn đa chuyến:** Mỗi khách hàng chỉ được có duy nhất 1 chuyến đang hoạt động (trạng thái `CREATED`, `MATCHING`, `ACCEPTED`, `PICKING_UP`, `IN_TRIP`). Nếu đã có thì chặn, trả mã lỗi `ACTIVE_TRIP_EXISTS`.<br>**Đánh giá độ chính xác & Giới hạn đã biết:** Kiểm tra số dư lúc đặt xe là đủ trong điều kiện vận hành bình thường; hạn chế đã biết là khách có thể đặt chuyến mới ngay khi chuyến trước vừa `COMPLETED` nhưng hệ thống thanh toán chưa kịp trừ tiền ví, khi đó lần thanh toán sau có thể bị `FAILED` (FR-24 đã xử lý tình huống này). Kịch bản demo sẽ chờ ví cập nhật xong rồi mới đặt tiếp. | **Bắt buộc** |
| **FR-16** | Tạo yêu cầu đặt xe & Chốt giá | Khách hàng gửi điểm đón, điểm trả $\rightarrow$ hệ thống gọi `pricing-service` lấy giá, tạo bản ghi chuyến với trạng thái ban đầu `CREATED` $\rightarrow$ chuyển sang `MATCHING`. **`CREATED` và `MATCHING` được tạo trong cùng một database transaction.** Giá chốt lúc tạo chuyến được lưu cố định vào bản ghi chuyến đi và là giá cuối cùng, không tính lại khi hoàn thành.<br>**Lưu bền hạn tìm xe:** Hạn tìm xe 30 giây được tính toán và lưu bền vững vào bản ghi chuyến trong DB (không chỉ lưu trong RAM). | **Bắt buộc** |
| **FR-17** | Broadcast đề nghị nhận chuyến | `dispatch-service` gọi `location-service` lấy danh sách ứng viên gần nhất, gọi `user-service` lọc các tài xế đang `ONLINE`, lấy Top 3–5 tài xế rảnh gần nhất và lưu danh sách tài xế được mời này vào chuyến xe. Phát thông báo mời cuốc qua Redis Pub/Sub $\rightarrow$ `ws-gateway` đẩy xuống WebSocket của các tài xế này.<br>**Đặc biệt:** Nếu quét ra 0 tài xế phù hợp, chuyến xe chuyển ngay sang trạng thái `EXPIRED` mà không cần chờ hết 30 giây. | **Bắt buộc** |
| **FR-18** | Nhận chuyến cạnh tranh (Atomic Lock) | Tài xế gửi request nhận cuốc.<br>**Các quy tắc nghiệp vụ bắt buộc:**<br>(a) **Chỉ tài xế nằm trong danh sách được mời** của chuyến mới được nhận chuyến; nếu không thuộc danh sách mời thì trả lỗi `NOT_OFFERED`.<br>(b) **Một tài xế không thể giữ 2 chuyến cùng lúc:** Tài xế phải đang `ONLINE`; nếu đang `BUSY`/`OFFLINE` hoặc đang giữ chuyến khác thì từ chối với lỗi `DRIVER_NOT_AVAILABLE`.<br>(c) **Tính toàn vẹn trạng thái nguồn:** Chuyển trạng thái chỉ thành công khi chuyến đang ở đúng trạng thái nguồn hợp lệ (`MATCHING`).<br>(d) **Xử lý race condition:** Hệ thống dùng Redis Atomic Lock: `SET ride:lock:<trip_id> <driver_id> NX EX 10`. Tài xế đầu tiên nhận thành công; các tài xế đến sau nhận lỗi `TRIP_ALREADY_TAKEN`. Nếu lock Redis hết hạn mà tài xế gửi request đến muộn thì trả lỗi `TRIP_ALREADY_TAKEN` và tuyệt đối không thay đổi trạng thái tài xế sang `BUSY`. (Thứ tự chi tiết do UC/LLD quyết định). | **Bắt buộc** |
| **FR-19** | Quản lý Máy trạng thái Chuyến | Đảm bảo chuyển trạng thái nghiêm ngặt: $\text{CREATED} \rightarrow \text{MATCHING} \rightarrow \text{ACCEPTED} \rightarrow \text{PICKING\_UP} \rightarrow \text{IN\_TRIP} \rightarrow \text{COMPLETED}$. Mọi chuyển đổi trạng thái chỉ thành công khi chuyến đang ở đúng trạng thái nguồn hợp lệ. | **Bắt buộc** |
| **FR-20** | Hủy chuyến xe | Quy định quyền hủy chuyến theo trạng thái nguồn hợp lệ:<br>- Khách hàng chỉ được hủy khi chuyến đang ở `MATCHING` hoặc `ACCEPTED`.<br>- Tài xế chỉ được hủy khi chuyến đang ở `ACCEPTED` hoặc `PICKING_UP` (gặp sự cố bất khả kháng).<br>- Khi chuyến đã chuyển sang `IN_TRIP`, **cả khách hàng và tài xế đều không được phép hủy**.<br>Khi hủy hợp lệ, trạng thái chuyến chuyển sang `CANCELLED`. | **Bắt buộc** |
| **FR-21** | Hết giờ tìm tài xế (Quét bền vững) | Hạn tìm xe 30 giây được lưu bền trong DB. `dispatch-service` có **tiến trình quét nền định kỳ (vài giây một lần)** để chuyển `EXPIRED` các chuyến `MATCHING` đã quá hạn. Tiến trình này cũng chạy ngay khi service khởi động lại để đảm bảo việc restart hoặc rolling update container không bao giờ làm chuyến xe bị kẹt vĩnh viễn ở `MATCHING`. | **Bắt buộc** |
| **FR-36** | Xem chuyến hiện tại & Chi tiết chuyến | Cung cấp API cho phép Khách hàng hoặc Tài xế xem thông tin chuyến đang hoạt động (`active trip`) và xem chi tiết một chuyến (`trip details`). Người dùng chỉ xem được chuyến của chính mình. **Admin không dùng endpoint này (Admin xem chuyến qua FR-32).**<br>**Chi tiết chuyến:** Bao gồm đầy đủ thông tin chuyến và mảng lịch sử chuyển trạng thái (`status_timeline`) có ghi nhận thời gian (timestamp) của từng lần đổi trạng thái. | **Nên có** |

### 3.5. Phân hệ Ví & Thanh toán (`payment-service`)

| Mã FR | Tên chức năng | Mô tả chi tiết | Mức ưu tiên |
| :--- | :--- | :--- | :--- |
| **FR-22** | Quản lý ví cá nhân (Lazy Creation) | **Khởi tạo ví kiểu Lazy:** Ví không tạo tự động lúc đăng ký mà được khởi tạo khi người dùng truy cập ví lần đầu hoặc khi phát sinh giao dịch đầu tiên. Thao tác tạo ví phải đảm bảo **nguyên tử (atomic, duy nhất theo `user_id`)** để chống tạo trùng lặp. Số dư ban đầu bằng 0. Admin không có ví cá nhân.<br>Cung cấp API xem số dư và lịch sử giao dịch chi tiết theo từng dòng riêng biệt có mã tham chiếu `trip_id` và trạng thái `SUCCESS` hoặc `FAILED`. | **Bắt buộc** |
| **FR-23** | Nạp tiền ví ảo (Top-up API) | Cung cấp API nạp tiền trực tiếp vào ví để phục vụ kịch bản test API demo. Tiền nạp là số nguyên VNĐ. | **Bắt buộc** |
| **FR-24** | Thanh toán chuyến đi tự động | Lắng nghe sự kiện `TripCompleted` từ Redis Streams:<br>- Trừ đúng giá cước đã chốt lúc tạo chuyến từ ví khách.<br>- Hoa hồng hệ thống: $\text{Commission} = \text{round}(\text{giá chốt} \times 15\%)$. Tài xế nhận phần còn lại: $\text{DriverIncome} = \text{giá chốt} - \text{Commission}$.<br>- **Hoa hồng chỉ là một dòng sổ giao dịch loại `COMMISSION`, không có ví hệ thống.**<br>- **Ba dòng giao dịch của một chuyến** (trừ ví khách, cộng ví tài xế, hoa hồng hệ thống) được ghi và cập nhật số dư ví trong **cùng một database transaction**, ràng buộc duy nhất theo `(trip_id, loại dòng)` để chống trừ trùng.<br>- Tất cả số tiền đều là **số nguyên VNĐ**.<br>**Quy tắc số dư:** Không bao giờ để số dư ví âm. Nếu vì lý do bất thường mà ví khách không đủ lúc trừ, hệ thống không trừ tiền, đánh dấu giao dịch `FAILED` và ghi log lỗi để xử lý thủ công. | **Bắt buộc** |
| **FR-25** | Miễn phí phạt hủy chuyến | **(Quyết định đã chốt):** Khi chuyến chuyển sang `CANCELLED` (kể cả do Khách, Tài xế hay Admin hủy cưỡng bức), hệ thống không trừ bất kỳ phí phạt nào từ tài khoản khách hoặc tài xế. | **Bắt buộc** |
| **FR-26** | Cổng thanh toán thẻ thực tế | Tích hợp cổng thanh toán thực tế (VNPAY, Momo, Stripe). | **Bỏ qua** |

### 3.6. Phân hệ Phân tích & Báo cáo (`ai-service`)

| Mã FR | Tên chức năng | Mô tả chi tiết | Mức ưu tiên |
| :--- | :--- | :--- | :--- |
| **FR-27** | Thu thập số liệu vận hành | Lắng nghe sự kiện từ Redis Streams (`TripCreated` do `dispatch-service` phát, `TripCompleted`, `TripCancelled`, và `TripExpired`) để tổng hợp số liệu vào `aidb`:<br>- **Tính lũy thừa (Idempotency):** Đảm bảo xử lý duy nhất theo cặp `(trip_id, loại event)` (nếu event bị giao lại thì không đếm trùng lặp).<br>- Số chuyến theo trạng thái (`COMPLETED`, `CANCELLED`, `EXPIRED`).<br>- **Công thức tỷ lệ:** Mẫu số của tỷ lệ hủy và tỷ lệ hết giờ là **tổng số chuyến đã kết thúc** ($\text{COMPLETED} + \text{CANCELLED} + \text{EXPIRED}$).<br>- Tổng doanh thu và tổng hoa hồng hệ thống thu được.<br>- **Múi giờ:** Thời gian lưu trữ trong DB theo chuẩn UTC; các mốc giờ cao điểm khi kết xuất báo cáo hiển thị theo múi giờ Việt Nam (`Asia/Ho_Chi_Minh`). Hỗ trợ lọc theo khoảng thời gian (`start_time`, `end_time`). | **Bắt buộc** |
| **FR-28** | Sinh nhận xét vận hành (AI/Rule) | Cung cấp endpoint báo cáo tổng kết (`GET /api/v1/admin/reports` - dùng chung với FR-32 mục 3, chỉ role `ADMIN` mới được gọi): Response bao gồm số liệu thống kê vận hành và nhận xét phân tích, kèm trường `insight_source` nhận giá trị `LLM` hoặc `RULE_BASED`.<br>**Cơ chế Chịu lỗi:** Nếu gọi mô hình LLM gặp lỗi hoặc timeout (mặc định 10 giây, cấu hình ENV) thì hệ thống tự động fallback sang cơ chế `RULE_BASED` theo tập luật, đảm bảo không bao giờ trả lỗi 500 cho Admin. Tỷ lệ hoa hồng 15% giữ cố định ở ENV, không có API đổi. | **Bắt buộc** |
| **FR-29** | Dự đoán nhu cầu di chuyển sâu | Huấn luyện mạng nơ-ron Deep Learning (LSTM/GRU) dự báo mật độ chuyến xe. | **Bỏ qua** *(Không phù hợp trên VPS 1 GB RAM)* |

### 3.7. Phân hệ Quản trị (Admin)

> **Quy ước Endpoint:** Toàn bộ API của Admin nằm thống nhất dưới đường dẫn `/api/v1/admin/*`.

| Mã FR | Tên chức năng | Mô tả chi tiết | Mức ưu tiên |
| :--- | :--- | :--- | :--- |
| **FR-32** | Xem dữ liệu vận hành (Admin) | Tuân thủ kiến trúc Database-per-Service: Admin chỉ đọc dữ liệu vận hành (read-only), mỗi loại dữ liệu do service sở hữu trực tiếp trả về qua endpoint dành cho role `ADMIN` (ngoại lệ duy nhất cho phép quyền ghi là FR-37):<br>1. Danh sách người dùng do `user-service` quản lý và trả về (`/api/v1/admin/users`).<br>2. Danh sách chuyến do `dispatch-service` quản lý và trả về (`/api/v1/admin/trips`).<br>3. Báo cáo vận hành do `ai-service` quản lý và trả về (`/api/v1/admin/reports`).<br>4. Danh sách tài xế đang hoạt động và vị trí GPS mới nhất do `location-service` trả về (`/api/v1/admin/drivers/active`): **"tài xế đang hoạt động" được định nghĩa là tài xế có cập nhật GPS trong 15 giây gần nhất** (Admin xem trạng thái `ONLINE`/`BUSY` ở danh sách người dùng của `user-service`).<br>Tất cả request đều đi qua `api-gateway` và chỉ tài khoản có role `ADMIN` mới được phép gọi. | **Nên có** |
| **FR-37** | Admin hủy cưỡng bức chuyến kẹt | Cho phép Admin hủy cưỡng bức bất kỳ chuyến đi nào chưa kết thúc (đang ở trạng thái `CREATED`, `MATCHING`, `ACCEPTED`, `PICKING_UP`, hoặc `IN_TRIP`) qua endpoint `/api/v1/admin/trips/:id/force-cancel` để giải phóng tài nguyên hệ thống khi phát hiện chuyến bị kẹt hoặc sự cố. Chuyến xe chuyển sang trạng thái `CANCELLED`, tài xế (nếu đã nhận chuyến) tự động quay lại trạng thái `ONLINE`, không trừ bất kỳ phí phạt nào (tuân thủ FR-25). Request đi qua `api-gateway`, chỉ tài khoản có role `ADMIN` mới có quyền thực hiện. | **Nên có** |

---

## 4. MÁY TRẠNG THÁI CHUYẾN XE (TRIP STATE MACHINE)

Sơ đồ chuyển trạng thái nghiêm ngặt trong `dispatch-service`:

```mermaid
stateDiagram-v2
    [*] --> CREATED: Khách tạo yêu cầu (Ví đủ tiền, chưa có chuyến active)
    CREATED --> MATCHING: Bắt đầu tìm tài xế (Cùng 1 DB transaction)
    
    MATCHING --> ACCEPTED: Tài xế trong danh sách mời bấm Accept (Redis Lock NX; Stream GPS cho khách - FR-38)
    MATCHING --> EXPIRED: Hết 30 giây không ai nhận HOẶC quét ra 0 tài xế (Quét nền DB bền vững)
    MATCHING --> CANCELLED: Khách hủy (khi chưa có tài xế)
    
    ACCEPTED --> PICKING_UP: Tài xế bắt đầu di chuyển đón khách
    ACCEPTED --> CANCELLED: Khách hoặc tài xế hủy chuyến
    
    PICKING_UP --> IN_TRIP: Tài xế đã đón khách lên xe
    PICKING_UP --> CANCELLED: Tài xế hủy chuyến (gặp sự cố bất khả kháng)
    
    IN_TRIP --> COMPLETED: Tài xế trả khách tại điểm đến (Không ai được hủy ở IN_TRIP)
    
    COMPLETED --> [*]: Bắn event TripCompleted (Trừ tiền khách theo giá đã chốt, chia hoa hồng), tài xế quay lại ONLINE, dừng stream GPS
    EXPIRED --> [*]
    CANCELLED --> [*]: Không trừ phí phạt, tài xế quay lại ONLINE (nếu đã nhận chuyến), dừng stream GPS
```

> **Ghi chú về Admin hủy cưỡng bức (FR-37):**  
> Admin có đặc quyền thực hiện lệnh hủy cưỡng bức tại **mọi trạng thái chưa kết thúc** (`CREATED`, `MATCHING`, `ACCEPTED`, `PICKING_UP`, `IN_TRIP`). Khi Admin gửi lệnh hủy, chuyến đi lập tức chuyển sang trạng thái `CANCELLED`, tài xế (nếu đã được gán) tự động quay lại `ONLINE`, và không áp dụng bất kỳ khoản phí phạt nào (nhằm giữ cho sơ đồ trực quan và dễ theo dõi, nhánh hủy cưỡng bức từ Admin không vẽ thêm các đường mũi tên trùng lặp).

---

## 5. YÊU CẦU PHI CHỨC NĂNG (NON-FUNCTIONAL REQUIREMENTS - NFR)

### NFR-01: Hiệu năng & Độ trễ (Performance & Latency)
* Độ trễ cập nhật GPS từ tài xế qua WebSocket đến Redis GEO đạt $< 500\text{ ms}$.
* **Độ trễ đầu-cuối của tính năng stream vị trí tài xế tới khách hàng (FR-38):** Từ thời điểm tài xế gửi tọa độ GPS lên WebSocket đến khi khách hàng nhận được qua WebSocket đạt $< 500\text{ ms}$.
* Thời gian phản hồi các REST API nội bộ đạt $< 100\text{ ms}$ ở điều kiện bình thường (**mốc 100 ms này không tính thời gian gọi OSRM bên ngoài, vì OSRM có cấu hình timeout ngắt riêng là $400\text{ ms}$**).
* Fallback nội bộ bằng Haversine $\times 1.35$ phản hồi tức thì $< 1\text{ ms}$.

### NFR-02: Ràng buộc tài nguyên (VPS Constraints - Tối hậu)
* Toàn bộ hệ thống chạy trên **1 VPS duy nhất: 1 vCPU, 1 GB RAM, 20 GB Disk, Ubuntu 22.04 LTS**.
* Sử dụng 1 container PostgreSQL duy nhất, chia **5 Database logic riêng biệt** (`userdb`, `dispatchdb`, `pricingdb`, `paymentdb`, `aidb`) theo mô hình Database-per-Service. `locationdb` không sử dụng do FR-10 đã bỏ qua (chỉ lưu tọa độ mới nhất trong Redis GEO để tối ưu RAM và I/O).
* Sử dụng 1 container Redis duy nhất đảm nhận cả 3 vai trò: GEO, Pub/Sub, Streams (thay thế Kafka).
* Tất cả container trong `docker-compose.yml` đều được gán `mem_limit` cụ thể (từ 25–40 MB cho mỗi Go service (đo thực tế khi deploy); Postgres tối đa 150 MB; Redis tối đa 60 MB) kèm bật 2 GB Swap để chống OOM (Out-Of-Memory) Killer.

### NFR-03: Tính sẵn sàng & Khả năng chịu lỗi (Resilience & Fault Tolerance)
* Khi server OSRM bên ngoài gặp sự cố, hệ thống tự động fallback tính khoảng cách qua Haversine mà không gây lỗi cho người dùng.
* Khi gọi LLM cho báo cáo thông minh tại `ai-service` bị lỗi hoặc timeout quá 10 giây, hệ thống tự động chuyển sang chế độ phân tích dựa trên tập luật (Rule-based) với `insight_source: RULE_BASED` mà không trả lỗi cho Admin.
* **Quản lý Redis Streams:** Luồng sự kiện Redis Streams được giới hạn độ dài (`MAXLEN` xấp xỉ vài nghìn tin nhắn để tránh tràn RAM VPS). Cơ chế Consumer Group + `ACK` rõ ràng; **mỗi consumer group tự động đọc lại các tin nhắn đang treo (pending messages) khi khởi động lại**. Đảm bảo tính lũy thừa (Idempotency) theo cặp `(trip_id, loại event)` tại `payment-service` và `ai-service` để tránh xử lý trùng lặp.
* Khóa phân tán Redis `SET NX` có TTL 10 giây để chống rò rỉ khóa (Deadlock) nếu tài xế ngắt mạng giữa chừng.

### NFR-04: An toàn & Phân vùng mạng (Security & Network Isolation)
* **Nginx chạy trực tiếp trên Host VPS** là cổng DUY NHẤT mở kết nối ra Internet qua cổng 80 (HTTP redirect) và 443 (HTTPS/WSS có chứng chỉ Let's Encrypt / DuckDNS).
* Hai gateway publish dạng cục bộ `127.0.0.1:8080` (`api-gateway`) và `127.0.0.1:8081` (`ws-gateway`), tuyệt đối không mở `0.0.0.0` để bảo vệ qua UFW.
* Toàn bộ các service nghiệp vụ, PostgreSQL, và Redis chỉ giao tiếp qua mạng nội bộ Docker (`backend-net`), không mở bất kỳ cổng nào ra máy chủ hay Internet.
* **Cơ chế xác thực & thu hồi:** JWT ký bằng secret chung qua ENV. Access token mang `jti` được kiểm tra blacklist tức thời qua Redis để hỗ trợ thu hồi token khi đăng xuất. Refresh token áp dụng Rotation và lưu server-side trong Redis với TTL 7 ngày.
* **Cơ chế Fail-Closed:** Khi Redis gặp sự cố hoặc mất kết nối, `api-gateway` **từ chối request (fail-closed)** để đảm bảo an toàn tuyệt đối, không cho phép token đã bị thu hồi lọt qua.

### NFR-05: Khả năng tải Demo & Khả năng phục hồi (Scalability & Simulation Load)
* Hệ thống chịu tải mượt mà kịch bản giả lập **100 tài xế ảo** kết nối WebSocket đồng thời và gửi tọa độ GPS liên tục mỗi 3 giây mà mức sử dụng RAM toàn hệ thống không vượt quá 850 MB.
* **Tự động kết nối lại (Auto-reconnect Backoff):** Script giả lập 100 tài xế ảo được lập trình có cơ chế tự động kết nối lại WebSocket với thuật toán Exponential Backoff khi bị ngắt kết nối (do quá trình Rolling Update hoặc khởi động lại container).

### NFR-06: Giám sát & Quản trị (Observability)
* Log chuẩn xuất ra `stdout`/`stderr` dưới định dạng JSON có cấu trúc.
* Xem log trực tiếp qua Dozzle (bind `127.0.0.1`, truy cập qua SSH Tunnel hoặc Nginx Basic Auth).
* Bộ giám sát Prometheus + node_exporter + cAdvisor được cấu hình theo docker profile riêng (`--profile monitoring`) để bật/tắt linh hoạt khi cần đo đạc tài nguyên.

---

## 6. NGOÀI PHẠM VI (OUT OF SCOPE)

Nhằm đảm bảo dự án tập trung cao độ vào tiêu chí DevOps, Microservices và giới hạn 1 GB RAM của VPS, các tính năng sau đây được **loại bỏ hoàn toàn khỏi phạm vi thực hiện**:
1. **Giao diện người dùng (Frontend UI):** Không viết ứng dụng Mobile (Android/iOS) hay Web React/Vue. Toàn bộ thao tác (kể cả Admin) thực hiện qua cURL / Postman.
2. **Cổng thanh toán thật:** Không tích hợp các cổng thanh toán ngân hàng/ví điện tử thương mại (VNPAY, Momo, ZaloPay, Stripe). Sử dụng ví nội bộ và API nạp tiền ảo.
3. **Engine định tuyến nặng nội bộ:** Không tự cài đặt OSRM C++ hay GraphHopper Java trên VPS do thiếu hụt RAM nghiêm ngặt.
4. **Hệ thống hạ tầng nặng:** Không sử dụng Apache Kafka, Zookeeper, Elasticsearch, Logstash, Kibana (EFK stack).
5. **Tính năng tiện ích mở rộng:** Đánh giá sao (Rating/Review), Chat realtime giữa khách và tài xế, quản lý mã voucher giảm giá phức tạp, đặt trước lịch hẹn chuyến đi, **giao diện bản đồ đồ họa hiển thị trực quan (Map UI)** (hệ thống chỉ cung cấp dữ liệu thô stream tọa độ qua WebSocket theo FR-38).
6. **Đăng ký Admin tự do & Thay đổi hoa hồng động:** Không cho phép đăng ký tài khoản Admin tự do (khởi tạo qua ENV). Không có API thay đổi tỷ lệ hoa hồng (cố định 15% trong ENV).
7. **Cơ chế xác thực nâng cao:** Không lưu refresh token bằng cookie `httpOnly` (do không làm UI), không hỗ trợ quản lý đăng nhập đa thiết bị nâng cao hay cơ chế thu hồi toàn bộ phiên (single sign-out toàn bộ thiết bị) của một người dùng.
