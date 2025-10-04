package sqlinline

const QUpsertGoogleUser = `--sql 5a82e2ad-7b09-40c5-9d22-2d28db58c0f0
with incoming as (
    select
        $1::text as google_sub,
        $2::text as email,
        $3::text as name,
        $4::text as picture,
        $5::text as locale,
        $6::text as country
),
upserted as (
    insert into users (id, clerk_user_id, email, name, avatar_url, plan, locale_pref, google_sub, last_ip_country, last_seen_at, properties, created_at, updated_at)
    values (gen_random_uuid(), (select google_sub from incoming), (select email from incoming), (select name from incoming),
            (select picture from incoming), 'free', (select locale from incoming), (select google_sub from incoming),
            nullif((select country from incoming), ''), now(),
            jsonb_build_object(
                'quota_daily', 2,
                'quota_used_today', 0,
                'preferred_locale', (select locale from incoming),
                'google_sub', (select google_sub from incoming),
                'google_email', (select email from incoming),
                'google_name', (select name from incoming),
                'google_picture', (select picture from incoming),
                'google_locale', (select locale from incoming)
            ), now(), now())
    on conflict (email) do update set
        clerk_user_id = excluded.clerk_user_id,
        name = excluded.name,
        avatar_url = excluded.avatar_url,
        locale_pref = excluded.locale_pref,
        google_sub = excluded.google_sub,
        last_ip_country = coalesce(nullif((select country from incoming), ''), users.last_ip_country),
        last_seen_at = now(),
        updated_at = now(),
        properties = jsonb_set(
            jsonb_set(
                jsonb_set(
                    jsonb_set(
                        jsonb_set(
                            jsonb_set(
                                users.properties,
                                '{preferred_locale}', to_jsonb((select locale from incoming)), true
                            ),
                            '{google_sub}', to_jsonb((select google_sub from incoming)), true
                        ),
                        '{google_email}', to_jsonb((select email from incoming)), true
                    ),
                    '{google_name}', to_jsonb((select name from incoming)), true
                ),
                '{google_picture}', to_jsonb((select picture from incoming)), true
            ),
            '{google_locale}', to_jsonb((select locale from incoming)), true
        )
    returning id, plan, properties
),
target_user as (
    select u.id, u.plan, u.properties from upserted u
    union all
    select existing.id, existing.plan, existing.properties
    from users existing
    where existing.email = (select email from incoming)
    limit 1
),
linked as (
    insert into external_accounts (id, user_id, provider, external_user_id, properties, created_at, updated_at)
    values (
        gen_random_uuid(),
        (select id from target_user),
        'google',
        (select google_sub from incoming),
        jsonb_build_object(
            'google_sub', (select google_sub from incoming),
            'google_email', (select email from incoming),
            'google_name', (select name from incoming),
            'google_picture', (select picture from incoming),
            'google_locale', (select locale from incoming)
        ),
        now(),
        now()
    )
    on conflict (provider, external_user_id) do update set
        user_id = excluded.user_id,
        updated_at = now(),
        properties = excluded.properties
    returning 1
)
select t.id, t.plan, t.properties
from target_user t
left join linked l on true;
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
