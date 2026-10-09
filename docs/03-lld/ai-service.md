# THIẾT KẾ CHI TIẾT DỊCH VỤ PHÂN TÍCH & BÁO CÁO AI (AI-SERVICE LLD)

> **Mã tài liệu:** `LLD-AI` | **Service:** `ai-service` | **Database:** `aidb` (PostgreSQL)  
> **Tài liệu tham chiếu:** [AGENTS.md](../../AGENTS.md), [SRS v1.5](../01-srs/srs.md), [Hợp đồng liên service](../03-lld/00-hop-dong-lien-service.md), [UC-06 (toàn bộ)](../02-use-cases/uc-06-thong-ke-ai.md), [UC-07 (UC-29)](../02-use-cases/uc-07-admin.md)

---

## 1. TRÁCH NHIỆM

- Tiêu thụ bất đồng bộ cả 4 loại sự kiện vòng đời chuyến đi (`TripCreated`, `TripCompleted`, `TripCancelled`, `TripExpired`) từ `stream:trip_events` qua consumer group `ai-group`.
- Tổng hợp và lưu trữ dữ liệu vào `aidb`, đảm bảo tính lũy thừa (Idempotency) theo cặp `(trip_id, event_type)` và thời gian chuẩn UTC.
- Cung cấp API báo cáo quản trị `GET /api/v1/admin/reports` (Tầng 2) trích xuất số liệu vận hành: số chuyến theo trạng thái, doanh thu, hoa hồng, tỷ lệ hủy, tỷ lệ hết giờ, khung giờ cao điểm theo múi giờ `Asia/Ho_Chi_Minh` (UTC+7), hỗ trợ lọc theo `start_time` và `end_time`.
- Sinh nhận xét phân tích kinh doanh bằng Google Gemini API (generateContent) (timeout 10 giây); tự động Fallback sang tập luật định sẵn (`RULE_BASED`) khi không có API key hoặc khi LLM lỗi/timeout, đảm bảo không bao giờ trả lỗi 500 cho Admin.

---

## 2. BẢNG DB (AIDB)

### 2.1. Bảng `trip_events_analytics` (Dữ liệu sự kiện vận hành)
| Tên cột | Kiểu dữ liệu | Ràng buộc | Mục đích & Ghi chú |
| :--- | :--- | :--- | :--- |
| `id` | `BIGINT` | PK, GENERATED ALWAYS AS IDENTITY | Định danh dòng sự kiện tăng tự động. |
| `trip_id` | `UUID` | NOT NULL | Định danh chuyến xe phát sinh sự kiện. |
| `event_type` | `VARCHAR(20)` | NOT NULL | `TripCreated`, `TripCompleted`, `TripCancelled`, `TripExpired`. |
| `fare` | `BIGINT` | NULLABLE | Cước phí chuyến đi (VND, chỉ có ở `TripCompleted`). |
| `commission` | `BIGINT` | NULLABLE | Hoa hồng hệ thống tự tính (VND, chỉ có ở `TripCompleted`). |
| `event_time` | `TIMESTAMPTZ` | NOT NULL | Mốc thời gian phát sinh sự kiện UTC trích xuất từ event. |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Thời gian ghi nhận vào cơ sở dữ liệu UTC. |

- **Chỉ mục & Ràng buộc toàn vẹn:**
  - PK: `id`.
  - UNIQUE Idempotency: `UNIQUE (trip_id, event_type)` (Ngăn chặn xử lý lặp lại cùng một sự kiện).
  - CHECK event_type: `CHECK (event_type IN ('TripCreated', 'TripCompleted', 'TripCancelled', 'TripExpired'))`.
  - `idx_analytics_event_time`: `CREATE INDEX ... ON trip_events_analytics (event_time)` (Tối ưu lọc báo cáo theo khoảng thời gian).
  - `idx_analytics_type_time`: `CREATE INDEX ... ON trip_events_analytics (event_type, event_time)`.

---

## 3. API SPECIFICATION

Định dạng JSON chuẩn: `{"success": bool, "data": ..., "error": {"code": string, "message": string}}`. Header `X-User-Id` và `X-User-Role` do Gateway chuyển tiếp vào.

