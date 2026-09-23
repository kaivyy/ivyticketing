# IvyTicketing Multi-Sport Architecture Specification

## 1. Executive Summary & Core Philosophy

IvyTicketing is evolving from a running-focused ticketing and timing platform into a generic, capability-driven multi-sport competition engine. 

### Core Tenets
1. **Zero-Rewrite Evolution**: Reuse existing, proven modules (`organizations`, `orders`, `payments`, `inventory`, `queue`, `ballot`, `forms`, `audit_logs`).
2. **Backward Compatibility First**: Existing running events, historical results, tickets, and checkout workflows continue to function without data loss or breaking changes.
3. **Capability-Driven Modeling**: Rather than hardcoding separate rules for football, badminton, or cycling, the system models competitions using composable capability flags (`has_timing`, `has_scoring`, `has_matches`, `has_heats_lanes`, `has_stages`, `team_based`).
4. **Strict Federation Compliance**: Adhere to official international sports governing rules (World Athletics, FIFA/IFAB, BWF, World Aquatics, UCI), including timing rounding rules, gun-time precedence, and match scoring logic.
5. **No Blind Status Fallbacks**: Missing, unknown, or unparsed timing/result states must never silently default to `FINISHED`.

---

## 2. Capability-Driven Domain Hierarchy

The platform organizes athletic competition into an eight-tier abstraction:

```
Sport (e.g., Athletics, Football, Racket Sports, Aquatics, Cycling)
  └── Sport Discipline (e.g., Road Running, Futsal, Badminton Singles, Freestyle Swimming)
        └── Competition / Event (IvyTicketing Event)
              └── Competition Format (Single Race, Round Robin, Single Elimination, Multi-Stage)
                    └── Category / Division (e.g., Men Open 10K, Under-17 Boys, Division A)
                          └── Stage / Round / Heat / Match (e.g., Group Stage, Quarterfinal, Heat 2)
                                └── Participant Entry (Individual Athlete or Team Roster)
                                      └── Observations (Lap splits, points, goals, cards, chip reads)
                                            └── Results & Standings (Official ranks, times, points, GD)
```

### Capability Matrix Across Key Archetypes

| Capability Flag | Running (Athletics) | Football / Futsal | Badminton | Swimming (Aquatics) | Cycling (Road/Stage) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `has_timing` | Yes (Gun + Chip) | Match clock only | Match duration only | Yes (Electronic Pad) | Yes (Gun, Net, Split) |
| `has_scoring` | No (Rank by time) | Yes (Goals) | Yes (Rally points, Games)| No (Rank by time) | Points / KOM / Sprint |
| `has_matches` | No | Yes (1v1 fixtures) | Yes (1v1 or 2v2) | No | No (Group starts/stages) |
| `has_heats_lanes` | Optional (Track) | No | No (Court assigned)| Yes (Lanes 1 to 8/10) | No |
| `has_stages` | Rare (Multi-day) | Multi-round / Group | Tournament rounds | Heats, Semis, Finals| Multi-stage (Tour / GC)|
| `team_based` | Optional (Relay) | Yes (Squad & XI) | Singles or Doubles | Individual or Relay | Team & Individual |
| `has_seedings` | Elite wave seeding | Group pot seeding | Seeded draw brackets| Psych-sheet seeding | Rider GC / Team ranking |

---

## 3. Five Archetype Implementations

### Archetype 1: Running & Athletics (Time-Based)
- **Governing Body**: World Athletics (Technical Rules 19 & 55).
- **Primary Metric**: Elapsed time (milliseconds), ranked ascending.
- **Timing Priority**:
  - Gun Time (time from starter pistol to finish line) determines official placing and podium awards (TR 19.24.5 & TR 55.3).
  - Chip Time (net time from crossing start line to finish line) is provided for personal achievement and age-group rankings.
- **Precision & Rounding**:
  - Out of stadium / road races: whole-second ceiling rounding (`(ms + 999) / 1000`).
  - In-stadium track races: hundredth of a second ceiling rounding (`(ms + 9) / 10`).
- **Validation**:
  - Verification of mandatory intermediate split checkpoints to prevent course cutting.
  - Distinct statuses: `REGISTERED`, `STARTED`, `IN_PROGRESS`, `FINISHED`, `DNS`, `DNF`, `DSQ`, `OTL` (Over Time Limit).

