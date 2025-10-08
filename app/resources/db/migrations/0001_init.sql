-- 0001_init.sql

CREATE TABLE IF NOT EXISTS public.packet_max (
  packet_id UUID NOT NULL,
  source_id UUID NOT NULL,
  value     DOUBLE PRECISION NOT NULL,
  ts        TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (packet_id, source_id)
);

CREATE INDEX IF NOT EXISTS packet_max_ts_idx
  ON public.packet_max (ts DESC);

CREATE UNIQUE INDEX IF NOT EXISTS packet_max_packet_source_uidx
ON public.packet_max (packet_id, source_id);
