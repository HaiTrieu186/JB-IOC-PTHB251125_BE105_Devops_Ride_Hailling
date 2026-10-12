# KỊCH BẢN KIỂM THỬ DỊCH VỤ ĐIỀU PHỐI (DISPATCH-SERVICE TEST SUITE)

> **Mã tài liệu:** `TEST-DISPATCH` | **Phiên bản:** `1.1`  
> **Dịch vụ kiểm thử:** `dispatch-service` (Port nội bộ: `8003`, Dev host: `127.0.0.1:8003`)  
> **Tài liệu tham chiếu:** [00-du-lieu-mau.md](00-du-lieu-mau.md), [LLD dispatch-service](../03-lld/dispatch-service.md), [LLD-00](../03-lld/00-hop-dong-lien-service.md)

---

## 1. TỔNG QUAN

Bộ kịch bản kiểm thử bao gồm **33 ca kiểm thử HTTP**:
- **16 ca chạy tự động qua Postman** (kèm 4 request thiết lập chuẩn bị ở đầu folder `05 dispatch-service`): `DP-00`, `DP-01a`..`DP-01f`, `DP-03`, `DP-05`, `DP-08a`, `DP-08c`, `DP-10`, `DP-11`, `DP-12a`, `DP-14a`, `DP-15`.
- **17 ca chạy tay qua script PowerShell**: `DP-02`, `DP-04`, `DP-06`, `DP-07a`..`DP-07e`, `DP-08b`, `DP-09`, `DP-12b`, `DP-13`, `DP-14b`, `DP-16`..`DP-19`.

---

## 2. HÀM POWERSHELL DÙNG CHUNG

Định nghĩa MỘT hàm duy nhất để gọi API `dispatch-service` tại `127.0.0.1:8003`:

```powershell
# Hàm PowerShell dùng chung cho các ca kiểm thử thủ công
function Call-Dispatch {
    param(
        [Parameter(Mandatory=$true)][string]$Role,
        [Parameter(Mandatory=$true)][string]$UserId,
        [Parameter(Mandatory=$true)][string]$Method,
        [Parameter(Mandatory=$true)][string]$Path,
        [Parameter(ValueFromPipeline=$true)][string]$Body = ""
    )
    $headers = @("-H", "X-User-Role: $Role", "-H", "X-User-Id: $UserId")
    if ($Body -ne "") {
        $Body | curl.exe -s -X $Method $headers -H "Content-Type: application/json" --data-binary "@-" "http://127.0.0.1:8003$Path"
    } else {
        curl.exe -s -X $Method $headers "http://127.0.0.1:8003$Path"
    }
}

# Biến môi trường mẫu
$c1_id = "d6ace9fc-5290-46a5-826d-d69f38c76c73"
$c3_id = "f8a071f7-bc32-4467-83eb-8e50b73c23e8"
$d1_id = "b6b9c2e8-ba47-4c06-a273-96ee9deec91e"
$d2_id = "58826d56-6512-4211-9a73-91185038c1f3"
$d5_id = "49015ba6-dfec-4ce3-8998-cb5800ca89f0"
$admin_id = "11111111-1111-1111-1111-111111111111"
$bodyP1P2 = '{"pickup_lat":10.7725,"pickup_lng":106.6980,"pickup_address":"Cho Ben Thanh, Q1","dropoff_lat":10.7951,"dropoff_lng":106.7218,"dropoff_address":"Landmark 81, Binh Thanh"}'
$REDIS_PASSWORD = "redis_secret_pass"
```

---

## 3. CHUẨN BỊ VÀ DỌN DẸP

1. **Nạp ví C1 đủ tiền cước** (cước P1 $\rightarrow$ P2 $\approx$ 69.000đ, khi surge có thể $\approx$ 100.000đ; W-01 chỉ nạp 50.000đ nên phải nạp thêm):
   ```powershell
   '{"amount":100000}' | curl.exe -s -X POST -H "Content-Type: application/json" -H "X-User-Id: $c1_id" -H "X-User-Role: CUSTOMER" --data-binary "@-" http://127.0.0.1:8005/api/v1/wallets/top-up
   ```

2. **Bật tài xế D1 và D2 ONLINE**:
   ```powershell
   '{"status":"ONLINE"}' | curl.exe -s -X PATCH -H "Content-Type: application/json" -H "X-User-Id: $d1_id" -H "X-User-Role: DRIVER" --data-binary "@-" http://127.0.0.1:8001/api/v1/driver/status
   '{"status":"ONLINE"}' | curl.exe -s -X PATCH -H "Content-Type: application/json" -H "X-User-Id: $d2_id" -H "X-User-Role: DRIVER" --data-binary "@-" http://127.0.0.1:8001/api/v1/driver/status
   ```

