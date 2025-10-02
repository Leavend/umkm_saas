package sqlinline

const QListAssetsByUser = `--sql 6fe62992-02b6-41a4-8829-2b9f384182d0
select
  id,
  request_id,
  storage_key,
  mime,
  bytes,
  width,
  height,
  aspect_ratio,
  properties,
  created_at
from assets
where user_id = $1::uuid
order by created_at desc
limit $2::int offset $3::int;
`

const QSelectAssetByID = `--sql 5e1a10af-829f-4e1d-9f62-9d725d543b48
select id, user_id, storage_key, mime, bytes, properties
from assets
where id = $1::uuid
limit 1;
`