### Archetype 2: Football & Futsal (Score & Match Fixtures)
- **Governing Body**: FIFA / IFAB.
- **Primary Metric**: Goals scored per fixture, aggregated into League / Group standings.
- **Standings Metric Hierarchy**:
  1. Points earned (Win = 3, Draw = 1, Loss = 0).
  2. Head-to-head points between tied teams.
  3. Head-to-head goal difference.
  4. Overall goal difference (GD = Goals For - Goals Against).
  5. Overall Goals For (GF).
  6. Fair Play disciplinary points (Yellow card = -1, Indirect Red = -3, Direct Red = -4).
- **Roster & Lineup Management**:
  - Team creation and roster registration linked to registration tickets.
  - Starting line-up, substitutes, and coach attribution.
  - In-match events: goals, assists, own goals, yellow cards, red cards, substitutions, penalty shootouts.

### Archetype 3: Badminton & Racket Sports (Sets & Tournament Brackets)
- **Governing Body**: Badminton World Federation (BWF).
- **Primary Metric**: Match won by best of 3 games (sets).
- **Scoring Rules**:
  - Rally point scoring: first to 21 points wins the game.
  - Deuce rule: at 20-20, side winning 2 consecutive points wins up to a cap of 30 (first to 30 wins at 29-29).
- **Tournament Formats**:
  - Single Elimination bracket (Knockout) with standard seeding (Seeds 1 & 2 placed at opposite ends).
  - Double Elimination bracket with repechage.
  - Round Robin group stage advancing to knockout stage.
- **Court Assignment**:
  - Real-time match queueing and court dispatching.

### Archetype 4: Swimming & Aquatics (Heats & Lane Seeding)
- **Governing Body**: World Aquatics (formerly FINA).
- **Primary Metric**: Time recorded to hundredths of a second (0.01s).
- **Heats & Lane Assignment Rules**:
  - Swimmers seeded according to verified entry qualifying times (psych sheets).
  - Spearhead / Zig-zag seeding: fastest swimmers placed in center lanes (Lane 4 in an 8-lane pool, Lane 5 in a 10-lane pool), alternating right and left.
- **Split Timing**:
  - Touchpad contact splits for each turn (50m / 100m).
  - Reaction time tracking from starting block sensors.
- **Relay Support**:
  - 4-leg swimmer rosters with individual takeover reaction times and leg split times.

### Archetype 5: Cycling (Road & Multi-Stage Tours)
- **Governing Body**: Union Cycliste Internationale (UCI).
- **Primary Metric**: General Classification (GC) accumulated time across stages.
- **Rules & Mechanics**:
  - Bunch finish / Peloton rule: riders crossing in the same continuous pack are awarded the same official finish time as the leading rider of the bunch.
  - Stage classifications: Stage Winner, Points / Sprinter (Green Jersey), King of the Mountains (Polka Dot), Best Young Rider (White Jersey).
  - Intermediate sprints and categorized climbs with point observation logs.
  - Time bonuses: 10s, 6s, 4s deducted from GC for podium stage finishes.

---

## 4. Multi-Sport Database Schema Extension

The database extension is additive and non-destructive. No existing tables are dropped. All existing running tables remain active, with a clean bridge to the new generic schema.

```
+----------------------------------------------------------------------------------------------------+
|                                    EXISTING CORE (REUSED AS-IS)                                    |
|  organizations  |  users  |  orders  |  payments  |  inventory  |  queue  |  ballot  |  audit_logs   |
+----------------------------------------------------------------------------------------------------+
                                                  |
                                                  v
+-----------------------------+         +-------------------------------+
|           events            | <-----> |            sports             |
| (extended with sport_id)    |         | (running, football, etc.)     |
+-----------------------------+         +-------------------------------+
       |               |                                |
       |               v                                v
       |        +-------------------+         +-------------------------+
       |        | event_categories  |         |    sport_disciplines    |
       |        | (division/race)   |         +-------------------------+
       |        +-------------------+                      |
       |                   |                               |
       v                   v                               v
+-----------------------------------------------------------------------+
|                         competition_configs                           |
|  (capability flags: has_timing, has_scoring, has_matches, rules_json) |
+-----------------------------------------------------------------------+
       |                                                   |
       v                                                   v
+-------------------------------+               +-----------------------+
|      competition_stages       |               |         teams         |
| (Group, Knockout, Heat, Tour) |               | (clubs, squads)       |
+-------------------------------+               +-----------------------+
       |                                                   |
       v                                                   v
+-------------------------------+               +-----------------------+
|      competition_matches      |               |     team_rosters      |
| (fixtures, courts, rounds)    |               | (player, jersey, pos) |
+-------------------------------+               +-----------------------+
       |                                                   |
       +-------------------------+-------------------------+
                                 |
                                 v
+-----------------------------------------------------------------------+
|                          participant_entries                          |
|   (polymorphic bridge: ticket_id, team_id, bib, seed, lane, heat)     |
+-----------------------------------------------------------------------+
       |                                                   |
       v                                                   v
+-------------------------------+               +-----------------------+
|   competition_observations    |               |  competition_results  |
| (points, goals, cards, splits)|               | (rank, time, pts, GD) |
+-------------------------------+               +-----------------------+
```