3. **Vòng lặp PowerShell chạy nền giữ GPS D1 và D2 (gửi mỗi 3 giây)**:
   > *Lưu ý:* GPS trong Redis có TTL 15 giây. Request đơn trong Postman chỉ đủ cho ca `DP-01a`. Các ca kiểm thử chờ đợi (nhất là `DP-06` chờ 30 giây) cần bật vòng lặp nền này.
   ```powershell
   $gpsJob = Start-Job -ScriptBlock {
       param($d1, $d2)
       while ($true) {
           '{"driver_id":"' + $d1 + '","latitude":10.7738,"longitude":106.6992}' | curl.exe -s -X POST -H "Content-Type: application/json" --data-binary "@-" http://127.0.0.1:8002/internal/v1/locations/record | Out-Null
           '{"driver_id":"' + $d2 + '","latitude":10.7775,"longitude":106.7032}' | curl.exe -s -X POST -H "Content-Type: application/json" --data-binary "@-" http://127.0.0.1:8002/internal/v1/locations/record | Out-Null
           Start-Sleep -Seconds 3
       }
   } -ArgumentList $d1_id, $d2_id
   ```

4. **Cách dừng vòng lặp GPS nền**:
   ```powershell
   Stop-Job $gpsJob; Remove-Job $gpsJob
   ```

5. **Cách đưa D1/D2 về trạng thái sạch**:
   ```powershell
   '{"status":"ONLINE"}' | curl.exe -s -X PATCH -H "Content-Type: application/json" -H "X-User-Id: $d1_id" -H "X-User-Role: DRIVER" --data-binary "@-" http://127.0.0.1:8001/api/v1/driver/status
   '{"status":"ONLINE"}' | curl.exe -s -X PATCH -H "Content-Type: application/json" -H "X-User-Id: $d2_id" -H "X-User-Role: DRIVER" --data-binary "@-" http://127.0.0.1:8001/api/v1/driver/status
   docker exec redis redis-cli -a "$REDIS_PASSWORD" --no-auth-warning DEL "trip:link:$d1_id" "trip:link:$d2_id"
   ```

---

## 4. BẢNG KỊCH BẢN KIỂM THỬ CHI TIẾT (33 CA)

