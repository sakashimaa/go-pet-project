🛍️ Go Microservices E-Commerce Platform
High-performance, scalable e-commerce backend system built with Clean Architecture principles. The project demonstrates an advanced implementation of distributed transactions using the Saga Pattern (Choreography-based) with Transactional Outbox for data consistency.

🏗️ Architecture Overview
The system is composed of loose-coupled microservices communicating via gRPC (synchronous internal calls) and Apache Kafka (asynchronous event-driven flows).

🚀 Tech Stack

Language: Golang (1.25.4) + go.work file

Communication: gRPC (Protobuf), Apache Kafka (Sarama)

Storage: PostgreSQL (pgx driver), Redis (Caching & Idempotency)

Infrastructure: Docker, Docker Compose

Observability: Prometheus, Grafana, Jaeger (Distributed Tracing), Zap Logger

Resiliency: Circuit Breaker, Graceful Shutdown, Retry Mechanisms

🧩 Microservices

| Service       | Description                       | Key Features                                                    |
|---------------|-----------------------------------|-----------------------------------------------------------------|
| Gateway       | Entry point for external requests | API composition, routing, rate limiting                         |
| Auth          | Identity & Access Management      | JWT issuance/validation, Refresh tokens, User registration.     |
| Product       | 	Inventory & Catalog management   | Stock reservation, Optimistic locking, Caching.                 |
| Order	        | Order lifecycle management        | 	State machine (Created -> Paid/Cancelled), Saga Orchestration. |
 | Payment       | 	Payment processing simulation	   | Idempotency keys, Business logic validation, Fault injection.   |
 | Notification	 | User alerting system              | 	Async notifications via Telegram/Email based on Kafka events.  |


🔄 Key Patterns & Implementations
1. Saga Pattern (Choreography)

Distributed transactions are handled without a central orchestrator. Services react to domain events to maintain eventual consistency.

Happy Path: OrderCreated → InventoryReserved → PaymentProcessed → OrderPaid.

Compensating Transactions (Rollback): If payment fails, the system triggers a rollback flow:

PaymentFailed event is emitted.

Order Service listens -> changes status to CANCELLED -> emits OrderCancelled.

Product Service listens -> releases reserved stock back to inventory.

2. Transactional Outbox

To guarantee that database updates and Kafka event publishing happen atomically, the Outbox Pattern is implemented. A background worker reads events from the outbox table and publishes them to Kafka, ensuring no data loss even if the broker is temporarily down.

3. Observability

Full visibility into the system state:

Tracing: Every request has a TraceID propagated through HTTP headers, gRPC metadata, and Kafka headers. Visualized in Jaeger.

Metrics: RPS, Latency, and Error rates are scraped by Prometheus and visualized in Grafana.

🛠️ Getting Started
Prerequisites

Docker & Docker Compose

Go 1.25.4 (for local development)

Make

Running the System

Clone the repository

Bash
git clone https://github.com/your-username/go-pet-project.git
cd go-pet-project
Start Infrastructure & Services

Bash
docker-compose up -d --build
Apply Migrations

Bash
make migrate-up-<service_name> (for example migrate-up-auth)
Access Interfaces

Run
Bash
make run-<service_name> (for example make run-gateway)

Jaeger UI: http://localhost:16686

Grafana: http://localhost:3000

Prometheus: http://localhost:9090

🧪 Testing the Saga (Compensation)
To verify the rollback mechanism:

Create an order with an ID that triggers a mock payment failure (e.g., OrderID % 2 == 0).

Check Jaeger: Observe the trace flowing from Order -> Product -> Payment (Fail) -> Order (Cancel) -> Product (Release).

Check Database: Verify orders table has status CANCELLED and products stock is restored.

Developed with ❤️ and Go.