### 3.1. API Công khai (Public Endpoints)

| Method & Path | Tầng | Actor | Request Fields | Response Data Fields | Mã lỗi kích hoạt |
| :--- | :-: | :--- | :--- | :--- | :--- |
| `GET /api/v1/admin/reports` | 2 | `ADMIN` | Query: `start_time` (iso8601, T)<br>`end_time` (iso8601, T) | `metrics` (object: `{total_trips, completed_trips, cancelled_trips, expired_trips, total_ended, total_revenue, total_commission, cancellation_rate, expiration_rate, peak_hours}`)<br>`insights` (string)<br>`insight_source` ("LLM" \| "RULE_BASED") | `FORBIDDEN` (403: role != ADMIN)<br>`VALIDATION_ERROR` (400: sai định dạng ISO 8601 hoặc `start_time > end_time`) |
| `GET /health` *(do LLD đặt)* | 1 | Tất cả | *(None)* | `status` ("ok") | *(None)* |

*(B: Bắt buộc, T: Tùy chọn. Không bao giờ trả lỗi 500: mọi sự cố LLM đều chuyển sang Fallback RULE_BASED).*

### 3.2. API Nội bộ (Internal Endpoints)
`ai-service` **không** cung cấp endpoint nội bộ nào cho service khác gọi. Service chỉ thu thập dữ liệu bất đồng bộ qua Redis Streams và phục vụ API báo cáo cho Admin.

---

## 4. LUỒNG XỬ LÝ TỪNG BƯỚC

### 4.1. Tiêu thụ sự kiện từ Redis Stream (UC-27)
1. Worker của `ai-service` tham gia consumer group `ai-group` lắng nghe stream `stream:trip_events`.
2. **Khởi tạo group (Idempotent):** Thực thi `XGROUP CREATE stream:trip_events ai-group 0 MKSTREAM`. Bỏ qua lỗi `BUSYGROUP`.
3. **Vòng lặp tiêu thụ tin nhắn:**
   - *Quét tồn đọng PEL:* Định kỳ mỗi 2–5 giây trong vòng lặp chính (không chỉ lúc khởi động), gọi `XREADGROUP GROUP ai-group ai-consumer-1 STREAMS stream:trip_events 0` để đọc lại và xử lý hết các message còn treo trong PEL cho tới khi hết.
   - *Đọc tin mới:* Gọi `XREADGROUP GROUP ai-group ai-consumer-1 BLOCK 2000 STREAMS stream:trip_events >`.
4. **Xử lý từng loại sự kiện:**
   - **Phân loại sự kiện:** Đọc trực tiếp trường `type` của message từ stream (`TripCreated`, `TripCompleted`, `TripCancelled`, `TripExpired`).
   - **Xử lý message hỏng:** Nếu message thiếu trường bắt buộc, sai định dạng UUID, `fare <= 0` (đối với `TripCompleted`), hoặc là entry rỗng (do tin nhắn bị xóa bởi cơ chế `MAXLEN ~ 5000` của Redis Stream khi đọc lại từ PEL) $\rightarrow$ log `ERROR` và gọi ngay `XACK stream:trip_events ai-group <msg_id>`.
   - **Kiểm tra tính lũy thừa (Idempotency):**
     `SELECT 1 FROM trip_events_analytics WHERE trip_id = <trip_id> AND event_type = <type> LIMIT 1;`  
     Nếu đã tồn tại $\rightarrow$ Gửi ngay `XACK stream:trip_events ai-group <msg_id>`, bỏ qua xử lý.
   - **Ghi nhận sự kiện hợp lệ:**
     - Với `TripCreated` (`type == "TripCreated"`): Trích xuất `created_at`. Chèn bản ghi với `event_type = 'TripCreated'`, `event_time = created_at`.
     - Với `TripCompleted` (`type == "TripCompleted"`): Trích xuất `fare`, `completed_at`. Tự tính hoa hồng số nguyên từ `fare` và `COMMISSION_RATE` (ENV):  
       $$\text{commission} = \lfloor\frac{\text{fare} \times \text{COMMISSION\_RATE} + 50}{100}\rfloor$$  
       Chèn bản ghi với `event_type = 'TripCompleted'`, `fare = fare`, `commission = commission`, `event_time = completed_at`.
     - Với `TripCancelled` (`type == "TripCancelled"`): Trích xuất `cancelled_at`. Chèn bản ghi với `event_type = 'TripCancelled'`, `event_time = cancelled_at`.
     - Với `TripExpired` (`type == "TripExpired"`): Trích xuất `expired_at`. Chèn bản ghi với `event_type = 'TripExpired'`, `event_time = expired_at`.
   - **Xử lý lỗi DB:** Nếu transaction DB commit thành công $\rightarrow$ gửi `XACK stream:trip_events ai-group <msg_id>` (lỗi XACK chỉ log ERROR). Nếu gặp lỗi DB tạm thời $\rightarrow$ **KHÔNG XACK**, để message nằm lại trong PEL và sẽ được đọc lại sau 2–5 giây ở chu kỳ quét PEL kế tiếp.

