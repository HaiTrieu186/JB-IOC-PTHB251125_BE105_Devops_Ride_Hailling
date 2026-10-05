---
name: spec-writer
description: Viết tài liệu SRS, use case, LLD, file test API theo mẫu chuẩn cho hệ thống ride-hailing. Dùng khi người dùng yêu cầu viết spec/tài liệu.
---

# Khi nào dùng
Khi được yêu cầu viết SRS, use case, LLD hoặc kịch bản test API.

# Mẫu SRS (toàn hệ thống)
Mục tiêu | Actor | Danh sách chức năng (ID, mô tả, ưu tiên) | Yêu cầu phi chức năng | Ngoài phạm vi

# Mẫu use case
Tên | Actor | Tiền điều kiện | Luồng chính | Luồng ngoại lệ | Hậu điều kiện

# Mẫu LLD cho mỗi service
1. Trách nhiệm
2. Bảng DB (cột, kiểu, khóa)
3. API (method, path, request, response, mã lỗi)
4. Luồng xử lý từng bước
5. Ngoại lệ
6. Sự kiện Redis phát/nhận

# Mẫu test API
Mỗi API: ví dụ curl, request, response mong đợi, trường hợp lỗi.
Kèm kịch bản demo end-to-end theo thứ tự gọi.
