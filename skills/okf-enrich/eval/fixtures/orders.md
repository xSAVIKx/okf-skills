---
type: SQLite Table
title: orders
description:
resource: sqlite:///shop.db/orders
tags: [sqlite, table]
---
# Columns

| Name | Type | Primary Key | Nullable | Default |
| --- | --- | --- | --- | --- |
| id | INTEGER | Yes | No |  |
| customer_id | INTEGER | No | No |  |
| status | TEXT | No | No |  |
| total_cents | INTEGER | No | No |  |
| created_at | TEXT | No | No |  |

# Relationships

- FK on customer_id [customers](/tables/customers.md)

## Data Profile

| Column | Non-Null | Null | Distinct | Min | Max | Semantic |
| --- | --- | --- | --- | --- | --- | --- |
| id | 1200 | 0 | 1200 | 1 | 1200 | fk-ish |
| customer_id | 1200 | 0 | 340 | 1 | 340 | fk-ish |
| status | 1200 | 0 | 3 | cancelled | shipped | enum |
| total_cents | 1200 | 0 | 900 | 199 | 250000 | monetary |
| created_at | 1200 | 0 | 1180 | 2019-01-02 | 2026-06-10 | iso-timestamp |

Values:
- status ∈ {cancelled, pending, shipped}

## Sample

| id | customer_id | status | total_cents | created_at |
| --- | --- | --- | --- | --- |
| 1 | 12 | shipped | 4999 | 2019-01-02 |
| 2 | 12 | pending | 1299 | 2026-06-09 |
