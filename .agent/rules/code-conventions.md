# Luật: Quy ước code
- Go + Fiber + GORM.
- Mỗi service: Dockerfile riêng (multi-stage), cấu hình qua biến môi trường,
  endpoint GET /health.
- Response JSON thống nhất: {"success":bool,"data":...,"error":{"code","message"}}.
- Tên service/thư mục: kebab-case. Comment ngắn, tiếng Việt hoặc Anh đều được.
