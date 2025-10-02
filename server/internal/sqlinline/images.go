package sqlinline

const QEnqueueImageJob = `--sql 2caa5b21-4c2b-4b72-8a36-7d3d0f9b77a1
with
input as (
  select
    $1::uuid     as user_id,
    $2::jsonb    as prompt_json,
    $3::int      as quantity,
    $4::text     as aspect_ratio,
    $5::text     as provider
),
quota as (
  select remaining from fn_consume_quota((select user_id from input), (select quantity from input))
),
job as (
  select job_id from fn_insert_job_and_usage(
    (select user_id from input),
    'IMAGE_GEN',
    'QUEUED',
    (select prompt_json from input),
    (select quantity from input),
    (select aspect_ratio from input),
    (select provider from input),
    '{}'::jsonb
  )
)
select job.job_id, quota.remaining
from job, quota;
`

const QUpdateJobStatus = `--sql 7936d6c1-ff41-47c1-970a-9bf27fa24d3b
update generation_requests
set status = $2::text,
    updated_at = now(),
    properties = jsonb_set(coalesce(properties, '{}'::jsonb), '{status_history}', coalesce(properties->'status_history', '[]'::jsonb) || jsonb_build_object('status', $2::text, 'at', now()), true)
where id = $1::uuid;
`

const QInsertAsset = `--sql 1a0b29f1-9b31-4d4c-9f5c-52dd2ad9f267
insert into assets(
  id,
  user_id,
  kind,
  request_id,
  storage_key,
  mime,
  bytes,
  width,
  height,
  aspect_ratio,
  properties,
  created_at,
  updated_at
) values (
  gen_random_uuid(),
  $1::uuid,
  $2::text,
  $3::uuid,
  $4::text,
  $5::text,
  $6::bigint,
  $7::int,
  $8::int,
  $9::text,
  $10::jsonb,
  now(),
  now()
) returning id;
`

const QSelectJobStatus = `--sql 8f12e6f8-812e-4c0d-bf9a-57f6318c12fb
select id, user_id, task_type, status, provider, quantity, aspect_ratio, created_at, updated_at, properties
from generation_requests
where id = $1::uuid
limit 1;
`

const QSelectJobAssets = `--sql 2ad63a52-99e0-4f3d-b0c8-34b18f81c979
select id, storage_key, mime, bytes, width, height, aspect_ratio, properties, created_at
from assets
where request_id = $1::uuid
order by created_at asc;
`
