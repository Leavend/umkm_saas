package sqlinline

const QInsertUsageEvent = `--sql e40f651c-a8b3-44c7-a911-bb8a0ed5f6ef
insert into usage_events(id, user_id, request_id, event_type, success, latency_ms, created_at, properties)
values (gen_random_uuid(), $1::uuid, $2::uuid, $3::text, $4::boolean, $5::int, now(), coalesce($6::jsonb, '{}'::jsonb));
`