| Mã KT | Tên kịch bản | Thực thi | Method & Path | Headers | Body / Query | Kỳ vọng HTTP Status & Mã lỗi |
|:---:|---|:---:|---|---|---|---|
| **DP-00** | Healthcheck | Postman | `GET /health` | *(None)* | *(None)* | **200 OK** (`status = "ok"`) |
| **DP-01a**| C1 tạo chuyến P1 $\rightarrow$ P2 | Postman | `POST /api/v1/trips` | `C1 / CUSTOMER` | Tọa độ P1 $\rightarrow$ P2 | **201 Created** (`MATCHING`) |
| **DP-01b**| D1 nhận cuốc (Accept) | Postman | `POST /api/v1/trips/:id/accept` | `D1 / DRIVER` | *(None)* | **200 OK** (`ACCEPTED`) |
| **DP-01c**| D1 đến đón (Picking Up) | Postman | `POST /api/v1/trips/:id/picking-up` | `D1 / DRIVER` | *(None)* | **200 OK** (`PICKING_UP`) |
| **DP-01d**| D1 bắt đầu chở (Start Trip) | Postman | `POST /api/v1/trips/:id/start-trip` | `D1 / DRIVER` | *(None)* | **200 OK** (`IN_TRIP`) |
| **DP-01e**| D1 hoàn thành cuốc (Complete) | Postman | `POST /api/v1/trips/:id/complete` | `D1 / DRIVER` | *(None)* | **200 OK** (`COMPLETED`) |
| **DP-01f**| C1 xem chi tiết và timeline | Postman | `GET /api/v1/trips/:id` | `C1 / CUSTOMER` | *(None)* | **200 OK** (timeline đủ 6 bước) |
| **DP-02** | D1 & D2 tranh cuốc đồng thời | Chạy tay | `POST /api/v1/trips/:id/accept` | `D1 & D2 / DRIVER` | *(None)* | 1 ca **200 OK**, 1 ca **409 Conflict** (`TRIP_ALREADY_TAKEN`) |
| **DP-03** | C3 ví 0đ đặt xe | Postman | `POST /api/v1/trips` | `C3 / CUSTOMER` | Tọa độ P1 $\rightarrow$ P2 | **400 Bad Request** (`INSUFFICIENT_BALANCE`) |
| **DP-04** | C1 đặt lần 2 khi đang có cuốc | Chạy tay | `POST /api/v1/trips` | `C1 / CUSTOMER` | Tọa độ P1 $\rightarrow$ P2 | **409 Conflict** (`ACTIVE_TRIP_EXISTS`) |
| **DP-05** | Đặt xe ở nơi không có tài xế | Postman | `POST /api/v1/trips` | `C1 / CUSTOMER` | Tọa độ Hồ Gươm (Hà Nội) | **201 Created** (`status = "EXPIRED"`) |
| **DP-06** | MATCHING hết hạn sau 30 giây | Chạy tay | `GET /api/v1/trips/:id` | `C1 / CUSTOMER` | *(None)* | **200 OK** (`EXPIRED`, note `TIMEOUT_30S`) |
| **DP-07a**| C1 hủy lúc MATCHING | Chạy tay | `POST /api/v1/trips/:id/cancel` | `C1 / CUSTOMER` | `{"reason": "Doi y"}` | **200 OK** (`CANCELLED`) |
| **DP-07b**| C1 hủy lúc ACCEPTED | Chạy tay | `POST /api/v1/trips/:id/cancel` | `C1 / CUSTOMER` | *(None)* | **200 OK** (`CANCELLED`, D1 về ONLINE) |
| **DP-07c**| D1 hủy lúc ACCEPTED | Chạy tay | `POST /api/v1/trips/:id/cancel` | `D1 / DRIVER` | `{"reason": "Hong xe"}` | **200 OK** (`CANCELLED`, D1 về ONLINE) |
| **DP-07d**| D1 hủy lúc PICKING_UP | Chạy tay | `POST /api/v1/trips/:id/cancel` | `D1 / DRIVER` | *(None)* | **200 OK** (`CANCELLED`, D1 về ONLINE) |
| **DP-07e**| Cố hủy khi đang IN_TRIP | Chạy tay | `POST /api/v1/trips/:id/cancel` | `C1/D1` | *(None)* | **400 Bad Request** (`INVALID_TRIP_STATUS`) |
| **DP-08a**| Admin Force-cancel lý do rỗng | Postman | `POST /admin/trips/:id/force-cancel` | `Admin / ADMIN` | `{"reason": "   "}` | **400 Bad Request** (`VALIDATION_ERROR`) |
| **DP-08b**| Admin Force-cancel thành công | Chạy tay | `POST /admin/trips/:id/force-cancel` | `Admin / ADMIN` | `{"reason": "Su co"}` | **200 OK** (`CANCELLED`, `ADMIN`) |
| **DP-08c**| Admin xem danh sách phân trang | Postman | `GET /admin/trips?page=1&limit=10` | `Admin / ADMIN` | *(None)* | **200 OK** (`trips`, `total > 0`) |
| **DP-09** | Cùng tài xế accept lần 2 | Chạy tay | `POST /api/v1/trips/:id/accept` | `D1 / DRIVER` | *(None)* | **200 OK** (`ACCEPTED`, idempotent) |
| **DP-10** | D5 ngoài bán kính accept | Postman | `POST /api/v1/trips/:id/accept` | `D5 / DRIVER` | *(None)* | **403 Forbidden** (`NOT_OFFERED`) |
| **DP-11** | Accept trip_id không tồn tại | Postman | `POST /api/v1/trips/:id/accept` | `D1 / DRIVER` | *(None)* | **404 Not Found** (`TRIP_NOT_FOUND`) |
| **DP-12a**| Driver gọi POST /trips | Postman | `POST /api/v1/trips` | `D1 / DRIVER` | Tọa độ P1 $\rightarrow$ P2 | **403 Forbidden** (`FORBIDDEN`) |
| **DP-12b**| Customer gọi POST /accept | Chạy tay | `POST /api/v1/trips/:id/accept` | `C1 / CUSTOMER` | *(None)* | **403 Forbidden** (`FORBIDDEN`) |
| **DP-13** | Khách hủy MATCHING rồi tài xế accept | Chạy tay | `POST /api/v1/trips/:id/accept` | `D1 / DRIVER` | *(None)* | **400 Bad Request** (`INVALID_TRIP_STATUS`) |
| **DP-14a**| Admin xem GET /trips/:id | Postman | `GET /api/v1/trips/:id` | `Admin / ADMIN` | *(None)* | **403 Forbidden** (`FORBIDDEN`) |
| **DP-14b**| Admin truyền page không hợp lệ | Chạy tay | `GET /admin/trips?page=abc` | `Admin / ADMIN` | *(None)* | **400 Bad Request** (`VALIDATION_ERROR`) |
| **DP-15** | Xem cuốc hiện tại khi rảnh | Postman | `GET /api/v1/trips/current` | `C1 / CUSTOMER` | *(None)* | **200 OK** (`data = null`) |
| **DP-16** | D1 nhận lời mời rồi OFFLINE, accept | Chạy tay | `POST /api/v1/trips/:id/accept` | `D1 / DRIVER` | *(None)* | **400 Bad Request** (`DRIVER_NOT_AVAILABLE`) |
| **DP-17** | Hoàn tác bước 4: DB hết hạn và accept ngay | Chạy tay | `POST /api/v1/trips/:id/accept` | `D1 / DRIVER` | *(None)* | **400 Bad Request** (`INVALID_TRIP_STATUS`) |
| **DP-18** | Nhảy cóc trạng thái | Chạy tay | `POST /complete` & `/picking-up` | `D1 & D2 / DRIVER` | *(None)* | Complete khi PICKING_UP $\rightarrow$ **400**; D2 pickup $\rightarrow$ **403** |
| **DP-19** | Downstream failure: pricing chết | Chạy tay | `POST /api/v1/trips` | `C1 / CUSTOMER` | Tọa độ P1 $\rightarrow$ P2 | **503 Service Unavailable** (`SERVICE_UNAVAILABLE`) |

