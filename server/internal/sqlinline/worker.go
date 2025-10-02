package sqlinline

const QWorkerClaimJob = `--sql 4f55a9b7-4e9f-4e45-a3b3-5a532d21d9db
with next_job as (
    select id
    from generation_requests
    where status = 'QUEUED'
    order by created_at asc
    for update skip locked
    limit 1
),
updated as (
    update generation_requests
    set status = 'RUNNING', updated_at = now()
    where id in (select id from next_job)
    returning id, user_id, provider, quantity, aspect_ratio, prompt_json
)
select * from updated;
`