### Table Definitions

#### 1. `sports`
Base catalog of supported sports.
- `id`: VARCHAR(32) PRIMARY KEY (e.g., `'running'`, `'football'`, `'badminton'`, `'swimming'`, `'cycling'`)
- `name`: VARCHAR(64) NOT NULL
- `governing_body`: VARCHAR(64) NOT NULL (e.g., `'World Athletics'`, `'FIFA'`, `'BWF'`, `'World Aquatics'`, `'UCI'`)
- `created_at`: TIMESTAMPTZ DEFAULT NOW()

#### 2. `sport_disciplines`
Disciplines belonging to each sport.
- `id`: VARCHAR(64) PRIMARY KEY (e.g., `'road_running'`, `'trail_running'`, `'futsal'`, `'badminton_singles'`, `'badminton_doubles'`)
- `sport_id`: VARCHAR(32) REFERENCES sports(id)
- `name`: VARCHAR(128) NOT NULL
- `default_config`: JSONB NOT NULL DEFAULT '{}'

#### 3. `competition_configs`
Configures capabilities and scoring rules per event or category.
- `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
- `event_id`: UUID REFERENCES events(id) ON DELETE CASCADE
- `category_id`: UUID REFERENCES event_categories(id) ON DELETE CASCADE NULLABLE
- `sport_id`: VARCHAR(32) REFERENCES sports(id)
- `discipline_id`: VARCHAR(64) REFERENCES sport_disciplines(id)
- `has_timing`: BOOLEAN NOT NULL DEFAULT FALSE
- `has_scoring`: BOOLEAN NOT NULL DEFAULT FALSE
- `has_matches`: BOOLEAN NOT NULL DEFAULT FALSE
- `has_heats_lanes`: BOOLEAN NOT NULL DEFAULT FALSE
- `has_stages`: BOOLEAN NOT NULL DEFAULT FALSE
- `team_based`: BOOLEAN NOT NULL DEFAULT FALSE
- `min_team_size`: INT NOT NULL DEFAULT 1
- `max_team_size`: INT NOT NULL DEFAULT 1
- `rules_config`: JSONB NOT NULL DEFAULT '{}'

#### 4. `teams` & `team_rosters`
Enables squad and roster sports while preserving individual ticket sales.
- `teams`:
  - `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
  - `event_id`: UUID REFERENCES events(id) ON DELETE CASCADE
  - `name`: VARCHAR(128) NOT NULL
  - `short_name`: VARCHAR(16) NULLABLE
  - `manager_user_id`: UUID REFERENCES users(id) NULLABLE
  - `logo_url`: VARCHAR(512) NULLABLE
- `team_rosters`:
  - `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
  - `team_id`: UUID REFERENCES teams(id) ON DELETE CASCADE
  - `ticket_id`: UUID REFERENCES tickets(id) ON DELETE CASCADE NULLABLE
  - `user_id`: UUID REFERENCES users(id) NULLABLE
  - `player_name`: VARCHAR(128) NOT NULL
  - `jersey_number`: INT NULLABLE
  - `position`: VARCHAR(64) NULLABLE
  - `is_captain`: BOOLEAN NOT NULL DEFAULT FALSE

#### 5. `competition_stages`
Hierarchical stages of a tournament or event.
- `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
- `event_id`: UUID REFERENCES events(id) ON DELETE CASCADE
- `category_id`: UUID REFERENCES event_categories(id) ON DELETE CASCADE NULLABLE
- `name`: VARCHAR(128) NOT NULL (e.g., `'Group Stage'`, `'Quarterfinal'`, `'Heat 1'`, `'Stage 3 - Mountain'`)
- `stage_type`: VARCHAR(32) NOT NULL (`'single_race'`, `'group_round_robin'`, `'single_elimination'`, `'double_elimination'`, `'heats'`, `'peloton_stage'`)
- `sequence_order`: INT NOT NULL DEFAULT 1
- `status`: VARCHAR(32) NOT NULL DEFAULT `'SCHEDULED'` (`'SCHEDULED'`, `'LIVE'`, `'COMPLETED'`, `'ABANDONED'`)

