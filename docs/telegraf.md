# Telegraf and TimescaleDB

keypulse speaks InfluxDB line protocol, so Telegraf can take it anywhere. This
example stores it in TimescaleDB with compression.

## Telegraf

```toml
# Receive keypulse on this computer.
[[inputs.socket_listener]]
  service_address = "udp://127.0.0.1:8094"
  data_format = "influx"

[[outputs.postgresql]]
  connection = "host=db.example port=5432 dbname=metrics user=telegraf password=secret sslmode=disable"
  tags_as_foreign_keys = true
  timestamp_column_type = "timestamp with time zone"
  create_templates = [
    '''CREATE TABLE {{ .table }} ({{ .columns }})''',
    '''SELECT create_hypertable({{ .table|quoteLiteral }}, 'time', chunk_time_interval => INTERVAL '7 days')''',
    '''ALTER TABLE {{ .table }} SET (timescaledb.compress, timescaledb.compress_segmentby = 'tag_id')''',
    '''SELECT add_compression_policy({{ .table|quoteLiteral }}, INTERVAL '7 days')''',
  ]
```

A Telegraf on another machine works too: point `output.url` at it
(`udp://192.168.1.10:8094`), or keep a local Telegraf and forward with its MQTT
or HTTP output.

With `tags_as_foreign_keys`, tags live in `<table>_tag` tables, joined on `tag_id`.

## Queries for Grafana

Keys and clicks per minute:

```sql
SELECT $__timeGroupAlias(k.time, $__interval), sum(k.keys) AS keys, sum(k.clicks) AS clicks
FROM keypulse k
WHERE $__timeFilter(k.time)
GROUP BY 1 ORDER BY 1
```

Top applications by keys:

```sql
SELECT t.app, sum(k.keys) AS keys, sum(k.clicks) AS clicks, sum(k.active_s) / 3600.0 AS active_hours
FROM keypulse k JOIN keypulse_tag t ON t.tag_id = k.tag_id
WHERE $__timeFilter(k.time)
GROUP BY 1 ORDER BY 2 DESC LIMIT 15
```

Keyboard heatmap (`per_key: true`):

```sql
SELECT t.key, sum(k.count) AS presses
FROM keypulse_keys k JOIN keypulse_keys_tag t ON t.tag_id = k.tag_id
WHERE $__timeFilter(k.time)
GROUP BY 1 ORDER BY 2 DESC
```

Click heatmap, for an XY chart (y inverted so the top of the screen is up):

```sql
SELECT t.x::int AS x, -t.y::int AS y, sum(h.count) AS clicks
FROM keypulse_heatmap h JOIN keypulse_heatmap_tag t ON t.tag_id = h.tag_id
WHERE $__timeFilter(h.time) AND t.kind = 'click'
GROUP BY 1, 2
```
