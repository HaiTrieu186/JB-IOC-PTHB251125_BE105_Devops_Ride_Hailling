# Quyết định đã chốt

1. Ví khách: kiểm tra số dư lúc đặt xe, không đủ thì chặn (không cho đặt).
2. Ghép xe: broadcast giới hạn cho Top 3-5 tài xế rảnh gần nhất qua Redis Pub/Sub -> WS. Ai nhận trước thì Redis SET NX chốt, người sau nhận lỗi TRIP_ALREADY_TAKEN.
3. Hoa hồng 15% (cấu hình qua env). Không có phí phạt khi hủy chuyến.
4. Khoảng cách/giá: gọi OSRM public (timeout 400ms); lỗi hoặc timeout thì fallback về Haversine x 1.35. Không tự host OSRM/GraphHopper (VPS 1 GB).

Ghi chú: docs/matching_recommend.txt viết cho bài toán carpooling OptiGo, chỉ tham khảo, không áp dụng nguyên.