### 4.2. Trích xuất số liệu thống kê (UC-28)
1. Kiểm tra header `X-User-Role == 'ADMIN'` (sai $\rightarrow$ `FORBIDDEN` 403).
2. Phân tích query param `start_time`, `end_time` (mặc định lấy 24 giờ gần nhất nếu không truyền). Nếu `start_time > end_time` $\rightarrow$ Trả `VALIDATION_ERROR` (400).
3. Thực thi truy vấn tổng hợp trên `trip_events_analytics` trong khoảng thời gian `[start_time, end_time]`:
   - `total_trips`: Số lượng sự kiện `TripCreated` có `event_time` trong khoảng.
   - `completed_trips`: Số lượng sự kiện `TripCompleted` có `event_time` trong khoảng.
   - `cancelled_trips`: Số lượng sự kiện `TripCancelled` có `event_time` trong khoảng.
   - `expired_trips`: Số lượng sự kiện `TripExpired` có `event_time` trong khoảng.
   - Tổng chuyến đã kết thúc: $\text{total\_ended} = \text{completed\_trips} + \text{cancelled\_trips} + \text{expired\_trips}$.
   - Tổng doanh thu: $\text{total\_revenue} = \sum \text{fare}$ (của các dòng `TripCompleted` trong khoảng).
   - Tổng hoa hồng: $\text{total\_commission} = \sum \text{commission}$ (của các dòng `TripCompleted` trong khoảng).
   - Tỷ lệ hủy chuyến: nếu $\text{total\_ended} > 0$ thì $\text{cancellation\_rate} = \text{round}\left(\frac{\text{cancelled\_trips}}{\text{total\_ended}} \times 100, 2\right)$, ngược lại $0.0$ (kiểu số thực `float`, biểu thị phần trăm làm tròn 2 chữ số thập phân, không chứa ký tự "%").
   - Tỷ lệ hết giờ: nếu $\text{total\_ended} > 0$ thì $\text{expiration\_rate} = \text{round}\left(\frac{\text{expired\_trips}}{\text{total\_ended}} \times 100, 2\right)$, ngược lại $0.0$ (kiểu số thực `float`, biểu thị phần trăm làm tròn 2 chữ số thập phân, không chứa ký tự "%").
   - Giờ cao điểm (`peak_hours`): Nhóm `TripCreated` theo `EXTRACT(HOUR FROM event_time AT TIME ZONE 'Asia/Ho_Chi_Minh')`. Khi nhiều khung giờ cùng có số chuyến cao nhất thì liệt kê tất cả các khung giờ đó (ví dụ: `["08:00-09:00", "17:00-18:00"]`); khi không có bất kỳ sự kiện `TripCreated` nào trong khoảng thời gian thì trả về mảng rỗng `[]`.
   - *Ghi chú thống kê:* Mỗi metric được lọc theo `event_time` của đúng loại event đó, nên `total_trips` và `total_ended` có thể không khớp nhau trong cùng cửa sổ thời gian (do các chuyến tạo ở chu kỳ trước nhưng kết thúc ở chu kỳ hiện tại).

