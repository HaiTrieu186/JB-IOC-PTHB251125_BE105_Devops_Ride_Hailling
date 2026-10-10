# Dự án: Ride-Hailing & Logistics (đồ án môn DevOps)

## Mục tiêu
Deploy hệ thống microservices lên VPS thật. Backend chỉ cần chạy được luồng
nghiệp vụ chính để demo test API (curl/Postman). KHÔNG làm UI.
Đề gốc: docs/00-brainstorm/de-bai-goc.md

## Kiến trúc (BẮT BUỘC giữ đúng)
Internet → Nginx (cài trực tiếp trên VPS, DUY NHẤT nhận kết nối từ Internet, cổng 80/443)
→ /    → api-gateway (127.0.0.1:8080) → user / location / dispatch / pricing / payment / ai-service
→ /ws/ → ws-gateway  (127.0.0.1:8081, có Upgrade header cho WebSocket)
Postgres, Redis, các service còn lại: chỉ trong mạng Docker, KHÔNG publish cổng.
Hai gateway publish dạng 127.0.0.1:PORT (không dùng 0.0.0.0, vì Docker có thể bỏ qua UFW).
- Nginx ĐÃ cài sẵn trên VPS cùng certbot + UFW (OpenSSH, Nginx Full) và domain DuckDNS đã có chứng chỉ.
  KHÔNG đưa Nginx vào docker-compose. Sửa file cấu hình có sẵn (thêm location /ws/), không tạo lại từ đầu.
- api-gateway: kiểm tra JWT, gắn header X-User-Id / X-User-Role rồi chuyển request.
  Các service nội bộ tin header này (mặc định, có thể đổi trong SRS).
- ws-gateway: CHỈ để nhận GPS tài xế, đẩy thông báo xuống client, và chuyển tiếp tọa độ tài xế cho khách của chuyến (FR-38). Hành động nghiệp vụ
  (đặt xe, accept, complete...) đi qua REST qua api-gateway để dễ test bằng curl/Postman.
- Giao tiếp đồng bộ: REST (client Go tự viết, gọi bằng tên service trong Docker). Bất đồng bộ: Redis Streams
  (sự kiện giữa service, consumer group + ACK) và Redis Pub/Sub (đẩy tin realtime tới ws-gateway).

## Tech stack
- Go + Fiber + GORM
- Redis: Pub/Sub (đẩy tin realtime tới ws-gateway, 1 instance) + Streams (sự kiện giữa service). Thay Kafka.
- PostgreSQL: DATABASE-PER-SERVICE. Dùng 1 container Postgres (tiết kiệm RAM), bên trong mỗi service
  có 1 database + 1 user/mật khẩu riêng. Service KHÔNG được truy cập database của service khác;
  cần dữ liệu thì gọi API hoặc nhận sự kiện Redis. KHÔNG dùng chung schema/bảng giữa các service.
- Docker Compose (1 file, tách profile cho monitoring)
- CI/CD: GitHub Actions, mỗi service 1 file ci-<tên>.yml, push lên GHCR
- Domain: DuckDNS + certbot

## Dữ liệu theo service
| Service | Lưu trữ |
|---|---|
| api-gateway, ws-gateway | Không có DB (Redis nếu cần) |
| user-service | Postgres: userdb (tài khoản, hồ sơ tài xế, trạng thái ONLINE/OFFLINE/BUSY) |
| location-service | Redis GEO (vị trí mới nhất); không có Postgres (SRS FR-10 đã bỏ qua, không tạo locationdb) |
| dispatch-service | Postgres: dispatchdb (chuyến đi, lịch sử trạng thái) |
| pricing-service | Postgres: pricingdb (bảng giá) + cache Redis |
| payment-service | Postgres: paymentdb (ví, giao dịch) |
| ai-service | Postgres: aidb (số liệu tổng hợp, tự thu qua sự kiện Redis hoặc API) |

## Luồng nghiệp vụ cốt lõi (dùng làm khung cho SRS/LLD)
1. Đăng ký / đăng nhập → JWT.
2. Tài xế bật ONLINE, gửi GPS mỗi 3-5 giây qua WebSocket → location-service cập nhật Redis GEO.
3. Khách đặt xe → pricing báo giá (khoảng cách × đơn giá × hệ số surge đơn giản)
   → dispatch tìm tài xế rảnh gần nhất (bán kính X km) → gửi đề nghị cho tài xế.
4. Chống 2 khách giành 1 tài xế: khóa nguyên tử bằng Redis (SET NX có TTL).
5. State machine chuyến: CREATED → MATCHING → ACCEPTED → PICKING_UP → IN_TRIP → COMPLETED
   (nhánh CANCELLED, và MATCHING hết giờ hoặc không có tài xế thì chuyển EXPIRED). Chỉ cho phép chuyển trạng thái hợp lệ.