#### 6. `competition_matches`
Fixtures between teams, athletes, or participants.
- `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
- `stage_id`: UUID REFERENCES competition_stages(id) ON DELETE CASCADE
- `home_team_id`: UUID REFERENCES teams(id) NULLABLE
- `away_team_id`: UUID REFERENCES teams(id) NULLABLE
- `home_participant_id`: UUID NULLABLE
- `away_participant_id`: UUID NULLABLE
- `scheduled_start_time`: TIMESTAMPTZ NULLABLE
- `venue_court_name`: VARCHAR(64) NULLABLE
- `home_score`: INT NOT NULL DEFAULT 0
- `away_score`: INT NOT NULL DEFAULT 0
- `match_status`: VARCHAR(32) NOT NULL DEFAULT `'SCHEDULED'`
- `round_number`: INT NOT NULL DEFAULT 1
- `match_metadata`: JSONB NOT NULL DEFAULT '{}'

#### 7. `participant_entries`
Universal participant identity bridging single tickets and teams to a competition.
- `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
- `event_id`: UUID REFERENCES events(id) ON DELETE CASCADE
- `category_id`: UUID REFERENCES event_categories(id) ON DELETE CASCADE
- `ticket_id`: UUID REFERENCES tickets(id) NULLABLE
- `team_id`: UUID REFERENCES teams(id) NULLABLE
- `display_name`: VARCHAR(128) NOT NULL
- `identifier_code`: VARCHAR(32) NULLABLE (Bib number, player license, or entry code)
- `seed_number`: INT NULLABLE
- `lane_number`: INT NULLABLE
- `heat_number`: INT NULLABLE

#### 8. `competition_observations`
Immutable log of all in-competition occurrences.
- `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
- `match_id`: UUID REFERENCES competition_matches(id) NULLABLE
- `stage_id`: UUID REFERENCES competition_stages(id) NULLABLE
- `entry_id`: UUID REFERENCES participant_entries(id) NULLABLE
- `observation_type`: VARCHAR(64) NOT NULL (`'goal'`, `'point'`, `'card'`, `'checkpoint_split'`, `'penalty'`, `'foul'`, `'set_finish'`)
- `numeric_value`: NUMERIC(12, 4) NULLABLE
- `text_value`: VARCHAR(255) NULLABLE
- `observed_at_ms`: BIGINT NOT NULL
- `created_at`: TIMESTAMPTZ DEFAULT NOW()

#### 9. `competition_results`
Universal standings, race results, and stage outcomes.
- `id`: UUID PRIMARY KEY DEFAULT gen_random_uuid()
- `stage_id`: UUID REFERENCES competition_stages(id) NULLABLE
- `entry_id`: UUID REFERENCES participant_entries(id) NOT NULL
- `rank_overall`: INT NULLABLE
- `rank_category`: INT NULLABLE
- `status`: VARCHAR(32) NOT NULL DEFAULT `'FINISHED'` (`'FINISHED'`, `'DNS'`, `'DNF'`, `'DSQ'`, `'OTL'`)
- `primary_time_ms`: BIGINT NULLABLE (Gun Time / Swim Finish Time)
- `secondary_time_ms`: BIGINT NULLABLE (Chip / Net Time)
- `points_scored`: NUMERIC(10, 2) NOT NULL DEFAULT 0
- `metrics`: JSONB NOT NULL DEFAULT '{}' (Contains sport-specific stats: GD, GF, sets won, laps)

---

## 5. Timing & Results Engine Correctness Audit

The audit of the existing timing system ([`services/api/internal/modules/results/`](file:///root/ivyticketing/services/api/internal/modules/results/)) revealed four critical deviations from international standards that must be corrected:

### Defect 1: Gun Time vs Net Time Ranking Precedence
- **Location**: [`database/queries/results.sql#L66-L77`](file:///root/ivyticketing/database/queries/results.sql#L66-L77)
- **Problem**: Queries use `ORDER BY COALESCE(chip_time_ms, gun_time_ms) ASC`.
- **Standard**: World Athletics Technical Rule 19.24.5 & Rule 55.3 mandates that official finish order and podium prizes are determined by Gun Time. Chip/Net time is informational.
- **Resolution**:
  ```sql
  ORDER BY 
    CASE WHEN status = 'FINISHED' THEN 0 ELSE 1 END,
    gun_time_ms ASC,
    chip_time_ms ASC
  ```

