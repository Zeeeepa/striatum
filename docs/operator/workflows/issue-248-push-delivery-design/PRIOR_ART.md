# Prior Art For Issue 248 Push Delivery

This note summarizes a short primary-source sweep for systems similar to
Striatum's proposed wake-on-work / push delivery design.

## Temporal Task Queues

Temporal workers poll task queues through a matching service; the architecture
documentation describes long-poll requests from workers being routed to the
matching service responsible for the task queue. This is close to Striatum's
current `work.await_packet` shape: workers stay resident and poll, but the poll
is long-lived and server-mediated rather than tight sleep/retry.

Useful lesson: long-polling is an accepted design when the worker process is
expected to stay resident. It does not solve Striatum's issue #248 goal of
letting a lane exit idle and be woken later.

Source: <https://github.com/temporalio/temporal/blob/main/docs/architecture/matching-service.md>

## Kubernetes List/Watch

Kubernetes clients list resources, receive a `resourceVersion`, and then start a
watch from that version to receive later changes. This separates durable state
from notification delivery: the watch is a resumable change stream over an
authoritative state store.

Useful lesson: if Striatum adds a push/wake surface, it should have a durable
cursor such as event id, queue message id, or run-scoped sequence. A reconnecting
client should be able to resume from authoritative daemon/PostgreSQL state
instead of trusting the notification channel as the record.

Source: <https://kubernetes.io/docs/reference/using-api/api-concepts/>

## PostgreSQL LISTEN/NOTIFY

PostgreSQL provides asynchronous notifications: a session registers with
`LISTEN`, and `NOTIFY` sends an event with an optional payload to currently
listening sessions.

Useful lesson: this is the most local-first, boring wake hint available because
Striatum already depends on PostgreSQL. Treat it as a hint only. It should wake a
driver or watcher that then reads durable queue/event rows; it should not become
the authoritative message bus.

Sources:

- <https://www.postgresql.org/docs/current/sql-listen.html>
- <https://www.postgresql.org/docs/current/sql-notify.html>
- <https://www.postgresql.org/docs/current/libpq-notify.html>

## NATS JetStream Consumers

JetStream distinguishes push consumers, where messages are delivered to a
subject, from pull consumers, where clients request batches on demand. Queue
groups and pull consumers are used to distribute load and manage horizontal
scaling.

Useful lesson: push vs pull is a product-level delivery choice, not just a
transport detail. Striatum's local-first boundary probably rules out embedding a
broker, but the distinction helps frame whether issue #248 is a notify/wake
feature, a true pushed delivery feature, or a pull/long-poll refinement.

Sources:

- <https://docs.nats.io/nats-concepts/jetstream/consumers>
- <https://docs.nats.io/nats-concepts/core-nats/queue>

## RabbitMQ Consumers

RabbitMQ documents consumer acknowledgement modes and recommends manual
acknowledgements with bounded prefetch to limit outstanding in-progress work.

Useful lesson: any Striatum push design still needs explicit ack/lease
semantics and backpressure. Delivery must not imply completion, and the system
should bound outstanding claimed work per lane/run.

Sources:

- <https://www.rabbitmq.com/docs/consumers>
- <https://www.rabbitmq.com/docs/consumer-prefetch>
- <https://www.rabbitmq.com/docs/confirms>

## systemd Socket Activation

systemd socket units can hold a socket and start a matching service when traffic
arrives.

Useful lesson: this is the closest local OS-level "wake a process" precedent.
It is attractive only if Striatum can define a local scheduler/principal model:
who is authorized to start a lane, as which OS user, with what token, and how
that wakeup is audited.

Source: <https://www.freedesktop.org/software/systemd/man/systemd.socket.html>

## GitHub Actions Self-Hosted Runners

GitHub's self-hosted runner model requires the runner application to be running
on the host to accept and run jobs.

Useful lesson: many production systems accept a resident runner as the simpler
contract. That is a useful counterexample: Striatum should only build exit-and-
wake if the owner accepts the extra scheduler/principal complexity; otherwise a
better long-poll or event-driven `run drive` loop may be the smaller design.

Source: <https://docs.github.com/en/actions/reference/runners/self-hosted-runners>

## Implication For The First Striatum Slice

The smallest likely-safe direction is not daemon-side auto-spawn. It is a
PostgreSQL-backed wake hint or event-stream cursor for the existing operator-side
driver:

- durable queue/event rows stay authoritative;
- notification loss is acceptable because the driver can relist/resume by cursor;
- `run drive` can block on wake instead of sleeping when no work is available;
- session creation and `supervise start` remain contemporaneous
  operator-authorized RPCs;
- #212 daemon-side spawn stays parked until the scheduler/principal model is
  accepted.
