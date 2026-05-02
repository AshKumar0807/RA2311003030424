# Notification System Design

---

## Stage 1

### Overview

A campus notification platform delivering real-time alerts to students for Placements, Events, and Results. The API is REST-ful with consistent JSON schemas and a real-time mechanism based on Server-Sent Events (SSE).

---

### Core Actions

| Action | Description |
|--------|-------------|
| List notifications | Paginated notifications for the logged-in student |
| Get single notification | Full detail of one notification by ID |
| Mark as read | Mark one notification as read |
| Mark all as read | Mark every unread notification as read |
| Delete notification | Remove a notification from the inbox |
| Get unread count | Lightweight badge count endpoint |
| Stream (real-time) | SSE stream for live push to browser/app |

---

### REST API Endpoints

#### 1. List Notifications

```
GET /api/v1/notifications
```

**Headers**
```
Authorization: Bearer <jwt_token>
Accept: application/json
```

**Query Parameters**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| page | int | 1 | Page number |
| limit | int | 20 | Items per page (max 100) |
| type | string | (all) | Filter: Placement, Event, Result |
| isRead | bool | (all) | Filter by read status |

**Response 200**
```json
{
  "data": [
    {
      "id": "b283218f-ea5a-4b7c-93a9-1f2f240d64b0",
      "type": "Placement",
      "message": "CSX Corporation hiring",
      "isRead": false,
      "createdAt": "2026-04-22T17:51:18Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 142,
    "unreadCount": 7
  }
}
```

---

#### 2. Get Single Notification

```
GET /api/v1/notifications/:id
```

**Response 200**
```json
{
  "id": "b283218f-ea5a-4b7c-93a9-1f2f240d64b0",
  "type": "Placement",
  "message": "CSX Corporation hiring",
  "isRead": true,
  "createdAt": "2026-04-22T17:51:18Z",
  "readAt": "2026-04-22T18:00:00Z"
}
```

---

#### 3. Mark Notification as Read

```
PATCH /api/v1/notifications/:id/read
```

**Response 200**
```json
{
  "id": "b283218f-ea5a-4b7c-93a9-1f2f240d64b0",
  "isRead": true,
  "readAt": "2026-04-22T18:00:00Z"
}
```

---

#### 4. Mark All as Read

```
PATCH /api/v1/notifications/read-all
```

**Response 200**
```json
{ "updatedCount": 7 }
```

---

#### 5. Delete Notification

```
DELETE /api/v1/notifications/:id
```

**Response 204** — No body.

---

#### 6. Get Unread Count

```
GET /api/v1/notifications/unread-count
```

**Response 200**
```json
{ "unreadCount": 7 }
```

---

#### 7. Real-Time Stream (SSE)

```
GET /api/v1/notifications/stream
```

**Headers**
```
Authorization: Bearer <jwt_token>
Accept: text/event-stream
Cache-Control: no-cache
```

Each SSE frame:
```
id: b283218f-ea5a-4b7c-93a9-1f2f240d64b0
event: notification
data: {"id":"b283218f-ea5a-4b7c-93a9-1f2f240d64b0","type":"Placement","message":"CSX Corporation hiring","isRead":false,"createdAt":"2026-04-22T17:51:18Z"}

```

---

### Real-Time Mechanism — Server-Sent Events (SSE)

SSE is chosen over WebSockets because notifications are unidirectional (server to client). SSE works over plain HTTP, supports native browser auto-reconnect via Last-Event-ID, and is trivially load-balanced via Redis Pub/Sub.

```
Student Browser
     |
     |  GET /api/v1/notifications/stream  (persistent HTTP)
     v
  API Server
     |
     |  SUB notifications:{studentID}
     v
  Redis Pub/Sub  <---- any service PUBLISHes here when a notification is created
```

---

## Stage 2

### Recommended Database: PostgreSQL

**Rationale:**
- Notifications have a well-defined stable schema — relational fits naturally.
- PostgreSQL supports partial indexes, composite indexes, and native ENUM types critical for notification query patterns.
- JSONB columns allow flexible metadata without schema migrations.
- Strong ACID guarantees prevent double-delivery bugs in concurrent inserts.
- Mature ecosystem: PgBouncer, read replicas, logical replication.

---

### DB Schema

```sql
CREATE TYPE notification_type AS ENUM ('Placement', 'Event', 'Result');

CREATE TABLE students (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    email       VARCHAR(255) UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id          UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    notification_type   notification_type NOT NULL,
    message             TEXT NOT NULL,
    metadata            JSONB,
    is_read             BOOLEAN NOT NULL DEFAULT false,
    read_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial index: only unread rows — small, fast
CREATE INDEX idx_notifications_student_unread
    ON notifications (student_id, is_read, created_at DESC)
    WHERE is_read = false;

-- Full pagination index
CREATE INDEX idx_notifications_student_created
    ON notifications (student_id, created_at DESC);

-- Admin queries by type
CREATE INDEX idx_notifications_type_created
    ON notifications (notification_type, created_at DESC);
```