### 4.3. Sinh nhận xét (LLM hoặc Rule-based Fallback)

1. **Bước 1: Kiểm tra API Key:**
   - Nếu `LLM_API_KEY` rỗng (`""`): **KHÔNG** gửi request nào tới Gemini, **KHÔNG** ghi log WARN (đây là cấu hình hợp lệ, không phải lỗi), chuyển thẳng sang **Bước 3**.
   - Nếu `LLM_API_KEY` không rỗng: Chuyển sang **Bước 2**.

2. **Bước 2: Gọi Google Gemini API (generateContent):**
   - **Request:** Gửi request `POST ${LLM_API_URL}/models/${LLM_MODEL}:generateContent` với context timeout = `LLM_TIMEOUT_SECONDS` (mặc định 10 giây).
     - Headers: `x-goog-api-key: <LLM_API_KEY>`, `Content-Type: application/json`.
   - **Body (JSON):**
     ```json
     {
       "systemInstruction": {
         "parts": [{"text": "Bạn là chuyên gia phân tích vận hành ride-hailing. Trả lời bằng tiếng Việt, văn bản thuần, tối đa 3 câu, không dùng markdown."}]
       },
       "contents": [
         {
           "role": "user",
           "parts": [{"text": "<prompt chứa số liệu>"}]
         }
       ],
       "generationConfig": {
         "temperature": 0.4,
         "maxOutputTokens": 1024
       }
     }
     ```
     *(Ghi chú: để 1024 vì một số model tính cả token suy luận vào hạn mức này; độ dài câu trả lời được giới hạn bằng chỉ dẫn hệ thống, không bằng maxOutputTokens).*
   - **Chỉ dẫn hệ thống:** `"Bạn là chuyên gia phân tích vận hành ride-hailing. Trả lời bằng tiếng Việt, văn bản thuần, tối đa 3 câu, không dùng markdown."`
   - **Prompt người dùng:** Chỉ chứa số liệu tổng hợp (`total_trips`, `completed_trips`, `total_revenue`, `cancellation_rate`, `expiration_rate`, `peak_hours`); **KHÔNG** gửi `trip_id`, `customer_id`, `driver_id`.
   - **Response:** Lấy text ở `candidates[0].content.parts[*].text` (nối các part text lại, bỏ khoảng trắng đầu cuối) làm `insights`, gán `insight_source = "LLM"`.
   - **Xử lý lỗi:** Nếu gặp bất kỳ điều kiện lỗi nào (timeout; lỗi mạng; HTTP khác 200 gồm 400/403/404/429; thân phản hồi không parse được; không có `candidates` hoặc text rỗng gồm `promptFeedback.blockReason`) thì log `WARN` (không log header/API key) và chuyển sang **Bước 3**.

3. **Bước 3: Fallback RULE_BASED:**
   - Gán `insight_source = "RULE_BASED"`.
   - Sinh nhận xét theo tập luật:
     - Hủy: Nếu $\text{cancellation\_rate} > \text{RULE\_HIGH\_CANCEL\_RATE\_THRESHOLD}$ (20%): *"Tỷ lệ hủy chuyến đang ở mức cao (X%). Cần kiểm tra lại thời gian tiếp cận của tài xế."* Ngược lại: *"Tỷ lệ hủy chuyến ở mức an toàn (X%)."*
     - Hết giờ: Nếu $\text{expiration\_rate} > \text{RULE\_HIGH\_EXPIRE\_RATE\_THRESHOLD}$ (15%): *"Tỷ lệ chuyến hết giờ cao (Y%). Khu vực đang thiếu tài xế trầm trọng."* Ngược lại: *"Tỷ lệ ghép chuyến thành công tốt."*
     - Khung giờ: *"Khung giờ cao điểm ghi nhận nhiều chuyến nhất là [peak_hours]. Đề xuất tăng hệ số surge để thu hút tài xế bật Online."* (nếu `peak_hours` rỗng thì nhận xét không ghi nhận giờ cao điểm).