6. COMPLETED → dispatch phát sự kiện TripCompleted (Redis Stream) →
   payment trừ ví khách + cộng ví tài xế (trừ hoa hồng); ai-service ghi số liệu.
7. ai-service: thống kê số chuyến, doanh thu, giờ cao điểm, tỷ lệ hủy; sinh nhận xét bằng LLM.
   Không có API key thì vẫn chạy bằng luật đơn giản.
   Surge: Demand = số yêu cầu đặt xe, Supply = số tài xế rảnh, tính theo khu vực/ô lưới (làm cực đơn giản).
   Phần phụ (hồ sơ chi tiết, khuyến mãi, thẻ ngân hàng thật): bỏ hoặc làm tối giản.

## Tiêu chí demo (từ đề của thầy)
- Vị trí tài xế cập nhật qua WebSocket với độ trễ < 500 ms.
- Ghép đúng tài xế rảnh và gần nhất.
- Chạy trên VPS thật, kiểm tra được HTTPS/WSS từ mạng ngoài.
- Có script giả lập 100 tài xế ảo gửi GPS; đặt xe thật và tài xế ảo nhận được chuyến.

## Ràng buộc VPS
1 GB RAM, 20 GB disk, Ubuntu → mỗi container có mem_limit, bật swap 2 GB, dọn image cũ định kỳ.
- Không dùng Kafka, Elasticsearch/EFK, Java. EFK bị loại vì Elasticsearch cần ≥ 2 GB RAM;
  thay bằng log Docker + Dozzle (xem log trực tiếp). Nếu nâng VPS lên ≥ 4 GB thì có thể thêm EFK.
- Monitoring: Prometheus + node_exporter + cAdvisor, tách profile riêng để tắt khi không demo.
- Dozzle/Prometheus chỉ bind 127.0.0.1, truy cập qua SSH tunnel hoặc Nginx có basic auth.

## Quy trình làm việc
1. Doc trước, code sau: SRS → use case → LLD → code.
2. Chỉ làm đúng việc được yêu cầu ở lượt hiện tại.
3. CHƯA được tự ý viết code, docker-compose, CI khi người dùng chưa yêu cầu.
4. Không tự thêm công nghệ ngoài danh sách ở "Tech stack"; muốn thêm thì hỏi trước.
5. Người dùng là sinh viên cần HIỂU luồng nghiệp vụ: giải thích ngắn gọn, tiếng Việt.

## Quy ước tài liệu
- Viết tiếng Việt, ngắn gọn, dễ hiểu. Sơ đồ dùng Mermaid.
- File đặt trong docs/ đúng thư mục: 01-srs, 02-use-cases, 03-lld, 04-api-test, 05-deploy.
- Quy tắc ưu tiên khi các tài liệu lệch nhau: LLD-00 > LLD service > Use Case > SRS (SRS là yêu cầu, LLD là chuẩn cài đặt).
- SRS: mỗi chức năng có mã (FR-01, FR-02...) và mức ưu tiên (bắt buộc / nên có / bỏ qua).
- LLD mỗi service: 1 file docs/03-lld/<tên-service>.md, gồm bảng DB của CHÍNH service đó,
  API, luồng xử lý, ngoại lệ, sự kiện Redis phát/nhận.
- Test API: curl chạy được, sắp theo thứ tự kịch bản demo end-to-end.
- Xong mỗi tài liệu: tóm tắt ngắn rồi DỪNG, chờ duyệt.

## Quy ước code (khi tới giai đoạn code)
- Go + Fiber + GORM.
- Mỗi service: Dockerfile riêng (multi-stage), cấu hình qua biến môi trường, endpoint GET /health.
- Response JSON thống nhất: {"success":bool,"data":...,"error":{"code","message"}}.
- Tên service/thư mục: kebab-case. Không commit file .env hay mật khẩu thật.

## Lệnh có sẵn (trong .agents/skills/)
- /write-spec : viết tài liệu theo phần được yêu cầu
- /implement-service : code 1 service từ LLD đã duyệt

## Skill nên dùng theo giai đoạn (.agents/skills/)
- Viết tài liệu: brainstorming, spec-writer, architecture, microservices-patterns, database-design, api-design-principles, openapi-spec-generation, diagram-generator
- Viết code Go: golang-pro, go-concurrency-patterns, auth-implementation-patterns, postgresql, redis
- Test API: postman-collection-generator, api-documentation
- Deploy/CI: docker-expert, docker-compose, github-actions-templates, github-actions-debugger, deployment-procedures, linux-administration, prometheus-configuration
- Kiểm tra/sửa lỗi: verification-before-completion, systematic-debugging
  Chỉ đọc skill khi giai đoạn hiện tại cần, không nạp hết cùng lúc.
