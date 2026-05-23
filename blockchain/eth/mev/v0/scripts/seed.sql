-- Sample data
INSERT INTO block_builds (block_hash, included, inclusion_reason, inclusion_latency_ms)
VALUES 
('0xabc123', TRUE, 'Highest profit', 250),
('0xdef456', TRUE, 'No conflict', 310);

INSERT INTO bundles (bundle_id, searcher, tx_count, target_block)
VALUES
('bundle-001', 'searcher-alpha', 2, '0x12A3B4'),
('bundle-002', 'searcher-beta', 3, '0x12A3B5');

INSERT INTO simulations (bundle_id, profit_eth, latency_ms, success, reason)
VALUES
('bundle-001', 0.0043, 512, TRUE, ''),
('bundle-002', 0.0021, 634, TRUE, '');