---

## 5. HƯỚNG DẪN CHI TIẾT CÁC CA CHẠY TAY

### DP-02: D1 và D2 tranh cuốc đồng thời (Distributed Lock)
- **Điều kiện ban đầu:** C1 đặt xe nhận `$trip_id`. D1 và D2 cùng được mời.
- **Lệnh chạy:**
  ```powershell
  $j1 = Start-Job { param($t, $d) Call-Dispatch DRIVER $d POST "/api/v1/trips/$t/accept" } -ArgumentList $trip_id, $d1_id
  $j2 = Start-Job { param($t, $d) Call-Dispatch DRIVER $d POST "/api/v1/trips/$t/accept" } -ArgumentList $trip_id, $d2_id
  Receive-Job $j1, $j2 -Wait
  ```
- **Kỳ vọng:** 1 job trả `200 OK` (`ACCEPTED`), 1 job trả `409 Conflict` (`TRIP_ALREADY_TAKEN`).
- **Dọn dẹp:** Tài xế thắng hoàn thành cuốc: `Call-Dispatch DRIVER $winner_id POST "/api/v1/trips/$trip_id/complete"`.

---

### DP-04: C1 đặt lần 2 khi đang có chuyến active
- **Điều kiện ban đầu:** C1 đang có cuốc ở trạng thái `MATCHING` hoặc `ACCEPTED`.
- **Lệnh chạy:**
  ```powershell
  $bodyP1P2 | Call-Dispatch CUSTOMER $c1_id POST "/api/v1/trips"
  ```
- **Kỳ vọng:** `409 Conflict` (`ACTIVE_TRIP_EXISTS`).
- **Dọn dẹp:** Không có.

---

### DP-06: Chuyến MATCHING hết hạn sau 30 giây
- **Điều kiện ban đầu:** C1 đặt xe nhận `$trip_id`, không có tài xế nào accept, chờ $\ge$ 30 giây để sweeper quét.
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch CUSTOMER $c1_id GET "/api/v1/trips/$trip_id"
  ```
- **Kỳ vọng:** `200 OK`, `status = "EXPIRED"`, timeline note: `TIMEOUT_30S`.
- **Dọn dẹp:** Không có.

---

### DP-07a: Khách C1 hủy chuyến lúc MATCHING
- **Điều kiện ban đầu:** C1 đặt xe nhận `$trip_id`.
- **Lệnh chạy:**
  ```powershell
  '{"reason":"Doi y"}' | Call-Dispatch CUSTOMER $c1_id POST "/api/v1/trips/$trip_id/cancel"
  ```
- **Kỳ vọng:** `200 OK` (`status = "CANCELLED"`).
- **Dọn dẹp:** Không có.

---

### DP-07b: Khách C1 hủy chuyến lúc ACCEPTED
- **Điều kiện ban đầu:** C1 đặt xe, D1 accept cuốc.
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch CUSTOMER $c1_id POST "/api/v1/trips/$trip_id/cancel"
  ```
