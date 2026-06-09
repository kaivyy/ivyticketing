-- +goose Up
CREATE TABLE ballot_draw_results (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id          uuid NOT NULL REFERENCES ballot_draws(id) ON DELETE CASCADE,
    ballot_entry_id  uuid NOT NULL REFERENCES ballot_entries(id),
    outcome          text NOT NULL CHECK (outcome IN ('WINNER','WAITLISTED','NOT_SELECTED')),
    rank             integer NOT NULL,
    result_hash      text NOT NULL,
    UNIQUE (draw_id, ballot_entry_id)
);
CREATE INDEX ballot_draw_results_rank_idx ON ballot_draw_results(draw_id, outcome, rank);

-- +goose Down
DROP TABLE ballot_draw_results;
