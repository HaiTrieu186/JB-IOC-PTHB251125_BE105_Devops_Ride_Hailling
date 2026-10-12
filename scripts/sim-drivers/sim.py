#!/usr/bin/env python3
"""
Ride-Hailing Simulation Script for Virtual Drivers and Trips.
Strictly relies on LLD ws-gateway and public REST APIs of user, dispatch, and payment services.
"""

import argparse
import asyncio
import json
import math
import random
import sys
import time
from typing import Dict, List, Optional, Tuple

import aiohttp

# Ensure UTF-8 output on Windows
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

# --- Constants & Coordinates ---
PASSWORD_DEFAULT = "Test@1234"
P1_LAT = 10.7725
P1_LNG = 106.6980
RADIUS_3KM_LAT = 3000.0 / 111000.0  # ~0.0270 deg
RADIUS_3KM_LNG = 3000.0 / (111000.0 * math.cos(math.radians(P1_LAT)))  # ~0.0275 deg

# Concurrency control for auth (bcrypt protection on 1 vCPU)
auth_semaphore = asyncio.Semaphore(2)

# Shared in-memory state
gps_timestamps: Dict[Tuple[str, float, float], float] = {}  # (driver_id, lat, lng) -> t_sent
latency_records: List[float] = []  # in milliseconds
trip_plans: Dict[str, str] = {}  # trip_id -> "COMPLETE" | "CANCEL" | "EXPIRE"
trip_results: Dict[str, str] = {}  # trip_id -> "COMPLETED" | "CANCELLED" | "EXPIRED" | "ERROR"

# Control flags
stop_event = asyncio.Event()


def get_random_location_near_p1(radius_m: float = 3000.0) -> Tuple[float, float]:
    """Generate random (lat, lng) within radius_m of P1."""
    angle = random.uniform(0, 2 * math.pi)
    r = math.sqrt(random.uniform(0, 1)) * radius_m
    dlat = (r * math.cos(angle)) / 111000.0
    dlng = (r * math.sin(angle)) / (111000.0 * math.cos(math.radians(P1_LAT)))
    return round(P1_LAT + dlat, 6), round(P1_LNG + dlng, 6)


def get_destination_offset(lat: float, lng: float, dist_m_min: float = 2000.0, dist_m_max: float = 4000.0) -> Tuple[float, float]:
    """Generate random destination 2-4 km away from origin."""
    angle = random.uniform(0, 2 * math.pi)
    dist = random.uniform(dist_m_min, dist_m_max)
    dlat = (dist * math.cos(angle)) / 111000.0
    dlng = (dist * math.sin(angle)) / (111000.0 * math.cos(math.radians(lat)))
    return round(lat + dlat, 6), round(lng + dlng, 6)


class UserSession:
    def __init__(self, phone: str, role: str, full_name: str, api_url: str, session: aiohttp.ClientSession):
        self.phone = phone
        self.role = role
        self.full_name = full_name
        self.api_url = api_url
        self.session = session
        self.token: Optional[str] = None
        self.user_id: Optional[str] = None

    async def register(self) -> None:
        async with auth_semaphore:
            url = f"{self.api_url}/api/v1/auth/register"
            payload = {
                "phone_number": self.phone,
                "password": PASSWORD_DEFAULT,
                "full_name": self.full_name,
                "role": self.role,
            }
            try:
                async with self.session.post(url, json=payload, timeout=aiohttp.ClientTimeout(total=15)) as resp:
                    # 201 or 409 (USER_ALREADY_EXISTS) are expected
                    pass
            except Exception as e:
                # Log non-fatal error
                pass

    async def login(self) -> bool:
        async with auth_semaphore:
            url = f"{self.api_url}/api/v1/auth/login"
            payload = {
                "phone_or_email": self.phone,
                "password": PASSWORD_DEFAULT,
            }
            try:
                async with self.session.post(url, json=payload, timeout=aiohttp.ClientTimeout(total=15)) as resp:
                    if resp.status == 200:
                        data = await resp.json()
                        if data.get("success") and "data" in data:
                            self.token = data["data"].get("access_token")
                            user = data["data"].get("user", {})
                            self.user_id = user.get("id")
                            return True
            except Exception as e:
                pass
            return False

    async def ensure_auth(self) -> bool:
        await self.register()
        return await self.login()

    def get_auth_headers(self) -> Dict[str, str]:
        return {"Authorization": f"Bearer {self.token}"} if self.token else {}

    async def request_with_reauth(self, method: str, path: str, json_data: Optional[dict] = None) -> Tuple[int, dict]:
        """Perform REST request with 1 re-login retry on 401."""
        url = f"{self.api_url}{path}"
        for attempt in range(2):
            headers = self.get_auth_headers()
            try:
                async with self.session.request(method, url, headers=headers, json=json_data, timeout=aiohttp.ClientTimeout(total=10)) as resp:
                    try:
                        body = await resp.json()
                    except Exception:
                        body = {}
                    if resp.status == 401 and attempt == 0:
                        # Re-login once
                        logged_in = await self.login()
                        if not logged_in:
                            return resp.status, body
                        continue
                    return resp.status, body
            except Exception as e:
                return 500, {"error": {"code": "NETWORK_ERROR", "message": str(e)}}
        return 401, {"error": {"code": "UNAUTHORIZED", "message": "Failed after re-auth"}}


