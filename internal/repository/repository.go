package repository

import (
	"Backend/internal/domain"
	"context"
)

type TeamRepository interface {
	CreateOrUpdateTeam(ctx context.Context, team domain.Team) (domain.Team, error)
	GetTeamByName(ctx context.Context, teamName string) (domain.Team, error)
	GetUserByID(ctx context.Context, userID string) (domain.User, error)
	SetUserIsActive(ctx context.Context, userID string, isActive bool) (domain.User, error)
}

type PullRequestRepository interface {
	CreatePullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error)
	GetPullRequestByID(ctx context.Context, prID string) (domain.PullRequest, error)
	UpdatePullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error)
	GetPRsByReviewerID(ctx context.Context, userID string) ([]domain.PullRequestShort, error)
}