### Defect 2: Whole-Second Ceiling Rounding
- **Location**: [`services/api/internal/modules/results/model.go#L199-L208`](file:///root/ivyticketing/services/api/internal/modules/results/model.go#L199-L208)
- **Problem**: Time formatting truncates using integer division `ms / 1000`. A time of 3600100ms (1:00:00.100) displays as 1:00:00.
- **Standard**: World Athletics TR 19.24.5 mandates whole-second ceiling rounding for road races: `(ms + 999) / 1000`. 3600100ms must be rounded to 1:00:01.
- **Resolution**: Implement standard rounding helper adhering to discipline rules:
  ```go
  func RoundRoadRaceTime(ms int64) int64 {
      if ms <= 0 { return 0 }
      return ((ms + 999) / 1000) * 1000
  }
  ```

### Defect 3: Parser Dropping DSQ and OTL Statuses
- **Location**: [`services/api/internal/modules/results/parse.go#L94-L107`](file:///root/ivyticketing/services/api/internal/modules/results/parse.go#L94-L107)
- **Problem**: Migration 00063 allows `'DSQ'` and `'OTL'`, but `normalizeStatus` returns `""` when encountering them, silently causing the record to default to `'FINISHED'`.
- **Resolution**: Expand `normalizeStatus` to explicitly map:
  - `"DSQ"`, `"DISQUALIFIED"` -> `"DSQ"`
  - `"OTL"`, `"OVER TIME"`, `"OVER_CUTOFF"` -> `"OTL"`

### Defect 4: Missing Mandatory Checkpoint Verification
- **Location**: [`services/api/internal/modules/results/timing/processor.go#L30`](file:///root/ivyticketing/services/api/internal/modules/results/timing/processor.go#L30)
- **Problem**: `Policy` defines `MandatoryCheckpoints []string`, but the processing loop never validates split presence. A participant with only start and finish chip reads receives a valid `FINISHED` result despite missing checkpoints.
- **Resolution**: In `ValidateSplitSequence`, iterate over `MandatoryCheckpoints`. If any mandatory checkpoint is missing from the participant's observation reads, mark status as `DNF` or `DSQ` with an explanatory flag `MISSING_MANDATORY_SPLIT`.

---

## 6. Architecture Extension Interfaces

The Go backend introduces extensible interfaces in `services/api/internal/modules/competitions/`:

```go
package competitions

import "context"

// ScoringEngine evaluates match or stage observations into standing points.
type ScoringEngine interface {
    CalculateMatchScore(ctx context.Context, matchID string) (MatchOutcome, error)
    CalculateStageStandings(ctx context.Context, stageID string) ([]StandingEntry, error)
}

// RankingEngine ranks entries by time, points, or head-to-head tie-breakers.
type RankingEngine interface {
    RankStage(ctx context.Context, stageID string, config RulesConfig) ([]RankedResult, error)
}

// FixtureGenerator handles tournament bracket seeding and group generation.
type FixtureGenerator interface {
    GenerateRoundRobin(ctx context.Context, stageID string, teamIDs []string) ([]MatchFixture, error)
    GenerateSingleElimination(ctx context.Context, stageID string, seeds []ParticipantSeed) ([]MatchFixture, error)
}
```

---

## 7. Migration & Rollout Plan

To ensure continuous uptime and zero disruption to active events:

### Phase 1: Timing Integrity & Standards Fixes
- Fix Gun Time ranking precedence in `database/queries/results.sql`.
- Update `services/api/internal/modules/results/parse.go` and `model.go` for whole-second ceiling rounding and status preservation.
- Enforce mandatory checkpoint policy in timing processor.

### Phase 2: Multi-Sport Schema Migration
- Execute Goose migration `00066_create_generic_sports_schema.sql`.
- Seed foundational sports catalog: `running`, `football`, `badminton`, `swimming`, `cycling`.
- Add foreign key `sport_id` to `events` (nullable, defaults to `'running'`).

### Phase 3: Team Roster & Universal Entry Layer
- Implement `teams` and `team_rosters` services.
- Connect participant registration tickets to roster slots via dynamic forms.

### Phase 4: Match Fixtures & Scoring Service
- Implement `competition_matches`, `competition_stages`, and `competition_observations`.
- Build tournament bracket generation (Knockout and Round Robin).

### Phase 5: Adaptive Frontend & Public Results
- Expand public results page to dynamically render:
  - Time Leaderboards (Running, Swimming, Cycling).
  - League Standings Tables (Football, Futsal).
  - Bracket Trees & Match Scorecards (Badminton, Tennis).
- Update Organizer Event Creation flow to allow sport discipline selection.

---

## 8. Conclusion

This architecture preserves the entire existing IvyTicketing transactional and registration foundation while expanding its domain model to govern any modern competitive sport with international regulatory precision.