class VirtualDriver:
    def __init__(self, idx: int, api_url: str, ws_url: str, session: aiohttp.ClientSession):
        self.idx = idx
        self.phone = f"0903{idx:06d}"
        self.name = f"Driver {idx:04d}"
        self.api_url = api_url
        self.ws_url = ws_url
        self.session = session
        self.user_session = UserSession(self.phone, "DRIVER", self.name, api_url, session)
        self.lat, self.lng = get_random_location_near_p1()
        self.ws: Optional[aiohttp.ClientWebSocketResponse] = None

    async def setup(self) -> bool:
        ok = await self.user_session.ensure_auth()
        if not ok:
            return False
        # PATCH ONLINE (ignore DRIVER_BUSY)
        await self.patch_online()
        return True

    async def patch_online(self) -> None:
        status, body = await self.user_session.request_with_reauth("PATCH", "/api/v1/driver/status", {"status": "ONLINE"})
        # DRIVER_BUSY is normal if interrupted earlier

    async def patch_offline(self) -> None:
        try:
            await self.user_session.request_with_reauth("PATCH", "/api/v1/driver/status", {"status": "OFFLINE"})
        except Exception:
            pass

    async def run(self) -> None:
        backoff = 1.0
        while not stop_event.is_set():
            headers = self.user_session.get_auth_headers()
            try:
                async with self.session.ws_connect(self.ws_url, headers=headers, heartbeat=25.0) as ws:
                    self.ws = ws
                    backoff = 1.0  # reset backoff on success
                    # Run sender and receiver concurrently
                    sender_task = asyncio.create_task(self._send_gps_loop(ws))
                    receiver_task = asyncio.create_task(self._receive_loop(ws))

                    done, pending = await asyncio.wait(
                        [sender_task, receiver_task, asyncio.create_task(stop_event.wait())],
                        return_when=asyncio.FIRST_COMPLETED,
                    )
                    for task in pending:
                        task.cancel()
            except aiohttp.WSServerHandshakeError as e:
                if e.status == 401:
                    await self.user_session.login()
                jitter = random.uniform(0, 1.0)
                await asyncio.sleep(min(backoff, 30.0) + jitter)
                backoff = min(backoff * 2.0, 30.0)
            except Exception as e:
                if stop_event.is_set():
                    break
                jitter = random.uniform(0, 1.0)
                await asyncio.sleep(min(backoff, 30.0) + jitter)
                backoff = min(backoff * 2.0, 30.0)
            finally:
                self.ws = None

    async def _send_gps_loop(self, ws: aiohttp.ClientWebSocketResponse) -> None:
        while not stop_event.is_set() and not ws.closed:
            # Shift location slightly (5 to 15m)
            step_angle = random.uniform(0, 2 * math.pi)
            step_m = random.uniform(5.0, 15.0)
            dlat = (step_m * math.cos(step_angle)) / 111000.0
            dlng = (step_m * math.sin(step_angle)) / (111000.0 * math.cos(math.radians(P1_LAT)))
            self.lat = round(self.lat + dlat, 6)
            self.lng = round(self.lng + dlng, 6)

            # Record timestamp for latency tracking
            if self.user_session.user_id:
                gps_timestamps[(self.user_session.user_id, self.lat, self.lng)] = time.perf_counter()

            payload = {"latitude": self.lat, "longitude": self.lng}
            await ws.send_json(payload)
            await asyncio.sleep(random.uniform(3.0, 5.0))

    async def _receive_loop(self, ws: aiohttp.ClientWebSocketResponse) -> None:
        async for msg in ws:
            if stop_event.is_set():
                break
            if msg.type == aiohttp.WSMsgType.TEXT:
                try:
                    data = json.loads(msg.data)
                except Exception:
                    continue

                event = data.get("event")
                if event == "TRIP_OFFERED":
                    trip_id = data.get("trip_id")
                    if trip_id:
                        asyncio.create_task(self._handle_offer(trip_id))
            elif msg.type in (aiohttp.WSMsgType.CLOSED, aiohttp.WSMsgType.ERROR):
                break

    async def _handle_offer(self, trip_id: str) -> None:
        # Wait up to 2s for customer to record plan in trip_plans
        for _ in range(40):
            if trip_id in trip_plans:
                break
            await asyncio.sleep(0.05)

        # Check trip plan
        plan = trip_plans.get(trip_id, "COMPLETE")
        if plan == "EXPIRE":
            # Ignore offer, let it expire
            return

        # Wait 1-3s before accepting
        await asyncio.sleep(random.uniform(1.0, 3.0))
        if stop_event.is_set():
            return

        status_code, body = await self.user_session.request_with_reauth("POST", f"/api/v1/trips/{trip_id}/accept")
        err_code = (body.get("error") or {}).get("code") if isinstance(body, dict) else None

        # Normal codes to ignore: TRIP_ALREADY_TAKEN, NOT_OFFERED, DRIVER_NOT_AVAILABLE, INVALID_TRIP_STATUS
        if status_code != 200:
            if err_code in ("TRIP_ALREADY_TAKEN", "NOT_OFFERED", "DRIVER_NOT_AVAILABLE", "INVALID_TRIP_STATUS"):
                return
            # Other error
            return

        # Accept succeeded! Driver won the trip.
        if plan == "CANCEL":
            await asyncio.sleep(random.uniform(2.0, 3.0))
            if stop_event.is_set():
                return
            await self.user_session.request_with_reauth("POST", f"/api/v1/trips/{trip_id}/cancel", {"reason": "Virtual driver cancel"})
        elif plan == "COMPLETE":
            # picking-up -> start-trip -> complete (each separated by 2-3s)
            await asyncio.sleep(random.uniform(2.0, 3.0))
            if stop_event.is_set():
                return
            await self.user_session.request_with_reauth("POST", f"/api/v1/trips/{trip_id}/picking-up")

            await asyncio.sleep(random.uniform(2.0, 3.0))
            if stop_event.is_set():
                return
            await self.user_session.request_with_reauth("POST", f"/api/v1/trips/{trip_id}/start-trip")

            await asyncio.sleep(random.uniform(2.0, 3.0))
            if stop_event.is_set():
                return
            await self.user_session.request_with_reauth("POST", f"/api/v1/trips/{trip_id}/complete")


