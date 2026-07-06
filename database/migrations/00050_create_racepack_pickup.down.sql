DROP INDEX IF EXISTS idx_racepack_problem_cases_ticket;
DROP INDEX IF EXISTS idx_racepack_problem_cases_event_status;
DROP INDEX IF EXISTS idx_racepack_problem_cases_event;
DROP TABLE IF EXISTS racepack_problem_cases;

DROP INDEX IF EXISTS idx_racepack_proxy_authorizations_pickup;
DROP INDEX IF EXISTS idx_racepack_proxy_authorizations_ticket;
DROP INDEX IF EXISTS idx_racepack_proxy_authorizations_event;
DROP TABLE IF EXISTS racepack_proxy_authorizations;

DROP INDEX IF EXISTS idx_racepack_pickup_records_event_timestamp;
DROP INDEX IF EXISTS idx_racepack_pickup_records_participant;
DROP INDEX IF EXISTS idx_racepack_pickup_records_staff;
DROP INDEX IF EXISTS idx_racepack_pickup_records_counter;
DROP INDEX IF EXISTS idx_racepack_pickup_records_event;
DROP INDEX IF EXISTS uniq_racepack_pickup_records_ticket_active;
DROP TABLE IF EXISTS racepack_pickup_records;

DROP INDEX IF EXISTS idx_racepack_pickup_slots_event_active;
DROP INDEX IF EXISTS idx_racepack_pickup_slots_event_date;
DROP INDEX IF EXISTS idx_racepack_pickup_slots_event;
DROP TABLE IF EXISTS racepack_pickup_slots;

DROP INDEX IF EXISTS idx_racepack_counters_event_active;
DROP INDEX IF EXISTS idx_racepack_counters_event;
DROP TABLE IF EXISTS racepack_counters;