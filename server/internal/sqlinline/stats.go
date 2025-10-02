package sqlinline

const QStatsSummary = `--sql 0f0557a2-1731-4fc6-8cbe-8540b1d2b6df
select
  total_users,
  image_generated,
  video_generated,
  request_success,
  request_fail,
  image_last24,
  video_last24
from vw_stats_summary;
`