class VirtualCustomer:
    def __init__(self, idx: int, api_url: str, ws_url: str, session: aiohttp.ClientSession):
        self.idx = idx
        self.phone = f"0904{idx:06d}"
        self.name = f"Customer {idx:04d}"
        self.api_url = api_url
        self.ws_url = ws_url
        self.session = session
        self.user_session = UserSession(self.phone, "CUSTOMER", self.name, api_url, session)
        self.current_trip_id: Optional[str] = None
        self.trip_done_event = asyncio.Event()

    async def setup(self) -> bool:
        ok = await self.user_session.ensure_auth()
        if not ok:
            return False
        # Top-up 2,000,000 VND once at start
        await self.user_session.request_with_reauth("POST", "/api/v1/wallets/top-up", {"amount": 2000000})
        # Check active trip recovery
        await self.recover_active_trip()
        return True

    async def recover_active_trip(self) -> None:
        status, body = await self.user_session.request_with_reauth("GET", "/api/v1/trips/current")
        data = body.get("data") if isinstance(body, dict) else None
        if status == 200 and isinstance(data, dict):
            trip_id = data.get("id") or (data.get("trip") or {}).get("id")
            if trip_id:
                # Try cancel
                c_status, c_body = await self.user_session.request_with_reauth("POST", f"/api/v1/trips/{trip_id}/cancel", {"reason": "cancel_existing"})
                if c_status != 200:
                    print(f"[WARN] Customer {self.phone} could not cancel active trip {trip_id}. Runbook force-cancel may be needed.")

    async def run_ws_listener(self) -> None:
        backoff = 1.0
        while not stop_event.is_set():
            headers = self.user_session.get_auth_headers()
            try:
                async with self.session.ws_connect(self.ws_url, headers=headers, heartbeat=25.0) as ws:
                    backoff = 1.0
                    async for msg in ws:
                        if stop_event.is_set():
                            break
                        if msg.type == aiohttp.WSMsgType.TEXT:
                            try:
                                data = json.loads(msg.data)
                            except Exception:
                                continue

                            event = data.get("event")
                            if event == "DRIVER_LOCATION_UPDATED":
                                d_id = data.get("driver_id")
                                lat = round(float(data.get("latitude", 0)), 6)
                                lng = round(float(data.get("longitude", 0)), 6)
                                key = (d_id, lat, lng)
                                if key in gps_timestamps:
                                    t_sent = gps_timestamps.pop(key)
                                    latency_ms = (time.perf_counter() - t_sent) * 1000.0
                                    latency_records.append(latency_ms)

                            elif event == "TRIP_STATUS_UPDATED":
                                t_id = data.get("trip_id")
                                t_status = data.get("status")
                                if t_id and t_id == self.current_trip_id:
                                    if t_status in ("COMPLETED", "CANCELLED", "EXPIRED"):
                                        trip_results[t_id] = t_status
                                        self.trip_done_event.set()
                        elif msg.type in (aiohttp.WSMsgType.CLOSED, aiohttp.WSMsgType.ERROR):
                            break
            except Exception:
                if stop_event.is_set():
                    break
                await asyncio.sleep(min(backoff, 10.0))
                backoff = min(backoff * 2.0, 10.0)

    async def execute_trip(self, trip_seq: int) -> bool:
        """Create a trip and wait for it to reach terminal status."""
        # Determine plan based on sequence: ~70% COMPLETE, 15% CANCEL, 15% EXPIRE
        rem = trip_seq % 20
        if rem in (14, 15, 16):
            plan = "CANCEL"
        elif rem in (17, 18, 19):
            plan = "EXPIRE"
        else:
            plan = "COMPLETE"

        # Record balance before trip
        w_status, w_body = await self.user_session.request_with_reauth("GET", "/api/v1/wallets/me")
        prev_balance = (w_body.get("data") or {}).get("balance", 0) if isinstance(w_body, dict) else 0

        # Generate pickup & dropoff
        p_lat, p_lng = get_random_location_near_p1(radius_m=3000.0)
        d_lat, d_lng = get_destination_offset(p_lat, p_lng, 2000.0, 4000.0)
        payload = {
            "pickup_lat": p_lat,
            "pickup_lng": p_lng,
            "pickup_address": f"Pickup Point {trip_seq}",
            "dropoff_lat": d_lat,
            "dropoff_lng": d_lng,
            "dropoff_address": f"Dropoff Point {trip_seq}",
        }

        # Create trip (retry top-up if INSUFFICIENT_BALANCE)
        create_status, create_body = await self.user_session.request_with_reauth("POST", "/api/v1/trips", payload)
        if create_status == 400 and (create_body.get("error") or {}).get("code") == "INSUFFICIENT_BALANCE":
            await self.user_session.request_with_reauth("POST", "/api/v1/wallets/top-up", {"amount": 2000000})
            create_status, create_body = await self.user_session.request_with_reauth("POST", "/api/v1/trips", payload)

        if create_status == 409 and (create_body.get("error") or {}).get("code") == "ACTIVE_TRIP_EXISTS":
            await self.recover_active_trip()
            create_status, create_body = await self.user_session.request_with_reauth("POST", "/api/v1/trips", payload)

        if create_status != 201 and create_status == 503:
            await asyncio.sleep(1.0)
            create_status, create_body = await self.user_session.request_with_reauth("POST", "/api/v1/trips", payload)

        if create_status != 201:
            err = (create_body.get("error") or {}).get("code", "UNKNOWN_ERROR") if isinstance(create_body, dict) else "UNKNOWN_ERROR"
            print(f"[WARN] Trip #{trip_seq} creation failed: {create_status} {err}")
            trip_results[f"trip_err_{trip_seq}"] = f"ERR_{err}"
            return False

        trip_data = (create_body.get("data") or {}) if isinstance(create_body, dict) else {}
        trip_id = trip_data.get("id")
        initial_status = trip_data.get("status")

        self.current_trip_id = trip_id
        self.trip_done_event.clear()
        trip_plans[trip_id] = plan

        if initial_status == "EXPIRED":
            trip_results[trip_id] = "EXPIRED"
            self.current_trip_id = None
            return True

        # Wait for terminal status via WS or fallback polling
        wait_deadline = time.monotonic() + (45.0 if plan == "EXPIRE" else 30.0)
        while time.monotonic() < wait_deadline and not stop_event.is_set():
            if self.trip_done_event.is_set() or trip_id in trip_results:
                break
            # Fallback poll status every 3s
            await asyncio.sleep(3.0)
            p_status, p_body = await self.user_session.request_with_reauth("GET", f"/api/v1/trips/{trip_id}")
            if p_status == 200 and isinstance(p_body, dict):
                t_st = (p_body.get("data") or {}).get("trip", {}).get("status")
                if t_st in ("COMPLETED", "CANCELLED", "EXPIRED"):
                    trip_results[trip_id] = t_st
                    break

        final_st = trip_results.get(trip_id, "TIMEOUT")
        trip_results[trip_id] = final_st
        self.current_trip_id = None

        # If completed, poll GET /api/v1/wallets/me until balance decreases (max 15s)
        if final_st == "COMPLETED":
            poll_deadline = time.monotonic() + 15.0
            while time.monotonic() < poll_deadline and not stop_event.is_set():
                w_status, w_body = await self.user_session.request_with_reauth("GET", "/api/v1/wallets/me")
                if w_status == 200 and isinstance(w_body, dict):
                    curr_balance = (w_body.get("data") or {}).get("balance", prev_balance)
                    if curr_balance < prev_balance:
                        break
                await asyncio.sleep(0.5)

        return True


