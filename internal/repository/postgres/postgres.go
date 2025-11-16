package postgres

import (
	"Backend/internal/domain"
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Init(ctx context.Context) error {
	const createSchemas = `
	CREATE TABLE IF NOT EXISTS teams (
		team_name TEXT PRIMARY KEY
	);
	
	CREATE TABLE IF NOT EXISTS users (
		user_id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		team_name TEXT NOT NULL REFERENCES teams(team_name) ON UPDATE CASCADE ON DELETE RESTRICT,
		is_active BOOLEAN NOT NULL
	);

	CREATE TABLE IF NOT EXISTS pull_requests (
		pr_id TEXT PRIMARY KEY,
		pr_name TEXT NOT NULL,
		author_id TEXT NOT NULL REFERENCES users(user_id) ON UPDATE CASCADE ON DELETE RESTRICT,
		status TEXT NOT NULL,
		assigned_reviewers TEXT[] NOT NULL,
		created_at TIMESTAMPTZ NOT NULL,
		merged_at TIMESTAMPTZ
	);
	CREATE INDEX IF NOT EXISTS idx_pr_reviewer ON pull_requests USING GIN (assigned_reviewers);
	`

	_, err := r.db.ExecContext(ctx, createSchemas)
	if err != nil {
		return fmt.Errorf("failed to initialize database schema: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreateOrUpdateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Team{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"INSERT INTO teams (team_name) VALUES ($1) ON CONFLICT (team_name) DO NOTHING",
		team.TeamName)
	if err != nil {
		return domain.Team{}, fmt.Errorf("failed to upsert team: %w", err)
	}

	for _, member := range team.Members {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO users (user_id, username, team_name, is_active) 
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id) DO UPDATE 
			 SET username = EXCLUDED.username, 
			     team_name = EXCLUDED.team_name, 
			     is_active = EXCLUDED.is_active`,
			member.UserID, member.Username, member.TeamName, member.IsActive)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
				return domain.Team{}, domain.NewBusinessError(domain.ErrNotFound, fmt.Sprintf("Team %s not found for user %s", member.TeamName, member.UserID))
			}
			return domain.Team{}, fmt.Errorf("failed to upsert user %s: %w", member.UserID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return domain.Team{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return team, nil
}

func (r *PostgresRepository) GetTeamByName(ctx context.Context, teamName string) (domain.Team, error) {
	var tName string
	err := r.db.QueryRowContext(ctx, "SELECT team_name FROM teams WHERE team_name = $1", teamName).Scan(&tName)
	if err == sql.ErrNoRows {
		return domain.Team{}, domain.NewBusinessError(domain.ErrNotFound, fmt.Sprintf("Team %s not found", teamName))
	}
	if err != nil {
		return domain.Team{}, fmt.Errorf("error querying team: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT user_id, username, team_name, is_active FROM users WHERE team_name = $1", teamName)
	if err != nil {
		return domain.Team{}, fmt.Errorf("error querying team members: %w", err)
	}
	defer rows.Close()

	team := domain.Team{TeamName: tName}
	for rows.Next() {
		var member domain.User
		if err := rows.Scan(&member.UserID, &member.Username, &member.TeamName, &member.IsActive); err != nil {
			return domain.Team{}, fmt.Errorf("error scanning team member: %w", err)
		}
		team.Members = append(team.Members, member)
	}

	if err := rows.Err(); err != nil {
		return domain.Team{}, fmt.Errorf("error iterating team members: %w", err)
	}

	return team, nil
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	var u domain.User
	row := r.db.QueryRowContext(ctx,
		"SELECT user_id, username, team_name, is_active FROM users WHERE user_id = $1", userID)

	err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive)

	if err == sql.ErrNoRows {
		return domain.User{}, domain.NewBusinessError(domain.ErrNotFound, fmt.Sprintf("User %s not found", userID))
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("error getting user from DB: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) SetUserIsActive(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	result, err := r.db.ExecContext(ctx,
		"UPDATE users SET is_active = $2 WHERE user_id = $1", userID, isActive)
	if err != nil {
		return domain.User{}, fmt.Errorf("error updating user activity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return domain.User{}, fmt.Errorf("error getting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.User{}, domain.NewBusinessError(domain.ErrNotFound, fmt.Sprintf("User %s not found for update", userID))
	}

	return r.GetUserByID(ctx, userID)
}

func (r *PostgresRepository) CreatePullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
	assignedReviewers := pq.Array(pr.AssignedReviewers)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pull_requests (pr_id, pr_name, author_id, status, assigned_reviewers, created_at, merged_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, assignedReviewers, pr.CreatedAt, pr.MergedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return domain.PullRequest{}, domain.NewBusinessError(domain.ErrPRExists, fmt.Sprintf("Pull Request with ID %s already exists", pr.PullRequestID))
		}
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return domain.PullRequest{}, domain.NewBusinessError(domain.ErrNotFound, fmt.Sprintf("Author %s not found", pr.AuthorID))
		}
		return domain.PullRequest{}, fmt.Errorf("failed to create PR: %w", err)
	}

	return pr, nil
}

func (r *PostgresRepository) GetPullRequestByID(ctx context.Context, prID string) (domain.PullRequest, error) {
	var pr domain.PullRequest
	var assignedReviewers pq.StringArray
	row := r.db.QueryRowContext(ctx,
		`SELECT pr_id, pr_name, author_id, status, assigned_reviewers, created_at, merged_at 
		 FROM pull_requests 
		 WHERE pr_id = $1`, prID)

	err := row.Scan(
		&pr.PullRequestID,
		&pr.PullRequestName,
		&pr.AuthorID,
		&pr.Status,
		&assignedReviewers,
		&pr.CreatedAt,
		&pr.MergedAt)

	if err == sql.ErrNoRows {
		return domain.PullRequest{}, domain.NewBusinessError(domain.ErrNotFound, fmt.Sprintf("Pull Request %s not found", prID))
	}
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("error getting PR from DB: %w", err)
	}

	pr.AssignedReviewers = []string(assignedReviewers)
	return pr, nil
}

func (r *PostgresRepository) UpdatePullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
	var mergedAt *time.Time
	if pr.MergedAt != nil {
		mergedAt = pr.MergedAt
	}

	assignedReviewers := pq.Array(pr.AssignedReviewers)

	result, err := r.db.ExecContext(ctx,
		`UPDATE pull_requests 
		 SET pr_name = $2, author_id = $3, status = $4, assigned_reviewers = $5, merged_at = $6
		 WHERE pr_id = $1`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, assignedReviewers, mergedAt)

	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("error updating PR: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("error getting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.PullRequest{}, domain.NewBusinessError(domain.ErrNotFound, "Pull Request not found for update")
	}

	return pr, nil
}

func (r *PostgresRepository) GetPRsByReviewerID(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT pr_id, pr_name, author_id, status 
		 FROM pull_requests 
		 WHERE $1 = ANY(assigned_reviewers) 
		 ORDER BY created_at DESC`, userID)

	if err != nil {
		return nil, fmt.Errorf("error querying PRs by reviewer: %w", err)
	}
	defer rows.Close()

	var prs []domain.PullRequestShort
	for rows.Next() {
		var pr domain.PullRequestShort
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status); err != nil {
			return nil, fmt.Errorf("error scanning PR short: %w", err)
		}
		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating PRs: %w", err)
	}

	return prs, nil
}
