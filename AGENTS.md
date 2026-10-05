# Dự án: Ride-Hailing & Logistics (đồ án môn DevOps)

## Mục tiêu
Deploy hệ thống microservices lên VPS thật. Backend chỉ cần chạy được luồng
nghiệp vụ chính để demo test API (curl/Postman). KHÔNG làm UI.
Đề gốc: docs/00-brainstorm/de-bai-goc.md

## Kiến trúc (BẮT BUỘC giữ đúng)
Internet → Nginx (container, DUY NHẤT mở cổng 80/443)
  → api-gateway → user / location / dispatch / pricing / payment / ai-service
  → ws-gateway (qua đường /ws/ của Nginx)
Postgres, Redis: chạy container, KHÔNG mở cổng ra ngoài.

## Tech stack
- Go + Fiber + GORM
- Redis (Pub/Sub + Streams) thay Kafka
- PostgreSQL (PostGIS nếu cần), mỗi service 1 schema
- Docker Compose
- CI/CD: GitHub Actions, mỗi service 1 file ci-<tên>.yml, push lên GHCR
- Domain: DuckDNS + certbot

## Ràng buộc VPS
1 GB RAM, 20 GB disk → mỗi container có mem_limit, bật swap 2 GB.
Không đề xuất Kafka, Elasticsearch, Java.

## Quy trình làm việc
1. Doc trước, code sau: SRS → use case → LLD → code.
2. Chỉ làm đúng việc được yêu cầu ở lượt hiện tại.
3. CHƯA được tự ý viết code, docker-compose, CI khi người dùng chưa yêu cầu.
4. Người dùng là sinh viên cần HIỂU luồng nghiệp vụ: giải thích ngắn gọn, tiếng Việt.

## Phạm vi MVP (luồng chính)
- Đăng ký / đăng nhập (JWT)
- Tài xế ONLINE, gửi GPS qua WebSocket
- Khách đặt xe → báo giá → ghép tài xế gần nhất → tài xế nhận → state machine chuyến
- Hoàn thành chuyến → trừ ví khách, cộng ví tài xế
- ai-service: thống kê chuyến, doanh thu, giờ cao điểm, tỷ lệ hủy
Các phần phụ: làm cực đơn giản hoặc bỏ.

## Luật bắt buộc

### Luật: Quy ước code
- Go + Fiber + GORM.
- Mỗi service: Dockerfile riêng (multi-stage), cấu hình qua biến môi trường,
  endpoint GET /health.
- Response JSON thống nhất: {"success":bool,"data":...,"error":{"code","message"}}.
- Tên service/thư mục: kebab-case. Comment ngắn, tiếng Việt hoặc Anh đều được.

### Luật: Doc trước, code sau
- Không viết code khi chưa có SRS/LLD của service đó được người dùng duyệt.
- Thiếu doc thì dừng lại và hỏi.
- Sau khi viết xong một tài liệu, dừng và chờ người dùng duyệt.

### Luật: Ràng buộc VPS
- VPS 1 GB RAM, 20 GB disk, Ubuntu.
- Không đề xuất Kafka, Elasticsearch/EFK, Java. Ưu tiên giải pháp nhẹ.
- Mọi container phải có mem_limit.
- Chỉ Nginx được publish cổng ra ngoài; còn lại dùng mạng nội bộ Docker.
- Monitoring (Prometheus, node_exporter) tách profile riêng để tắt được.

## Lệnh có sẵn (trong .agents/skills/)
- /write-spec  : viết tài liệu theo phần được yêu cầu
- /implement-service : code 1 service từ LLD đã duyệt

## Skill nên dùng theo giai đoạn (.agents/skills/)
- Viết tài liệu: brainstorming, spec-writer, architecture, microservices-patterns, database-design, api-design-principles, openapi-spec-generation, diagram-generator
- Viết code Go: golang-pro, go-concurrency-patterns, auth-implementation-patterns, postgresql, redis
- Test API: postman-collection-generator, api-documentation
- Deploy/CI: docker-expert, docker-compose, github-actions-templates, github-actions-debugger, deployment-procedures, linux-administration, prometheus-configuration
- Kiểm tra/sửa lỗi: verification-before-completion, systematic-debugging
Chỉ đọc skill khi giai đoạn hiện tại cần, không nạp hết cùng lúc.