---

### Problems at Scale and Solutions

| Problem | Why it happens | Solution |
|---------|---------------|----------|
| Slow reads | Table grows to hundreds of millions of rows | Range partitioning by created_at (monthly) |
| Hot student rows | Many concurrent reads per student | Read replicas; route all SELECTs to replica |
| Fan-out bottleneck | 50k synchronous INSERTs blocks the API | Async queue with batch workers |
| Index bloat | Dead tuples from UPDATE is_read | Tuned autovacuum |
| Connection exhaustion | 50k SSE connections | PgBouncer in transaction mode |

---

### SQL Queries

**List notifications (paginated)**
```sql
SELECT id, notification_type, message, is_read, created_at
FROM   notifications
WHERE  student_id = $1
ORDER  BY created_at DESC
LIMIT  $2 OFFSET $3;
```

**Mark as read**
```sql
UPDATE notifications
SET    is_read = true, read_at = now()
WHERE  id = $1 AND student_id = $2 AND is_read = false
RETURNING id, is_read, read_at;
```

**Mark all as read**
```sql
UPDATE notifications
SET    is_read = true, read_at = now()
WHERE  student_id = $1 AND is_read = false;
```

**Unread count**
```sql
SELECT COUNT(*) AS unread_count
FROM   notifications
WHERE  student_id = $1 AND is_read = false;
```

---

## Stage 3

### Is the original query accurate?

```sql
SELECT * FROM notifications
WHERE studentID = 1042 AND isRead = false
ORDER BY createdAt DESC;
```

Functionally correct — returns the right rows. However:
- SELECT * fetches every column including heavy JSONB metadata unnecessarily.
- No LIMIT means thousands of rows could be returned for one student in one call.

---

### Why is it slow?

Without a composite index, PostgreSQL performs a full sequential scan of 5,000,000 rows, filters by studentID, then filters by isRead = false, then sorts — O(N log N) over the full table. Even with a simple index on studentID alone, all rows for that student are scanned before the isRead filter is applied.

---

### What to change

**Add a composite partial index:**
```sql
CREATE INDEX idx_notifications_student_unread
    ON notifications (student_id, is_read, created_at DESC)
    WHERE is_read = false;
```

**Improved query:**
```sql
SELECT id, notification_type, message, created_at
FROM   notifications
WHERE  student_id = $1
  AND  is_read = false
ORDER  BY created_at DESC
LIMIT  20;
```

**Cost after fix:** Index seek on a B-tree leaf for student_id, forward scan of created_at DESC on only unread rows — O(log N + k) where k is unread count for that student. For 50,000 students with ~100 unread each, each query touches ~100 rows, not 5,000,000.

---

### Is indexing every column good advice?

No. Each index is a separate B-tree updated on every INSERT, UPDATE, and DELETE. With 50,000 simultaneous notification inserts, write throughput collapses. Indexes consume disk and RAM. The query planner may pick a sub-optimal index when too many exist. Index based on actual query patterns — composite indexes matching WHERE + ORDER BY, with partial indexes for constant filter predicates.

---

### Query: students who got a Placement notification in the last 7 days

```sql
SELECT DISTINCT s.id, s.name, s.email
FROM   students s
JOIN   notifications n ON n.student_id = s.id
WHERE  n.notification_type = 'Placement'
  AND  n.created_at >= now() - INTERVAL '7 days';
```

---

## Stage 4

### Problem

Every page load fires a live DB query. At 50,000 students refreshing during placement season, the DB receives tens of thousands of near-identical queries per second.

---

### Solutions and Tradeoffs

#### Strategy 1 — Redis Application Cache

Cache each student's notification list and unread count with a TTL (e.g. 30 seconds).

| Pro | Con |
|-----|-----|
| 95%+ cache-hit ratio, sub-millisecond reads | Stale data up to TTL |
| Dramatically reduces DB load | Cache invalidation must happen on every write |
| Redis scales horizontally | Adds operational complexity |

Invalidation: on any write (insert, mark-read), evict that student's Redis key.

---

#### Strategy 2 — HTTP ETags

Return an ETag header on list responses. Clients send If-None-Match; server returns 304 Not Modified if nothing changed.

| Pro | Con |
|-----|-----|
| No stale data | Server still computes ETag on each request |
| Reduces payload bytes | Slightly more complex client logic |

---

#### Strategy 3 — Read Replicas

Route all SELECT queries to a PostgreSQL read replica.

| Pro | Con |
|-----|-----|
| Doubles read capacity, transparent to application | Replication lag (~1s) means slight delay in seeing new notifications |
| No application cache to manage | Adds cost and operational overhead |

---

#### Recommended Combined Approach

1. Redis per-student cache (TTL 30s) — handles the bulk of read load.
2. Invalidate Redis key on every write.
3. Read replica for cache misses.
4. ETag on list endpoint to reduce payload re-transfer.