def print_summary() -> None:
    print("\n" + "=" * 50)
    print("           KẾT QUẢ MÔ PHỎNG CHI TIẾT")
    print("=" * 50)

    # Trip summary
    total_trips = len(trip_results)
    if total_trips > 0:
        c_completed = sum(1 for s in trip_results.values() if s == "COMPLETED")
        c_cancelled = sum(1 for s in trip_results.values() if s == "CANCELLED")
        c_expired = sum(1 for s in trip_results.values() if s == "EXPIRED")
        c_other = total_trips - (c_completed + c_cancelled + c_expired)

        print(f"Tổng số chuyến kết thúc: {total_trips}")
        print(f"  - COMPLETED : {c_completed} ({c_completed / total_trips * 100:.1f}%)")
        print(f"  - CANCELLED : {c_cancelled} ({c_cancelled / total_trips * 100:.1f}%)")
        print(f"  - EXPIRED   : {c_expired} ({c_expired / total_trips * 100:.1f}%)")
        if c_other > 0:
            print(f"  - Lỗi khác  : {c_other}")
    else:
        print("Chế độ: Chỉ giữ tài xế gửi GPS (trips = 0)")

    # Latency summary
    n_samples = len(latency_records)
    print("\n--- ĐỘ TRỄ CHUYỂN TIẾP VỊ TRÍ (DRIVER_LOCATION_UPDATED) ---")
    if n_samples > 0:
        sorted_lat = sorted(latency_records)
        p50 = sorted_lat[int(n_samples * 0.50)]
        p95 = sorted_lat[int(n_samples * 0.95)]
        max_lat = sorted_lat[-1]
        print(f"Số mẫu ghi nhận: {n_samples}")
        print(f"p50 : {p50:.2f} ms")
        print(f"p95 : {p95:.2f} ms")
        print(f"max : {max_lat:.2f} ms")
    else:
        print("Không có mẫu độ trễ (chưa phát sinh chuyến có khách theo dõi).")
    print("=" * 50)


