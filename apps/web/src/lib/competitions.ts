import { authedFetch } from "./api";
import { getApiBaseUrl } from "./auth";

export interface Sport {
  id: string;
  name: string;
  governingBody: string;
  createdAt: string;
}

export interface SportDiscipline {
  id: string;
  sportId: string;
  name: string;
  defaultConfig: Record<string, unknown>;
  createdAt: string;
}

export interface CompetitionConfig {
  id?: string;
  organizationId?: string;
  eventId: string;
  categoryId?: string;
  sportId: string;
  disciplineId: string;
  hasTiming: boolean;
  hasScoring: boolean;
  hasMatches: boolean;
  hasHeatsLanes: boolean;
  hasStages: boolean;
  teamBased: boolean;
  minTeamSize: number;
  maxTeamSize: number;
  rankingStrategy: string;
  timePrecision: string;
  rulesConfig?: Record<string, unknown>;
  createdAt?: string;
  updatedAt?: string;
}

export interface Team {
  id: string;
  organizationId: string;
  eventId: string;
  categoryId?: string;
  name: string;
  shortName?: string;
  managerUserId?: string;
  logoUrl?: string;
  seedNumber?: number;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export interface TeamRosterMember {
  id: string;
  teamId: string;
  ticketId?: string;
  userId?: string;
  playerName: string;
  jerseyNumber?: number;
  position?: string;
  isCaptain: boolean;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export interface CompetitionStage {
  id: string;
  organizationId: string;
  eventId: string;
  categoryId?: string;
  name: string;
  stageType: string;
  sequenceOrder: number;
  status: string;
  stageConfig?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface MatchScoreDetail {
  periodNumber: number;
  homeScore: number;
  awayScore: number;
}

export interface CompetitionMatch {
  id: string;
  organizationId: string;
  eventId: string;
  stageId: string;
  roundNumber: number;
  matchNumber: number;
  homeTeamId?: string;
  awayTeamId?: string;
  homeEntryId?: string;
  awayEntryId?: string;
  scheduledStartTime?: string;
  venueCourtName?: string;
  homeScore: number;
  awayScore: number;
  matchStatus: string;
  winnerId?: string;
  scoreDetails?: MatchScoreDetail[];
  createdAt: string;
  updatedAt: string;
}

export interface GenericStandingsRow {
  rank: number;
  entryId: string;
  displayName: string;
  identifierCode?: string;
  played: number;
  wins: number;
  draws: number;
  losses: number;
  goalsFor: number;
  goalsAgainst: number;
  goalDifference: number;
  points: number;
  primaryTimeMs?: number;
  primaryTime?: string;
  status: string;
  customMetrics?: Record<string, unknown>;
}

// Public API endpoints
export async function fetchSports(): Promise<Sport[]> {
  const base = getApiBaseUrl();
  const res = await fetch(`${base}/api/v1/sports`);
  if (!res.ok) throw new Error("Gagal memuat katalog olahraga");
  const data = (await res.json()) as { sports: Sport[] };
  return data.sports;
}

export async function fetchDisciplines(sportId: string): Promise<SportDiscipline[]> {
  const base = getApiBaseUrl();
  const res = await fetch(`${base}/api/v1/sports/${encodeURIComponent(sportId)}/disciplines`);
  if (!res.ok) throw new Error("Gagal memuat disiplin olahraga");
  const data = (await res.json()) as { disciplines: SportDiscipline[] };
  return data.disciplines;
}

export async function fetchCompetitionConfig(eventId: string): Promise<CompetitionConfig> {
  const base = getApiBaseUrl();
  const res = await fetch(`${base}/api/v1/events/${encodeURIComponent(eventId)}/competition`);
  if (!res.ok) throw new Error("Gagal memuat konfigurasi kompetisi");
  return (await res.json()) as CompetitionConfig;
}

export async function fetchTeams(eventId: string): Promise<Team[]> {
  const base = getApiBaseUrl();
  const res = await fetch(`${base}/api/v1/events/${encodeURIComponent(eventId)}/teams`);
  if (!res.ok) return [];
  const data = (await res.json()) as { teams: Team[] };
  return data.teams ?? [];
}

export async function fetchStages(eventId: string): Promise<CompetitionStage[]> {
  const base = getApiBaseUrl();
  const res = await fetch(`${base}/api/v1/events/${encodeURIComponent(eventId)}/stages`);
  if (!res.ok) return [];
  const data = (await res.json()) as { stages: CompetitionStage[] };
  return data.stages ?? [];
}

export async function fetchMatches(eventId: string, stageId: string): Promise<CompetitionMatch[]> {
  const base = getApiBaseUrl();
  const res = await fetch(`${base}/api/v1/events/${encodeURIComponent(eventId)}/stages/${encodeURIComponent(stageId)}/matches`);
  if (!res.ok) return [];
  const data = (await res.json()) as { matches: CompetitionMatch[] };
  return data.matches ?? [];
}

export async function fetchStandings(eventId: string, stageId: string): Promise<GenericStandingsRow[]> {
  const base = getApiBaseUrl();
  const res = await fetch(`${base}/api/v1/events/${encodeURIComponent(eventId)}/stages/${encodeURIComponent(stageId)}/standings`);
  if (!res.ok) return [];
  const data = (await res.json()) as { standings: GenericStandingsRow[] };
  return data.standings ?? [];
}

// Organizer management endpoints
export function saveCompetitionConfig(
  orgId: string,
  eventId: string,
  cfg: Partial<CompetitionConfig>
): Promise<CompetitionConfig> {
  return authedFetch<CompetitionConfig>(
    `/organizations/${orgId}/events/${eventId}/competition/config`,
    {
      method: "PUT",
      body: JSON.stringify(cfg),
    }
  );
}

export function createTeam(
  orgId: string,
  eventId: string,
  data: {
    name: string;
    shortName?: string;
    categoryId?: string;
    logoUrl?: string;
    seedNumber?: number;
  }
): Promise<Team> {
  return authedFetch<Team>(
    `/organizations/${orgId}/events/${eventId}/competition/teams`,
    {
      method: "POST",
      body: JSON.stringify(data),
    }
  );
}

export function createStage(
  orgId: string,
  eventId: string,
  data: {
    name: string;
    stageType: string;
    sequenceOrder: number;
    categoryId?: string;
    autoGenerateFixtures?: boolean;
    entryIds?: string[];
  }
): Promise<{ stage: CompetitionStage; matches: CompetitionMatch[] }> {
  return authedFetch<{ stage: CompetitionStage; matches: CompetitionMatch[] }>(
    `/organizations/${orgId}/events/${eventId}/competition/stages`,
    {
      method: "POST",
      body: JSON.stringify(data),
    }
  );
}

export function recordMatchScore(
  orgId: string,
  eventId: string,
  matchId: string,
  data: {
    homeScore: number;
    awayScore: number;
    status: string;
    winnerId?: string;
    details?: MatchScoreDetail[];
  }
): Promise<CompetitionMatch> {
  return authedFetch<CompetitionMatch>(
    `/organizations/${orgId}/events/${eventId}/competition/matches/${matchId}/score`,
    {
      method: "POST",
      body: JSON.stringify(data),
    }
  );
}
