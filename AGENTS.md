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
- ws-gateway: CHỈ để nhận GPS tài xế và đẩy thông báo xuống client. Hành động nghiệp vụ
  (đặt xe, accept, complete...) đi qua REST qua api-gateway để dễ test bằng curl/Postman.
- Giao tiếp đồng bộ: REST (client Go tự viết, gọi bằng tên service trong Docker). Bất đồng bộ: Redis Streams
  (sự kiện giữa service, consumer group + ACK) và Redis Pub/Sub (đẩy tin realtime tới ws-gateway).

## Tech stack
- Go + Fiber + GORM
- Redis: Pub/Sub (đẩy tin realtime giữa các instance ws-gateway) + Streams (sự kiện giữa service). Thay Kafka.
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
| location-service | Redis GEO (vị trí mới nhất); Postgres: locationdb chỉ khi cần lưu lịch sử |
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
   (nhánh CANCELLED, và MATCHING hết giờ thì thất bại). Chỉ cho phép chuyển trạng thái hợp lệ.
6. COMPLETED → dispatch phát sự kiện TripCompleted (Redis Stream) →
   payment trừ ví khách + cộng ví tài xế (trừ hoa hồng); user cập nhật lịch sử; ai-service ghi số liệu.
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
## Việc chưa chốt (hỏi người dùng khi viết SRS, đừng tự quyết)
- Ví khách không đủ tiền thì xử lý thế nào (chặn lúc đặt xe, hay cho âm).
- Ghép chuyến: gửi đề nghị lần lượt từng tài xế gần nhất (đề xuất: 15 giây mỗi người, tối đa 3 lần) hay gửi nhiều người cùng lúc.
- Tỷ lệ hoa hồng (đề xuất 15%, đặt trong cấu hình) và có phạt khi hủy hay không (đề xuất: bỏ).