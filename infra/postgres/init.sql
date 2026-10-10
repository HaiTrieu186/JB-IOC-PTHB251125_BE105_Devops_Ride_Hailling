-- ==============================================================================
-- Khởi tạo Database và User riêng biệt cho từng microservice (Database-per-service)
-- Mỗi service sở hữu 1 user và 1 DB riêng với quyền OWNER để chạy GORM AutoMigrate.
-- Thu hồi quyền CONNECT từ PUBLIC để đảm bảo cô lập dữ liệu hoàn toàn giữa các service.
-- ==============================================================================

-- 1. user-service
CREATE USER user_user WITH PASSWORD 'user_pass';
CREATE DATABASE userdb OWNER user_user;
REVOKE CONNECT ON DATABASE userdb FROM PUBLIC;
GRANT CONNECT ON DATABASE userdb TO user_user;

-- 2. dispatch-service
CREATE USER dispatch_user WITH PASSWORD 'dispatch_pass';
CREATE DATABASE dispatchdb OWNER dispatch_user;
REVOKE CONNECT ON DATABASE dispatchdb FROM PUBLIC;
GRANT CONNECT ON DATABASE dispatchdb TO dispatch_user;

-- 3. pricing-service
CREATE USER pricing_user WITH PASSWORD 'pricing_pass';
CREATE DATABASE pricingdb OWNER pricing_user;
REVOKE CONNECT ON DATABASE pricingdb FROM PUBLIC;
GRANT CONNECT ON DATABASE pricingdb TO pricing_user;

-- 4. payment-service
CREATE USER payment_user WITH PASSWORD 'payment_pass';
CREATE DATABASE paymentdb OWNER payment_user;
REVOKE CONNECT ON DATABASE paymentdb FROM PUBLIC;
GRANT CONNECT ON DATABASE paymentdb TO payment_user;

-- 5. ai-service
CREATE USER ai_user WITH PASSWORD 'ai_pass';
CREATE DATABASE aidb OWNER ai_user;
REVOKE CONNECT ON DATABASE aidb FROM PUBLIC;
GRANT CONNECT ON DATABASE aidb TO ai_user;
