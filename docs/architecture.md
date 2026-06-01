# Backend Architecture

## 1. System Context

```mermaid
flowchart LR
  FE[Storefront Web/Mobile] --> API[API Gateway: Gin Router]
  ADM[Admin Panel] --> API

  API --> AUTH[Auth & Identity]
  API --> CATALOG[Catalog]
  API --> CART[Cart]
  API --> CHECKOUT[Checkout]
  API --> OMS[Order Management]
  API --> PAY[Payments]
  API --> INV[Inventory]
  API --> CMS[CMS/Banners]
  API --> FIT[Fit Recommendation]
  API --> ANALYTICS[Analytics & Tracking]
  API --> NOTIFY[Notifications]

  AUTH --> PG[(PostgreSQL)]
  CATALOG --> PG
  CART --> PG
  CHECKOUT --> PG
  OMS --> PG
  PAY --> PG
  INV --> PG
  CMS --> PG
  FIT --> PG
  ANALYTICS --> PG
  NOTIFY --> PG

  PAY --> RAZ[Razorpay]
  OMS --> SHIP[Shipping Partners]
  NOTIFY --> SMS[SMS Provider]
  NOTIFY --> EMAIL[Email Provider]
  NOTIFY --> PUSH[Push Provider]
```

## 2. Auth and Identity

```mermaid
flowchart LR
  AH[Auth Handlers] --> AS[Auth Service]
  AS --> OTP[OTP Service]
  AS --> JWT[JWT Token Service]
  AS --> SESS[Session/Refresh Store]
  AS --> UR[User Repository]
  AS --> RBAC[Role Policy]
  UR --> PG[(PostgreSQL)]
  SESS --> REDIS[(Redis)]
  OTP --> SMS[SMS/Email Gateway]
```

## 3. Catalog and Product Data

```mermaid
flowchart LR
  CH[Catalog Handlers] --> CS[Catalog Service]
  CS --> PR[Product Repo]
  CS --> CR[Category Repo]
  CS --> BR[Brand Repo]
  CS --> ATTR[Fabric/Performance Attribute Mapper]
  CS --> BULK[Bulk Upload Processor]
  PR --> PG[(PostgreSQL)]
  CR --> PG
  BR --> PG
  BULK --> S3[(S3 Import Files)]
```

## 4. Cart and Checkout

```mermaid
flowchart LR
  CAH[Cart Handlers] --> CAS[Cart Service]
  CTH[Checkout Handlers] --> CTS[Checkout Service]
  CAS --> CARTDB[Cart Repo]
  CTS --> COUPON[Coupon Engine]
  CTS --> TAX[GST Calculator]
  CTS --> SHIPCHG[Shipping Charge Calculator]
  CTS --> COD[COD Eligibility Rules]
  CTS --> INVLOCK[Inventory Lock]
  CTS --> OMS[Order Service]
  CARTDB --> PG[(PostgreSQL)]
  COUPON --> PG
```

## 5. Orders, Shipping, Returns and Refunds

```mermaid
flowchart LR
  OH[Order Handlers] --> OS[Order Service]
  OS --> OSM[Order State Machine]
  OS --> TRACK[Shipment Tracking Sync]
  OS --> RET[Return Handler]
  OS --> REF[Refund Processor]
  OS --> CODREC[COD Reconciliation]
  OS --> OR[Order Repo]
  OR --> PG[(PostgreSQL)]
  TRACK --> SHIP[Shipping Webhooks/API]
  REF --> PAY[Payment Service]
```

## 6. Payments

```mermaid
flowchart LR
  PH[Payment Handlers] --> PS[Payment Service]
  PS --> PI[Payment Provider Interface]
  PI --> RZ[Razorpay Adapter]
  PH --> WH[Webhook Verifier + Idempotency]
  WH --> PS
  PS --> PR[Payment Repo]
  PR --> PG[(PostgreSQL)]
  RZ --> RAZ[Razorpay API/Webhooks]
```

## 7. Inventory

```mermaid
flowchart LR
  IH[Inventory Handlers] --> IS[Inventory Service]
  IS --> RES[Reservation Manager]
  IS --> ADJ[Manual Adjustment]
  IS --> ALERT[Low-stock Alerts]
  IS --> IR[Inventory Repo]
  IR --> PG[(PostgreSQL)]
  ALERT --> NOTIFY[Notification Service]
```

## 8. Fit Recommendation Engine (Phase 1)

```mermaid
flowchart LR
  FH[Fit Handlers] --> FS[Fit Rule Service]
  FS --> RULES[Configurable Rules]
  FS --> INPUT[Height/Weight/Fit Preference]
  FS --> OUT[Recommended Size + Confidence]
  RULES --> PG[(PostgreSQL)]
```

## 9. Admin and CMS

```mermaid
flowchart LR
  ADH[Admin Handlers] --> ADS[Admin Orchestration Service]
  ADS --> CATALOG[Catalog Service]
  ADS --> INV[Inventory Service]
  ADS --> OMS[Order Service]
  ADS --> CMS[Banner/CMS Content Service]
  ADS --> USERS[User Management Service]
  ADS --> AUDIT[Audit Log]
  AUDIT --> PG[(PostgreSQL)]
  CMS --> PG
```

## 10. Analytics and Event Tracking

```mermaid
flowchart LR
  EH[Event Ingestion API] --> BUS[Event Stream/Queue]
  BUS --> AGG[Aggregation Jobs]
  AGG --> MV[Reporting Tables/Materialized Views]
  ANH[Analytics Handlers] --> ANS[Analytics Query Service]
  ANS --> MV
  MV --> PG[(PostgreSQL)]
```

## 11. Notifications

```mermaid
flowchart LR
  NH[Notification Handlers] --> NS[Notification Service]
  NS --> TPL[Template Engine]
  NS --> NQ[Dispatcher Queue]
  NS --> LOG[Notification Log Repo]
  NQ --> EMAIL[Email]
  NQ --> SMS[SMS]
  NQ --> PUSH[Push]
  LOG --> PG[(PostgreSQL)]
```

## 12. Infrastructure Topology

```mermaid
flowchart LR
  U[Users] --> CDN[CDN]
  CDN --> ALB[AWS ALB]
  ALB --> ECS[ECS Service: Go API]
  ECS --> RDS[(RDS PostgreSQL)]
  ECS --> REDIS[(Redis Cache)]
  ECS --> S3[(S3 Media/Imports)]
  ECS --> CW[CloudWatch + Alarms]
  ECS --> SEC[Secrets Manager]
```

## 13. Cross-Cutting Requirements
- Environments: Development, Staging, Production.
- Security: SSL/TLS, secrets management, API rate limiting, WAF/firewall.
- Reliability: retries, idempotency keys, webhook signature checks.
- Operations: centralized logs, error tracking, SLO alerts, rollback runbook.
- Backup/DR: automated backups, retention policy, periodic restore tests.

