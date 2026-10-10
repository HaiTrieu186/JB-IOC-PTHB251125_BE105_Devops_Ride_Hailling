# KẾ HOẠCH TRIỂN KHAI & VẬN HÀNH (DEPLOYMENT PLAN)

> **Mã tài liệu:** `DEP-00` | **Phiên bản:** `1.0` | **Trạng thái:** Bản khung chuẩn bị triển khai VPS

---

## 1. Danh sách file cần có
- `infra/postgres/init.sql`: Tạo 5 DB (`userdb`, `dispatchdb`, `pricingdb`, `paymentdb`, `aidb`) và 5 user/password riêng biệt.
- `infra/redis/redis.conf`: Cấu hình `maxmemory 50mb`, `maxmemory-policy noeviction`, `appendonly yes`, `appendfsync everysec`.
- `docker-compose.yml`: Khai báo các service, healthcheck, depends_on với condition `service_healthy` cho Postgres/Redis, giới hạn `mem_limit`, và biến môi trường fail-fast `${VAR:?}` cho `JWT_SECRET`, `REDIS_PASSWORD`, `ADMIN_PASSWORD`.

---

## 2. Cấu hình Nginx Reverse Proxy cho WebSocket (`/ws/`)
Sửa file cấu hình Nginx hiện có của domain, thêm location `/ws/` proxy WSS về `ws-gateway` (`127.0.0.1:8081`). Lưu ý: client phải gọi đúng `/ws/` có dấu `/` ở cuối.
```nginx
location /ws/ {
    proxy_pass http://127.0.0.1:8081;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_read_timeout 120s;  # >= 120s: Chống đứt kết nối WebSocket khi tài xế đứng yên
    proxy_send_timeout 120s;
}
```

---

## 3. Runbook gỡ tài xế kẹt trạng thái BUSY
- **Gỡ chuyến kẹt của hệ thống:** Tìm chuyến kẹt qua `GET /api/v1/admin/trips?status=...` rồi gọi `POST /api/v1/admin/trips/:id/force-cancel`.
- **Cưỡng bức giải phóng tài xế (chỉ chạy khi chắc chắn tài xế không còn chuyến active):**
  ```bash
  docker exec -i postgres psql -U user_user -d userdb \
    -c "UPDATE users SET driver_status = 'ONLINE', updated_at = now() WHERE id = '<driver_id>' AND driver_status = 'BUSY';"
  ```

---

## 4. Ghi chú Demo trên VPS 1GB RAM
- **OSRM Public API:** Gọi từ VPS Việt Nam ra quốc tế thường bị vượt ngưỡng timeout 400 ms và tự fallback sang HAVERSINE; nếu muốn demo nhánh OSRM thì tạm nâng `OSRM_TIMEOUT_MS`.
- **Script 100 tài xế ảo:** Khởi chạy đăng nhập so le (do tính toán bcrypt cost 10 tốn CPU trên 1 vCPU) và tự động kết nối lại WebSocket với exponential backoff kèm jitter.