- **Kỳ vọng:** `200 OK` (`status = "CANCELLED"`).
- **Dọn dẹp:** D1 tự động hoàn tác về `ONLINE`.

---

### DP-07c: Tài xế D1 hủy chuyến lúc ACCEPTED
- **Điều kiện ban đầu:** C1 đặt xe, D1 accept cuốc.
- **Lệnh chạy:**
  ```powershell
  '{"reason":"Hong xe"}' | Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/cancel"
  ```
- **Kỳ vọng:** `200 OK` (`status = "CANCELLED"`).
- **Dọn dẹp:** D1 tự động hoàn tác về `ONLINE`.

---

### DP-07d: Tài xế D1 hủy chuyến lúc PICKING_UP
- **Điều kiện ban đầu:** C1 đặt xe, D1 accept, D1 gọi picking-up.
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/cancel"
  ```
- **Kỳ vọng:** `200 OK` (`status = "CANCELLED"`).
- **Dọn dẹp:** D1 tự động hoàn tác về `ONLINE`.

---

### DP-07e: Khách hoặc tài xế cố hủy chuyến lúc IN_TRIP
- **Điều kiện ban đầu:** C1 đặt xe, D1 accept, picking-up, start-trip (`status = IN_TRIP`).
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch CUSTOMER $c1_id POST "/api/v1/trips/$trip_id/cancel"
  ```
- **Kỳ vọng:** `400 Bad Request` (`INVALID_TRIP_STATUS`).
- **Dọn dẹp:** D1 hoàn thành cuốc: `Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/complete"`.

---

### DP-08b: Admin Force-cancel thành công
- **Điều kiện ban đầu:** C1 đặt xe, D1 accept (`status = ACCEPTED`).
- **Lệnh chạy:**
  ```powershell
  '{"reason":"Su co nghiem trong"}' | Call-Dispatch ADMIN $admin_id POST "/api/v1/admin/trips/$trip_id/force-cancel"
  ```
- **Kỳ vọng:** `200 OK` (`status = "CANCELLED"`, `cancelled_by = "ADMIN"`).
- **Dọn dẹp:** D1 được giải phóng về `ONLINE`, xóa `trip:link:$d1_id`.

---

### DP-09: Cùng tài xế accept lần 2 (Idempotent)
- **Điều kiện ban đầu:** C1 đặt xe, D1 accept lần 1 thành công (`200 OK`).
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/accept"
  ```
- **Kỳ vọng:** `200 OK` (`status = "ACCEPTED"`, `driver_id = $d1_id`).
- **Dọn dẹp:** D1 hoàn thành cuốc.

---

### DP-12b: Khách gọi POST /accept
- **Điều kiện ban đầu:** C1 đặt xe nhận `$trip_id`.
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch CUSTOMER $c1_id POST "/api/v1/trips/$trip_id/accept"
  ```
- **Kỳ vọng:** `403 Forbidden` (`FORBIDDEN`).
- **Dọn dẹp:** D1 accept và hoàn thành chuyến hoặc C1 hủy chuyến.

---

### DP-13: Khách hủy MATCHING rồi tài xế accept
- **Điều kiện ban đầu:** C1 đặt xe nhận `$trip_id`. C1 hủy lúc MATCHING (`200 OK`).
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/accept"
  ```
- **Kỳ vọng:** `400 Bad Request` (`INVALID_TRIP_STATUS`).
- **Dọn dẹp:** D1 vẫn giữ `ONLINE`, lock đã nhả.

---

### DP-14b: Admin truyền page không hợp lệ
- **Điều kiện ban đầu:** Không có.
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch ADMIN $admin_id GET "/api/v1/admin/trips?page=abc"
  ```
- **Kỳ vọng:** `400 Bad Request` (`VALIDATION_ERROR`).
- **Dọn dẹp:** Không có.

---

