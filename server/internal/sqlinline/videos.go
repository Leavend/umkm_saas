package sqlinline

const QEnqueueVideoJob = `--sql c64c96de-6ba5-4b47-94f9-d811423d0235
with input as (
  select
    $1::uuid as user_id,
    $2::jsonb as prompt_json,
    $3::text as provider
),
quota as (
  select remaining from fn_consume_quota((select user_id from input), 1)
),
job as (
  select job_id from fn_insert_job_and_usage(
    (select user_id from input),
    'VIDEO_GEN',
    'QUEUED',
    (select prompt_json from input),
    1,
    '16:9',
    (select provider from input),
    '{}'::jsonb
  )
)
select job.job_id, quota.remaining
from job, quota;
`