**Kết quả trả về:** Sau 3 bước, trả về HTTP 200 OK kèm `metrics`, `insights`, `insight_source`. Tuyệt đối không bao giờ trả lỗi 500 cho Admin khi Google Gemini API gặp sự cố.  
*Ghi chú: Tổng thời gian xử lý tối đa = LLM_TIMEOUT_SECONDS + thời gian truy vấn DB, luôn nhỏ hơn PROXY_TIMEOUT_MS của api-gateway.*

---

## 5. XỬ LÝ NGOẠI LỆ (EXCEPTION HANDLING)

| Tình huống ngoại lệ | Ngữ cảnh phát sinh | Hành vi xử lý & Mã lỗi |
| :--- | :--- | :--- |
| Sai định dạng thời gian | `start_time` hoặc `end_time` không đúng chuẩn ISO 8601 | Trả `VALIDATION_ERROR` (400). |
| Khoảng thời gian không hợp lệ | `start_time > end_time` | Trả `VALIDATION_ERROR` (400). |
| Sai vai trò Quản trị viên | Người dùng không phải ADMIN gọi API báo cáo | Trả `FORBIDDEN` (403). |
| LLM lỗi / Timeout / Hết quota / Chặn an toàn | `LLM_API_KEY` rỗng; timeout; lỗi mạng; HTTP khác 200 (gồm 400/403/404/429); thân phản hồi không parse được; không có `candidates` hoặc text rỗng (bị chặn an toàn, `promptFeedback.blockReason`) | `LLM_API_KEY` rỗng: không gọi, không log WARN; các điều kiện lỗi còn lại: log WARN (không log header/API key). Tự động Fallback sang Heuristic Rule, trả 200 OK kèm `insight_source = "RULE_BASED"`. Tuyệt đối không trả 500. |
| Event Stream lặp lại | Consumer nhận lại event đã có trong DB | Idempotency theo `(trip_id, event_type)` bỏ qua và gửi `XACK`. |
| Message Stream hỏng | Thiếu trường, UUID sai, fare $\le 0$, hoặc entry rỗng | Log ERROR, gửi XACK loại bỏ tin nhắn hỏng. |
| Lỗi DB tạm thời lúc tiêu thụ stream | Lỗi kết nối PostgreSQL khi lưu sự kiện analytics | KHÔNG XACK, giữ trong PEL để xử lý lại sau 2–5s. |
| Lỗi XACK sau commit DB | Redis timeout/mất mạng khi gửi XACK | Log ERROR, không rollback DB. Event trong PEL sẽ được idempotency bỏ qua khi quét lại. |

### 5.1. Hạn chế đã biết
- **Doanh thu và hoa hồng không phản ánh giao dịch FAILED:** Doanh thu và hoa hồng trong `ai-service` được tổng hợp trực tiếp từ giá cước chốt trong sự kiện `TripCompleted` (do dispatch-service phát), nên không phản ánh các trường hợp khách thiếu tiền dẫn đến giao dịch thanh toán bị `FAILED` tại `payment-service`. Đây là trade-off chấp nhận được cho phân tích vận hành tổng thể.
- **Mất sự kiện do crash:** Nếu `dispatch-service` crash giữa lúc `UPDATE` và `XADD` Stream (theo Mục 5.1 của `dispatch-service`), sự kiện chuyến không được phát đi dẫn đến số liệu thống kê có thể thiếu một vài chuyến, cần đối soát DB để bù đắp nếu cần.
- **Biến động model Google Gemini:** Tên model Gemini thay đổi theo thời gian; đổi bằng LLM_MODEL, nếu model không còn thì gọi lỗi 404 và hệ thống tự rơi về RULE_BASED.

---

## 6. SỰ KIỆN & KHÓA REDIS

### 6.1. Dữ liệu Redis quản lý
`ai-service` **không** lưu trữ khóa dữ liệu tạm hay khóa phân tán trong Redis.

### 6.2. Kênh Pub/Sub
`ai-service` **không** phát hay nhận trực tiếp bất kỳ kênh Pub/Sub nào.

