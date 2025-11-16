package service

import (
	"context"
	"math/rand"
	"time"

	"Backend/internal/domain"
	"Backend/internal/repository"
)

type PRService interface {
	CreateAndAssignReviewers(ctx context.Context, prID, prName, authorID string) (domain.PullRequest, error)
	MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error)
	ReassignReviewer(ctx context.Context, prID, oldUserID string) (domain.PullRequest, string, error)
}

type TeamService interface {
	CreateOrUpdateTeam(ctx context.Context, team domain.Team) (domain.Team, error)
	GetTeamByName(ctx context.Context, teamName string) (domain.Team, error)
}

type UserService interface {
	SetUserIsActive(ctx context.Context, userID string, isActive bool) (domain.User, error)
	GetReviewPRsByUserID(ctx context.Context, userID string) ([]domain.PullRequestShort, error)
}

type PRServiceImpl struct {
	prRepo   repository.PullRequestRepository
	teamRepo repository.TeamRepository
	random   *rand.Rand
}

func NewPRService(prRepo repository.PullRequestRepository, teamRepo repository.TeamRepository) PRService {
	return &PRServiceImpl{
		prRepo:   prRepo,
		teamRepo: teamRepo,
		random:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *PRServiceImpl) selectRandomReviewers(candidates []string, count int) []string {
	if len(candidates) == 0 {
		return []string{}
	}

	s.random.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	numToSelect := count
	if len(candidates) < count {
		numToSelect = len(candidates)
	}

	return candidates[:numToSelect]
}

func (s *PRServiceImpl) CreateAndAssignReviewers(ctx context.Context, prID, prName, authorID string) (domain.PullRequest, error) {
	author, err := s.teamRepo.GetUserByID(ctx, authorID)
	if err != nil {
		return domain.PullRequest{}, err
	}

	team, err := s.teamRepo.GetTeamByName(ctx, author.TeamName)
	if err != nil {
		return domain.PullRequest{}, err
	}

	var candidates []string
	for _, member := range team.Members {
		if member.IsActive && member.UserID != authorID {
			candidates = append(candidates, member.UserID)
		}
	}

	reviewers := s.selectRandomReviewers(candidates, 2)

	now := time.Now().UTC()
	newPR := domain.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            domain.StatusOpen,
		AssignedReviewers: reviewers,
		CreatedAt:         &now,
	}

	return s.prRepo.CreatePullRequest(ctx, newPR)
}

func (s *PRServiceImpl) MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	pr, err := s.prRepo.GetPullRequestByID(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, err
	}

	if pr.Status == domain.StatusMerged {
		return pr, nil
	}

	pr.Status = domain.StatusMerged
	now := time.Now().UTC()
	pr.MergedAt = &now

	return s.prRepo.UpdatePullRequest(ctx, pr)
}

func (s *PRServiceImpl) ReassignReviewer(ctx context.Context, prID, oldUserID string) (domain.PullRequest, string, error) {
	pr, err := s.prRepo.GetPullRequestByID(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	if pr.Status == domain.StatusMerged {
		return domain.PullRequest{}, "", &domain.BusinessError{
			Code:    domain.ErrPRMerged,
			Message: "cannot reassign on merged PR",
		}
	}

	oldReviewerIndex := -1
	for i, assignedID := range pr.AssignedReviewers {
		if assignedID == oldUserID {
			oldReviewerIndex = i
			break
		}
	}
	if oldReviewerIndex == -1 {
		return domain.PullRequest{}, "", &domain.BusinessError{
			Code:    domain.ErrNotAssigned,
			Message: "reviewer is not assigned to this PR",
		}
	}

	oldReviewer, err := s.teamRepo.GetUserByID(ctx, oldUserID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}
	team, err := s.teamRepo.GetTeamByName(ctx, oldReviewer.TeamName)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	assignedSet := make(map[string]bool)
	for _, id := range pr.AssignedReviewers {
		assignedSet[id] = true
	}
	assignedSet[pr.AuthorID] = true

	var candidates []string
	for _, member := range team.Members {
		if member.IsActive && !assignedSet[member.UserID] {
			candidates = append(candidates, member.UserID)
		}
	}

	if len(candidates) == 0 {
		return domain.PullRequest{}, "", &domain.BusinessError{
			Code:    domain.ErrNoCandidate,
			Message: "no active replacement candidate in team",
		}
	}

	newUserID := s.selectRandomReviewers(candidates, 1)[0]

	pr.AssignedReviewers[oldReviewerIndex] = newUserID

	updatedPR, err := s.prRepo.UpdatePullRequest(ctx, pr)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	return updatedPR, newUserID, nil
}

type TeamServiceImpl struct {
	teamRepo repository.TeamRepository
}

func NewTeamService(teamRepo repository.TeamRepository) TeamService {
	return &TeamServiceImpl{teamRepo: teamRepo}
}

func (s *TeamServiceImpl) CreateOrUpdateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	return s.teamRepo.CreateOrUpdateTeam(ctx, team)
}

func (s *TeamServiceImpl) GetTeamByName(ctx context.Context, teamName string) (domain.Team, error) {
	return s.teamRepo.GetTeamByName(ctx, teamName)
}

type UserServiceImpl struct {
	teamRepo repository.TeamRepository
	prRepo   repository.PullRequestRepository
}

func NewUserService(teamRepo repository.TeamRepository, prRepo repository.PullRequestRepository) UserService {
	return &UserServiceImpl{teamRepo: teamRepo, prRepo: prRepo}
}

func (s *UserServiceImpl) SetUserIsActive(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	return s.teamRepo.SetUserIsActive(ctx, userID, isActive)
}

func (s *UserServiceImpl) GetReviewPRsByUserID(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	if _, err := s.teamRepo.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}

	return s.prRepo.GetPRsByReviewerID(ctx, userID)
}
