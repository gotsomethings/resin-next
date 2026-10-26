-- The manual egress-IP allow/deny list was removed as a feature (its UI was
-- dropped earlier). Clear the stale values so they cannot silently keep
-- filtering platforms (an allow list of one IP empties the platform).
UPDATE platforms
SET egress_ip_filters_json = '[]'
WHERE egress_ip_filters_json IS NOT NULL
  AND egress_ip_filters_json <> '[]';