### DP-16: D1 được mời rồi cho OFFLINE, accept
- **Điều kiện ban đầu:** C1 đặt xe nhận `$trip_id`. Cho D1 sang `OFFLINE`:
  ```powershell
  '{"status":"OFFLINE"}' | curl.exe -s -X PATCH -H "Content-Type: application/json" -H "X-User-Id: $d1_id" -H "X-User-Role: DRIVER" --data-binary "@-" http://127.0.0.1:8001/api/v1/driver/status
  ```
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/accept"
  ```
- **Kỳ vọng:** `400 Bad Request` (`DRIVER_NOT_AVAILABLE`).
- **Kiểm tra:**
  1. D1 vẫn giữ trạng thái (kiểm tra qua `filter-online`):
     ```powershell
     '{"driver_ids":["' + $d1_id + '"]}' | curl.exe -s -X POST -H "Content-Type: application/json" --data-binary "@-" http://127.0.0.1:8001/internal/v1/users/filter-online
     ```
  2. Lock đã nhả:
     ```powershell
     docker exec redis redis-cli -a "$REDIS_PASSWORD" --no-auth-warning GET "ride:lock:$trip_id"
     ```
     Kỳ vọng trả về `(nil)`.
- **Dọn dẹp:** Bật lại D1 `ONLINE`:
  ```powershell
  '{"status":"ONLINE"}' | curl.exe -s -X PATCH -H "Content-Type: application/json" -H "X-User-Id: $d1_id" -H "X-User-Role: DRIVER" --data-binary "@-" http://127.0.0.1:8001/api/v1/driver/status
  '{"reason":"Don dep"}' | Call-Dispatch CUSTOMER $c1_id POST "/api/v1/trips/$trip_id/cancel"
  ```

---

### DP-17: Hoàn tác bước 4: DB hết hạn và accept ngay
- **Điều kiện ban đầu:** C1 đặt xe nhận `$trip_id`. Cập nhật thời gian hết hạn lùi về quá khứ:
  ```powershell
  docker exec -i postgres psql -U dispatch_user -d dispatchdb -c "UPDATE trips SET matching_expires_at = now() - interval '1 second' WHERE id='$trip_id'"
  ```
- **Lệnh chạy:**
  ```powershell
  Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/accept"
  ```
- **Kỳ vọng:** `400 Bad Request` (`INVALID_TRIP_STATUS`), message chứa `"không còn ở trạng thái MATCHING hoặc đã hết hạn"`.
- **Kiểm tra:**
  1. D1 vẫn ONLINE:
     ```powershell
     '{"driver_ids":["' + $d1_id + '"]}' | curl.exe -s -X POST -H "Content-Type: application/json" --data-binary "@-" http://127.0.0.1:8001/internal/v1/users/filter-online
     ```
     `$d1_id` nằm trong `online_driver_ids`.
  2. Lock đã nhả:
     ```powershell
     docker exec redis redis-cli -a "$REDIS_PASSWORD" --no-auth-warning GET "ride:lock:$trip_id"
     ```
     Kỳ vọng trả về `(nil)`.
- **Dọn dẹp:** Không có.

---

### DP-18: Nhảy cóc trạng thái chuyến
- **Điều kiện ban đầu:** C1 đặt xe, D1 accept, D1 gọi picking-up (chuyến đang ở trạng thái `PICKING_UP`).
- **Lệnh chạy:**
  1. D1 nhảy cóc gọi complete:
     ```powershell
     Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/complete"
     ```
     $\rightarrow$ Kỳ vọng `400 Bad Request` (`INVALID_TRIP_STATUS`).
  2. D2 (tài xế khác) cố gọi picking-up chuyến của D1:
     ```powershell
     Call-Dispatch DRIVER $d2_id POST "/api/v1/trips/$trip_id/picking-up"
     ```
     $\rightarrow$ Kỳ vọng `403 Forbidden` (`FORBIDDEN`).
- **Dọn dẹp:** D1 tiếp tục start-trip và complete:
  ```powershell
  Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/start-trip"
  Call-Dispatch DRIVER $d1_id POST "/api/v1/trips/$trip_id/complete"
  ```

---

### DP-19: Downstream Failure (pricing-service chết)
- **Điều kiện ban đầu:** Tắt pricing-service:
  ```powershell
  docker stop pricing-service
  ```
- **Lệnh chạy:**
  ```powershell
  $bodyP1P2 | Call-Dispatch CUSTOMER $c1_id POST "/api/v1/trips"
  ```
- **Kỳ vọng:** `503 Service Unavailable` (`SERVICE_UNAVAILABLE`).
- **Dọn dẹp:** Khởi động lại pricing-service:
  ```powershell
  docker start pricing-service
  ```
