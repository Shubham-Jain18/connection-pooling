# Go Database Connection Pooling Benchmark

## Code Overview
This repository contains a Go application demonstrating the performance differences between managed and unmanaged database connections.

* `pool` Struct: A custom connection pool using a buffered channel (`chan *sql.DB`) to manage a fixed number of concurrent database connections.
* `newPool(maxConnections, dsn)`: Initializes the database connection object, restricts it to a single underlying TCP socket via `SetMaxOpenConns(1)` and `SetMaxIdleConns(1)`, verifies the connection with `Ping()`, and pre-fills the buffered channel.
* `acquire()` and `release()`: Thread-safe methods to pull from and push to the buffered channel. `acquire` blocks goroutines when the pool is empty.
* `benchmarkNonPool`: Spawns concurrent goroutines where each executes `sql.Open`, establishes a fresh TCP handshake, runs `SELECT SLEEP(0.01)`, and closes the connection.
* `benchmarkPool`: Spawns concurrent goroutines that retrieve an already established connection from the buffered channel, execute the query, and return the connection.

## Setup Instructions

**1. Database Configuration**
Ensure a local MySQL server is running on port 3306. Update the global maximum connections variable in the MySQL prompt to support the benchmark load:

```sql
SET GLOBAL max_connections = 1000;
```

**2. Environment Variables**
Create a `.env` file in the project root directory:

```text
DB_USER=YourUsername
DB_PASS=YourPassword
DB_HOST=127.0.0.1
DB_PORT=3306
DB_NAME=YourDatabaseName
```

**3. Dependency Installation**
Initialize the Go module and install required packages:

```bash
go mod init pool-test
go get [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
go get [github.com/joho/godotenv](https://github.com/joho/godotenv)
```

**4. Execution**
Run the benchmark suite:

```bash
go run main.go
```

## Benchmark Output

| Concurrent Queries | Non-Pool Time | Pool Time (50 Connections) |
| :--- | :--- | :--- |
| 10 | 12.92ms | 10.38ms |
| 20 | 12.97ms | 10.89ms |
| 30 | 17.80ms | 11.29ms |
| 40 | 18.71ms | 12.05ms |
| 50 | 17.03ms | 12.59ms |
| 75 | 18.77ms | 22.77ms |
| 100 | 20.79ms | 22.31ms |
| 150 | 23.34ms | 34.64ms |
| 200 | 24.67ms | 46.22ms |
| 300 | 1.016s | 63.18ms |
| 400 | 1.032s | 87.97ms |

## Output Analysis: System Overwhelm

The benchmark data isolates the exact threshold where unmanaged network architecture fails.

**Non-Pool Behavior:**
Execution remains stable up to 200 concurrent queries (12ms to 24ms). The operating system handles the creation of 200 concurrent TCP sockets efficiently. At 300 and 400 queries, execution time surges to over 1 second. This indicates network threshold collapse. The operating system TCP backlog or the database connection manager is completely overwhelmed by the sudden burst of socket requests. Hardware compute cycles are consumed by network protocol negotiation and thread contention rather than executing the actual 10ms query.

**Pool Behavior:**
Execution time scales linearly and predictably. The pool is restricted to 50 physical connections.
* 50 queries: ~12ms (1 batch)
* 100 queries: ~22ms (2 batches)
* 200 queries: ~46ms (4 batches)
* 400 queries: ~87ms (8 batches)

The channel strictures force 400 concurrent requests to queue and execute in 8 sequential batches. The database is shielded from traffic spikes, and zero time is wasted on establishing new TCP handshakes.

## Connection Pooling Utility

**Why Use Connection Pooling**
* Mitigates Network Latency: Bypasses the three-way TCP handshake, TLS negotiation, and database authentication protocols for every query.
* Prevents Denial of Service: Caps the maximum number of connections, preventing traffic spikes from exhausting database server memory or OS file descriptors.
* Resource Management: Reuses expensive memory allocations associated with connection objects.

**When to Use Connection Pooling**
Implement connection pooling for persistent applications communicating with a client-server database over a network.
* Web servers processing concurrent HTTP requests.
* Background worker queues handling continuous asynchronous tasks.
* Persistent microservices and daemons.

**When NOT to Use Connection Pooling**
Disable connection pooling in architectures where maintaining persistent sockets is either impossible or redundant.
* **Embedded Databases:** Databases like SQLite read directly from local disk files. There is no TCP network layer or remote authentication to bypass.
* **Single-Execution Scripts:** Cron jobs, database migration files, or CLI tools that execute a finite set of commands and immediately terminate. Initializing a multi-connection pool adds latency.
* **Serverless Functions:** Ephemeral computing environments (e.g., AWS Lambda) scale by spinning up new isolated instances. If 1000 serverless functions initialize simultaneously, they create 1000 isolated pools, multiplying connections and overwhelming the database. These architectures require connections to be opened and closed per execution, relying on an external centralized proxy server (like PgBouncer or RDS Proxy) to handle the actual pooling logic.