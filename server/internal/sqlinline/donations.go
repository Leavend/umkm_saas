package sqlinline

const QInsertDonation = `--sql 9b79c57c-3615-48a2-9d85-3426d5b3f7eb
insert into donations(id, user_id, amount_int, note, testimonial, properties, created_at, updated_at)
values (gen_random_uuid(), nullif($1::text, '')::uuid, $2::bigint, $3::text, $4::text, coalesce($5::jsonb, '{}'::jsonb), now(), now())
returning id;
`

const QListDonations = `--sql 7a08e4f6-cb8a-42c4-bd7f-291d6e913edc
select id, user_id, amount_int, note, testimonial, properties, created_at
from donations
order by created_at desc
limit $1::int;
`
