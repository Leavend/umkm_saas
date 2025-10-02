package sqlinline

const QUpsertGoogleUser = `--sql 5a82e2ad-7b09-40c5-9d22-2d28db58c0f0
with incoming as (
    select
        $1::text as google_sub,
        $2::text as email,
        $3::text as name,
        $4::text as picture,
        $5::text as locale
),
upserted as (
    insert into users (id, clerk_user_id, email, name, avatar_url, plan, locale_pref, google_sub, properties, created_at, updated_at)
    values (gen_random_uuid(), (select google_sub from incoming), (select email from incoming), (select name from incoming),
            (select picture from incoming), 'free', (select locale from incoming), (select google_sub from incoming),
            jsonb_build_object('quota_daily', 2, 'quota_used_today', 0, 'preferred_locale', (select locale from incoming)), now(), now())
    on conflict (email) do update set
        name = excluded.name,
        avatar_url = excluded.avatar_url,
        locale_pref = excluded.locale_pref,
        google_sub = excluded.google_sub,
        updated_at = now(),
        properties = jsonb_set(
            jsonb_set(users.properties, '{preferred_locale}', to_jsonb((select locale from incoming)), true),
            '{google_picture}', to_jsonb((select picture from incoming)), true
        )
    returning id, plan, properties
),
linked as (
    insert into external_accounts (id, user_id, provider, external_user_id, properties, created_at, updated_at)
    values (gen_random_uuid(), (select id from upserted), 'google', (select google_sub from incoming), '{}'::jsonb, now(), now())
    on conflict (provider, external_user_id) do update set
        user_id = excluded.user_id,
        updated_at = now()
    returning user_id
)
select u.id, u.plan, u.properties
from upserted u;
`

const QSelectUserByID = `--sql 1239018e-4f5f-46a0-8f0d-81b2a3a5f0f8
select
    id,
    coalesce(google_sub, clerk_user_id) as google_sub,
    email,
    coalesce(locale_pref, properties->>'preferred_locale') as locale,
    plan,
    properties,
    created_at,
    updated_at
from users
where id = $1::uuid
limit 1;
`
