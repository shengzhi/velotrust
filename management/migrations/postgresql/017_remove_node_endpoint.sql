-- The node table already stores the current control, public, and private
-- endpoint values. The separate candidate-endpoint table is unused by the
-- application and duplicates the current endpoint model.
-- Keep this migration additive to the migration history; do not edit 011/012.

DROP TABLE public.t_ztna_node_endpoint;
