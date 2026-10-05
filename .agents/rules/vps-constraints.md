# Luật: Ràng buộc VPS
- VPS 1 GB RAM, 20 GB disk, Ubuntu.
- Không đề xuất Kafka, Elasticsearch/EFK, Java. Ưu tiên giải pháp nhẹ.
- Mọi container phải có mem_limit.
- Chỉ Nginx được publish cổng ra ngoài; còn lại dùng mạng nội bộ Docker.
- Monitoring (Prometheus, node_exporter) tách profile riêng để tắt được.
