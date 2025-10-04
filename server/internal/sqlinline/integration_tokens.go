package sqlinline

const QSelectIntegrationToken = `--sql 8a8e0d52-7f5d-4f21-8b7d-f7d4b821eed7
select token
from integration_tokens
where provider = $1::text
limit 1;
`

const QUpsertIntegrationToken = `--sql 6d4f5660-0f7c-4f73-a1f3-9ab6d5e6c7a3
with incoming as (
    select
        $1::text as provider,
        $2::text as token,
        coalesce($3::jsonb, '{}'::jsonb) as properties
)
insert into integration_tokens (id, provider, token, properties, created_at, updated_at)
values (gen_random_uuid(), (select provider from incoming), (select token from incoming), (select properties from incoming), now(), now())
on conflict (provider) do update set
    token = excluded.token,
    properties = excluded.properties,
    updated_at = now();
`
