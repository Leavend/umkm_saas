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
consumed as (
  update users u
  set properties = jsonb_set(
    u.properties,
    '{quota_used_today}',
    (coalesce((u.properties->>'quota_used_today')::int, 0) + (select quantity from input))::text::jsonb,
    true
  ),
  updated_at = now()
  where u.id = (select user_id from input)
  returning u.id
),
ins_job as (
    insert into generation_requests(
      id,
      user_id,
      task_type,
      status,
      prompt_json,
      quantity,
      aspect_ratio,
      provider,
    properties
  )
  values (
    gen_random_uuid(),
    (select user_id from input),
      'IMAGE_GEN',
      'RUNNING',
    (select prompt_json from input),
    (select quantity from input),
    (select aspect_ratio from input),
    (select provider from input),
    '{}'::jsonb
  )
  returning id
)
select id from ins_job;
`
