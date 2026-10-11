# Quy tắc cập nhật .env trước khi deploy

1. **Cập nhật biến môi trường trên VPS trước khi deploy:**
   Mỗi lần file `.env.example` trong repository có biến mới (ví dụ: `LLM_API_KEY`, `JWT_SECRET`, v.v.), người vận hành bắt buộc phải cập nhật file `.env` trên VPS **TRƯỚC** khi kích hoạt tiến trình deploy CI/CD.
   - **Lý do:** Các biến môi trường trong `docker-compose.yml` được cấu hình fail-fast theo cú pháp `${VAR:?}`. Nếu thiếu bất kỳ biến nào, lệnh Docker Compose sẽ dừng ngay lập tức và báo lỗi.
   - **Phạm vi ảnh hưởng:** Docker Compose phân giải (resolve) biến môi trường cho **CẢ FILE** `docker-compose.yml`. Do đó, chỉ cần thiếu 1 biến của bất kỳ service nào cũng sẽ làm hỏng tiến trình deploy của **TẤT CẢ** các service khác, không chỉ riêng service mới.

2. **Ghi chú về `ws-gateway` và kết nối WebSocket:**
   Dịch vụ `ws-gateway` chỉ chạy duy nhất 1 instance (do tài nguyên VPS 1 GB RAM). Do đó, khi rolling update hoặc recreate container `ws-gateway`, kết nối WebSocket sẽ bị gián đoạn tạm thời (đã biết trước trong thiết kế; script tài xế ảo sẽ tự động kết nối lại).