## Thứ tự triển khai code
> **Lưu ý:** Đây là thứ tự ưu tiên khi bắt tay vào code; tài liệu kỹ thuật (SRS, Use Case, LLD) vẫn được viết đầy đủ toàn bộ hệ thống từ đầu.

- **Tầng 1 (Đủ điều kiện demo cốt lõi):**
  - Toàn bộ FR mức "Bắt buộc" trừ `ai-service`, cộng thêm `FR-38` (stream vị trí tài xế cho khách), `FR-36` (xem chuyến hiện tại và chi tiết chuyến), `FR-30` (khởi tạo Admin từ ENV), `FR-31` (GET/PUT `/api/v1/admin/pricing/config`), `FR-37` (Admin hủy cưỡng bức), và riêng `GET /api/v1/admin/trips` (một phần của `FR-32`).
  - *Lý do đưa lên Tầng 1:* `FR-37` là công cụ duy nhất gỡ chuyến kẹt; `GET /admin/trips` để tìm `trip_id` khi cần gỡ; `FR-31` cần để hạ $T$ khi demo surge; `FR-30` cần để có tài khoản Admin gọi các API trên.
  - Đường luồng demo chính: Đăng ký → Đăng nhập → Nạp tiền ví ảo → Tài xế bật ONLINE gửi GPS → Khách tra ước tính cước → Đặt xe → Tài xế nhận chuyến → Hoàn thành cuốc → Kiểm tra số dư ví.
  - ENV chỉ là giá trị seed lần đầu khi bảng `pricing_configs` rỗng; sau đó đọc từ DB (cache Redis 1 giờ), muốn đổi thì dùng PUT config (`FR-31`).
- **Tầng 2 (Sau khi Deploy VPS và CI/CD hoạt động ổn định):**
  - Triển khai `ai-service`: `FR-27` (thu thập số liệu vận hành từ Redis Streams) và `FR-28` (báo cáo và nhận xét AI / Heuristic Rule).
- **Tầng 3 (Nếu còn thời gian tối ưu):**
  - Các tính năng mở rộng: `FR-05` (hồ sơ cá nhân), `FR-06` (phương tiện tài xế), phần còn lại của `FR-32` (xem người dùng, xem tài xế active).
  - Các tính năng làm kèm cùng lúc vì chi phí triển khai thấp: `FR-34` (đăng xuất & blacklist Redis), `FR-35` (thông báo đổi trạng thái chuyến qua WS).

## Thư viện & công nghệ được phép (chốt 10/10/2026)
- Go: tối thiểu 1.22; go.mod dùng phiên bản mà thư viện yêu cầu; image build golang:<phiên bản đó>-alpine, CGO_ENABLED=0; image cuối alpine:3 + ca-certificates + tzdata (không scratch vì OSRM/Gemini gọi HTTPS và ai-service cần múi giờ Asia/Ho_Chi_Minh); healthcheck dùng wget.
- Thư viện Go: github.com/gofiber/fiber/v2 (giữ v2); gorm.io/gorm + gorm.io/driver/postgres; github.com/redis/go-redis/v9; github.com/golang-jwt/jwt/v5 (bắt buộc jwt.WithValidMethods HS256); golang.org/x/crypto/bcrypt; github.com/google/uuid; github.com/gofiber/contrib/websocket (chỉ ws-gateway); github.com/mmcloughlin/geohash (chỉ location, pricing, dispatch).
- Dùng stdlib, KHÔNG thêm thư viện: log/slog (JSON handler), os.Getenv cho cấu hình, tự viết validate, net/http cho REST nội bộ, OSRM, Gemini. Không viper/zap/zerolog/validator. Không viết unit test (CI chỉ go vet + go build).
- CI/CD: actions/checkout, actions/setup-go, docker/login-action, build bằng docker build + docker push thường (không buildx, không cache); deploy bằng appleboy/scp-action và appleboy/ssh-action. Tên owner GHCR luôn chữ thường.
- Script demo: Python 3 + aiohttp. Test WS: image websocat chạy tạm. Test REST nội bộ: curlimages/curl trên mạng backend-net.
- Compose: container_name = tên service; restart: unless-stopped; logging json-file max-size 5m, max-file 2; mỗi service có cả build và image ghcr.io/${GHCR_OWNER}/<tên>:latest; mạng default đặt tên backend-net.
- Chưa làm (để sau cùng nếu còn thời gian, profile riêng): Dozzle, Prometheus + node_exporter + cAdvisor.
- Muốn dùng thư viện/công nghệ ngoài danh sách này: HỎI người dùng trước.