### 6.3. Sự kiện Redis Stream lắng nghe (`stream:trip_events`)
- **Stream:** `stream:trip_events`. Consumer Group: `ai-group`. Consumer Name: `ai-consumer-1`.
- **Sự kiện quan tâm:** Phân loại bằng trường `type`. Lắng nghe toàn bộ 4 sự kiện vòng đời:
  1. `TripCreated`: `type="TripCreated"`, `version="1.0"`, `trip_id`, `customer_id`, `pickup_geohash5`, `created_at`.
  2. `TripCompleted`: `type="TripCompleted"`, `version="1.0"`, `trip_id`, `customer_id`, `driver_id`, `fare`, `completed_at` *(tự tính hoa hồng bằng COMMISSION_RATE, không đọc từ event)*.
  3. `TripCancelled`: `type="TripCancelled"`, `version="1.0"`, `trip_id`, `cancelled_at`.
  4. `TripExpired`: `type="TripExpired"`, `version="1.0"`, `trip_id`, `reason`, `expired_at`.

---

## 7. CẤU HÌNH (ENV) VÀ TÀI NGUYÊN

### 7.1. Bảng biến môi trường
| Tên biến ENV | Mặc định | Ý nghĩa & Mục đích sử dụng |
| :--- | :--- | :--- |
| `PORT` | `8006` | Cổng HTTP nội bộ của `ai-service` trong mạng Docker. |
| `DATABASE_URL` | `postgres://ai_user:ai_pass@postgres:5432/aidb?sslmode=disable` | Chuỗi kết nối cơ sở dữ liệu `aidb`. |
| `REDIS_ADDR` | `redis:6379` | Địa chỉ kết nối Redis container trong Docker. |
| `REDIS_PASSWORD` | `redis_secret_pass` | Mật khẩu xác thực kết nối Redis (demo, ghi đè trong .env). |
| `COMMISSION_RATE` | `15` | Phần trăm hoa hồng hệ thống dùng để tính tích lũy `total_commission` (nguyên). |
| `LLM_API_KEY` | `""` | Khóa API dịch vụ trí tuệ nhân tạo (lấy từ Google AI Studio, chỉ đặt trong .env, không commit; nếu rỗng tự động chạy Heuristic Rule). |
| `LLM_API_URL` | `https://generativelanguage.googleapis.com/v1beta` | URL gốc của Google Gemini API. |
| `LLM_MODEL` | `gemini-3.5-flash-lite` | Tên model Gemini (nhà cung cấp là Google Gemini; tên model có thể đổi qua ENV mà không sửa code). |
| `LLM_TIMEOUT_SECONDS` | `10` | Thời gian chờ tối đa khi gọi Google Gemini API trước khi kích hoạt Fallback (giây). |
| `RULE_HIGH_CANCEL_RATE_THRESHOLD` | `20.0` | Ngưỡng tỷ lệ hủy (%) kích hoạt cảnh báo trong Heuristic Rule. |
| `RULE_HIGH_EXPIRE_RATE_THRESHOLD` | `15.0` | Ngưỡng tỷ lệ hết giờ (%) kích hoạt cảnh báo trong Heuristic Rule. |

### 7.2. Tài nguyên & Thứ tự khởi động
- **Connection Pool PostgreSQL (Mục 5(h) LLD-00):** `MaxOpenConns = 5`, `MaxIdleConns = 2`, `ConnMaxLifetime = 30m`.
- **Ràng buộc bộ nhớ VPS 1GB:** `mem_limit` Docker đề xuất: 30 MB; `GOMEMLIMIT = 26MiB` (chống OOM).
- **Thứ tự khởi động dịch vụ (Startup Sequence):**
  1. Kết nối PostgreSQL `aidb` và thực thi migration bảng `trip_events_analytics`.
  2. Kết nối Redis, ping kiểm tra `PONG`.
  3. Khởi tạo consumer group `ai-group` trên `stream:trip_events` (`MKSTREAM`, bỏ qua lỗi `BUSYGROUP`).
  4. Khởi chạy goroutine worker tiêu thụ stream (vòng lặp quét PEL định kỳ mỗi 2–5 giây kết hợp đọc tin mới `>`).
  5. Khởi động HTTP web server (Fiber) lắng nghe cổng `8006` tiếp nhận request.