async def main():
    parser = argparse.ArgumentParser(description="Simulation script for Ride-Hailing drivers and trips")
    parser.add_argument("--api-url", default="http://127.0.0.1:8080", help="API Gateway URL")
    parser.add_argument("--ws-url", default=None, help="WebSocket Gateway URL")
    parser.add_argument("--drivers", type=int, default=100, help="Number of virtual drivers")
    parser.add_argument("--duration", type=int, default=300, help="Duration in seconds")
    parser.add_argument("--trips", type=int, default=0, help="Number of trips to simulate (0 = drivers only)")

    args = parser.parse_args()

    # Determine WS URL
    api_url = args.api_url.rstrip("/")
    if args.ws_url:
        ws_url = args.ws_url
    else:
        ws_base = api_url.replace("http://", "ws://").replace("https://", "wss://")
        ws_url = f"{ws_base}/ws/"
    if not ws_url.endswith("/"):
        ws_url += "/"

    print(f"[*] API URL: {api_url}")
    print(f"[*] WS URL : {ws_url}")
    print(f"[*] Tài xế : {args.drivers}")
    print(f"[*] Số chuyến: {args.trips}")
    print(f"[*] Thời lượng trần: {args.duration}s")

    # Safety ceiling for duration (max 900s)
    duration_limit = min(args.duration, 900)

    # Connector with high pool limit for 100+ websockets
    connector = aiohttp.TCPConnector(limit=500, limit_per_host=500)
    async with aiohttp.ClientSession(connector=connector) as session:
        # Initialize virtual drivers
        print(f"[*] Đang khởi tạo và đăng nhập {args.drivers} tài xế ảo...")
        drivers = [VirtualDriver(i + 1, api_url, ws_url, session) for i in range(args.drivers)]

        # Setup drivers concurrently (bounded by auth_semaphore)
        driver_setup_tasks = [d.setup() for d in drivers]
        setup_results = await asyncio.gather(*driver_setup_tasks)
        active_drivers = [d for d, ok in zip(drivers, setup_results) if ok]
        print(f"[+] {len(active_drivers)}/{args.drivers} tài xế sẵn sàng.")

        # Launch driver background tasks
        driver_tasks = [asyncio.create_task(d.run()) for d in active_drivers]

        # Initialize customers if in trip mode
        customer_tasks = []
        customer_ws_tasks = []
        customers: List[VirtualCustomer] = []
        if args.trips > 0:
            num_customers = min(10, args.trips)
            print(f"[*] Đang khởi tạo {num_customers} khách hàng ảo...")
            customers = [VirtualCustomer(i + 1, api_url, ws_url, session) for i in range(num_customers)]
            c_setup = await asyncio.gather(*[c.setup() for c in customers])
            active_customers = [c for c, ok in zip(customers, c_setup) if ok]
            print(f"[+] {len(active_customers)} khách hàng sẵn sàng.")

            # Launch customer WS listeners
            customer_ws_tasks = [asyncio.create_task(c.run_ws_listener()) for c in active_customers]

            # Trip coordinator
            async def trip_worker(cust: VirtualCustomer, trip_queue: asyncio.Queue):
                while not stop_event.is_set():
                    try:
                        seq = trip_queue.get_nowait()
                    except asyncio.QueueEmpty:
                        break
                    await cust.execute_trip(seq)
                    trip_queue.task_done()

            trip_queue = asyncio.Queue()
            for seq in range(1, args.trips + 1):
                trip_queue.put_nowait(seq)

            for c in active_customers:
                customer_tasks.append(asyncio.create_task(trip_worker(c, trip_queue)))

        # Wait loop
        start_time = time.monotonic()
        try:
            while not stop_event.is_set():
                elapsed = time.monotonic() - start_time
                if elapsed >= duration_limit:
                    print(f"\n[*] Đạt ngưỡng thời gian trần {duration_limit}s. Đang dừng...")
                    break

                if args.trips > 0:
                    if len(trip_results) >= args.trips:
                        print(f"\n[*] Hoàn thành toàn bộ {args.trips} chuyến xe! Đang dừng...")
                        break

                await asyncio.sleep(1.0)
        except KeyboardInterrupt:
            print("\n[!] Nhận tín hiệu ngắt (Ctrl+C). Đang dọn dẹp...")
        finally:
            stop_event.set()

            # Wait for customer workers if any
            if customer_tasks:
                for t in customer_tasks:
                    t.cancel()
                await asyncio.gather(*customer_tasks, return_exceptions=True)

            if customer_ws_tasks:
                for t in customer_ws_tasks:
                    t.cancel()
                await asyncio.gather(*customer_ws_tasks, return_exceptions=True)

            # Cancel driver WS tasks
            for t in driver_tasks:
                t.cancel()
            await asyncio.gather(*driver_tasks, return_exceptions=True)

            # PATCH OFFLINE drivers
            print("[*] Đang chuyển trạng thái tài xế về OFFLINE...")
            offline_tasks = [d.patch_offline() for d in active_drivers]
            await asyncio.gather(*offline_tasks, return_exceptions=True)

    print_summary()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