---

## Stage 5

### Shortcomings of the proposed pseudocode

1. **Sequential loop** — 50,000 students at 50ms each = ~42 minutes. Students at the end of the list are notified hours later.
2. **No atomicity** — if send_email succeeds but save_to_db fails, the student gets an email with no in-app record.
3. **No error handling** — 200 email failures are silently lost with no replay path.
4. **Single point of failure** — a crash mid-loop leaves an unknown cohort un-notified.
5. **Tight coupling** — a slow push provider blocks the email for every subsequent student.

---

### Logs show 200 email failures — what now?

With the current design, those 200 students are lost. There is no record of which IDs failed or whether the DB insert had already occurred.

---

### Redesigned Implementation

```
HR clicks "Notify All"
        |
        v
POST /api/v1/notifications/broadcast
        |
        v
  API enqueues 50,000 messages into Queue (Redis Streams / SQS)
        |
  Return 202 Accepted immediately
        |
  +-----+------------------------------------------+
  |          Notification Workers (N pods)          |
  |                                                 |
  |  for each message:                              |
  |    1. save_to_db(student_id, message)  <- first |
  |    2. send_email(student_id, message)           |
  |    3. push_to_app(student_id, notif_id)         |
  |    4. ack message                               |
  |                                                 |
  |  on failure: nack -> retry (max 3x, exp backoff)|
  |  after 3 failures: dead-letter queue            |
  +-------------------------------------------------+
```

**Revised pseudocode:**

```python
function broadcast_handler(student_ids: array, message: string) -> Response:
    batch_id = generate_uuid()
    for student_id in student_ids:
        enqueue(queue="notifications", payload={
            "batch_id": batch_id,
            "student_id": student_id,
            "message": message,
            "attempts": 0
        })
    log("backend", "info", "handler",
        f"broadcast enqueued: batch_id={batch_id} recipients={len(student_ids)}")
    return Response(202, {"batchId": batch_id, "enqueued": len(student_ids)})


function notification_worker():
    while true:
        job = dequeue(queue="notifications", timeout=5s)
        if job is None: continue

        try:
            # DB first — this is the durable source of truth
            notification_id = save_to_db(job.student_id, job.message)
            log("backend", "info", "db",
                f"notification saved: id={notification_id} student={job.student_id}")

            send_email(job.student_id, job.message)
            log("backend", "info", "service",
                f"email sent: student={job.student_id} batch={job.batch_id}")

            push_to_app(job.student_id, notification_id)
            log("backend", "info", "service",
                f"push sent: student={job.student_id}")

            ack(job)

        except EmailError as e:
            log("backend", "error", "service",
                f"email failed: student={job.student_id} attempt={job.attempts} err={e}")
            job.attempts += 1
            if job.attempts < 3:
                requeue_with_backoff(job)
            else:
                move_to_dead_letter(job)
                log("backend", "fatal", "service",
                    f"email permanently failed after 3 attempts: student={job.student_id}")

        except DBError as e:
            log("backend", "fatal", "db",
                f"DB insert failed: student={job.student_id} err={e}")
            nack(job)
```

---

### Should saving to DB and sending email happen in the same transaction?

**No.** The DB is the system of record and should be written first, independently. Email is an external side effect — external API calls must never be inside a DB transaction because: (a) they can be slow, holding locks; (b) if the email succeeds but the DB commit fails, you cannot un-send the email. The correct pattern is write DB first, then attempt side effects. If email fails, the notification exists in DB and can be retried. If DB fails, nothing is sent — the student is never in an inconsistent state.

---

## Stage 6

### Approach: Priority Inbox with a Min-Heap

**Scoring formula:**

```
priority_score = type_weight + recency_score

type_weight:
  Placement -> 300
  Result    -> 200
  Event     -> 100

recency_score = max(0, 100 - floor(hours_since_notification))
(notifications older than 100 hours contribute 0 from recency)

final_priority_score = type_weight + recency_score
```

**Why a min-heap?**

A fixed-size min-heap of capacity N maintains the top-N notifications in O(log N) per insertion:
- If heap size < N: push the notification.
- If heap size == N and new score > heap minimum: pop the minimum, push the new notification.
- Otherwise: discard.

This runs efficiently as new notifications continuously arrive from the SSE stream — no full re-sort of the list is ever needed.

**Handling continuous new notifications:**

When a new notification arrives via the real-time stream, compute its score and run the heap insertion. The heap always reflects the current top-N without rescanning all notifications.

See `notification_app_be/priority_inbox.go` for the complete working Go implementation.

The program:
1. Fetches notifications from `http://20.207.122.201/evaluation-service/notifications`
2. Scores each notification using the formula above
3. Uses a min-heap to efficiently find the top 10
4. Prints results in ranked order with score breakdown

Screenshots of the output are in `notification_app_be/screenshots/`.
