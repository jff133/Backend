package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"Backend/internal/domain"
	"Backend/internal/repository"
	"Backend/internal/service"
)

type MockTeamRepo struct {
	GetUserByIDFn        func(ctx context.Context, userID string) (domain.User, error)
	GetTeamByNameFn      func(ctx context.Context, teamName string) (domain.Team, error)
	CreateOrUpdateTeamFn func(ctx context.Context, team domain.Team) (domain.Team, error)
	SetUserIsActiveFn    func(ctx context.Context, userID string, isActive bool) (domain.User, error)
}

func (m *MockTeamRepo) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	return m.GetUserByIDFn(ctx, userID)
}
func (m *MockTeamRepo) GetTeamByName(ctx context.Context, teamName string) (domain.Team, error) {
	return m.GetTeamByNameFn(ctx, teamName)
}
func (m *MockTeamRepo) CreateOrUpdateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	return m.CreateOrUpdateTeamFn(ctx, team)
}
func (m *MockTeamRepo) SetUserIsActive(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	return m.SetUserIsActiveFn(ctx, userID, isActive)
}

type MockPRRepo struct {
	CreatePullRequestFn  func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error)
	GetPullRequestByIDFn func(ctx context.Context, prID string) (domain.PullRequest, error)
	UpdatePullRequestFn  func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error)
	GetPRsByReviewerIDFn func(ctx context.Context, userID string) ([]domain.PullRequestShort, error)
}

func (m *MockPRRepo) CreatePullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
	return m.CreatePullRequestFn(ctx, pr)
}
func (m *MockPRRepo) GetPullRequestByID(ctx context.Context, prID string) (domain.PullRequest, error) {
	return m.GetPullRequestByIDFn(ctx, prID)
}
func (m *MockPRRepo) UpdatePullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
	return m.UpdatePullRequestFn(ctx, pr)
}
func (m *MockPRRepo) GetPRsByReviewerID(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	return m.GetPRsByReviewerIDFn(ctx, userID)
}

var _ repository.TeamRepository = (*MockTeamRepo)(nil)
var _ repository.PullRequestRepository = (*MockPRRepo)(nil)

func newMockTeamRepo() *MockTeamRepo {
	return &MockTeamRepo{
		GetUserByIDFn: func(ctx context.Context, userID string) (domain.User, error) {
			return domain.User{}, errors.New("default not implemented")
		},
		GetTeamByNameFn: func(ctx context.Context, teamName string) (domain.Team, error) {
			return domain.Team{}, errors.New("default not implemented")
		},
		CreateOrUpdateTeamFn: func(ctx context.Context, team domain.Team) (domain.Team, error) {
			return team, nil
		},
		SetUserIsActiveFn: func(ctx context.Context, userID string, isActive bool) (domain.User, error) {
			return domain.User{UserID: userID, IsActive: isActive}, nil
		},
	}
}

func newMockPRRepo() *MockPRRepo {
	return &MockPRRepo{
		CreatePullRequestFn: func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
			return pr, nil
		},
		GetPullRequestByIDFn: func(ctx context.Context, prID string) (domain.PullRequest, error) {
			return domain.PullRequest{}, nil
		},
		UpdatePullRequestFn: func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
			return pr, nil
		},
		GetPRsByReviewerIDFn: func(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
			return nil, nil
		},
	}
}

func stringSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func TestCreateAndAssignReviewers_Success(t *testing.T) {
	ctx := context.Background()
	authorID := "u1"
	teamName := "backend-team"

	author := domain.User{UserID: authorID, TeamName: teamName, IsActive: true}
	team := domain.Team{
		TeamName: teamName,
		Members: []domain.User{
			author,
			{UserID: "u2", TeamName: teamName, IsActive: true},  // Candidate 1
			{UserID: "u3", TeamName: teamName, IsActive: true},  // Candidate 2
			{UserID: "u4", TeamName: teamName, IsActive: false}, // Inactive (должен быть исключен)
		},
	}

	mockTeamRepo := newMockTeamRepo()
	mockTeamRepo.GetUserByIDFn = func(ctx context.Context, userID string) (domain.User, error) { return author, nil }
	mockTeamRepo.GetTeamByNameFn = func(ctx context.Context, teamName string) (domain.Team, error) { return team, nil }

	mockPRRepo := newMockPRRepo()
	mockPRRepo.CreatePullRequestFn = func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
		if len(pr.AssignedReviewers) != 2 {
			t.Fatalf("Expected 2 reviewers, got %d", len(pr.AssignedReviewers))
		}
		if stringSliceContains(pr.AssignedReviewers, authorID) {
			t.Fatalf("Author u1 was assigned as reviewer")
		}
		if stringSliceContains(pr.AssignedReviewers, "u4") {
			t.Fatalf("Inactive user u4 was assigned as reviewer")
		}
		return pr, nil
	}

	prService := service.NewPRService(mockPRRepo, mockTeamRepo)

	_, err := prService.CreateAndAssignReviewers(ctx, "pr-1", "Test PR", authorID)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestCreateAndAssignReviewers_NotEnoughCandidates(t *testing.T) {
	ctx := context.Background()
	authorID := "u1"
	teamName := "small-team"

	author := domain.User{UserID: authorID, TeamName: teamName, IsActive: true}
	team := domain.Team{
		TeamName: teamName,
		Members: []domain.User{
			author,
			{UserID: "u2", TeamName: teamName, IsActive: true},
			{UserID: "u3", TeamName: teamName, IsActive: false},
		},
	}

	mockTeamRepo := newMockTeamRepo()
	mockTeamRepo.GetUserByIDFn = func(ctx context.Context, userID string) (domain.User, error) { return author, nil }
	mockTeamRepo.GetTeamByNameFn = func(ctx context.Context, teamName string) (domain.Team, error) { return team, nil }

	mockPRRepo := newMockPRRepo()
	mockPRRepo.CreatePullRequestFn = func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
		if len(pr.AssignedReviewers) != 1 {
			t.Fatalf("Expected 1 reviewer, got %d", len(pr.AssignedReviewers))
		}
		if pr.AssignedReviewers[0] != "u2" {
			t.Fatalf("Expected reviewer u2, got %s", pr.AssignedReviewers[0])
		}
		return pr, nil
	}

	prService := service.NewPRService(mockPRRepo, mockTeamRepo)

	_, err := prService.CreateAndAssignReviewers(ctx, "pr-2", "Small Team PR", authorID)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestMergePullRequest_Idempotent(t *testing.T) {
	ctx := context.Background()
	prID := "pr-merged"

	alreadyMergedPR := domain.PullRequest{
		PullRequestID: prID,
		Status:        domain.StatusMerged,
		MergedAt:      func() *time.Time { t := time.Now(); return &t }(),
	}

	mockPRRepo := newMockPRRepo()
	mockPRRepo.GetPullRequestByIDFn = func(ctx context.Context, id string) (domain.PullRequest, error) {
		return alreadyMergedPR, nil
	}
	mockPRRepo.UpdatePullRequestFn = func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
		t.Fatal("UpdatePullRequest should NOT be called on an already merged PR")
		return pr, nil
	}

	prService := service.NewPRService(mockPRRepo, newMockTeamRepo())

	_, err := prService.MergePullRequest(ctx, prID)

	if err != nil {
		t.Errorf("Expected no error (idempotent), got %v", err)
	}
}

func TestReassignReviewer_Success(t *testing.T) {
	ctx := context.Background()
	prID := "pr-reassign"
	oldUserID := "u2"
	newCandidateID := "u5"

	openPR := domain.PullRequest{
		PullRequestID:     prID,
		AuthorID:          "u1",
		Status:            domain.StatusOpen,
		AssignedReviewers: []string{oldUserID, "u3"},
	}

	teamName := "backend-team"
	oldUser := domain.User{UserID: oldUserID, TeamName: teamName, IsActive: true}
	team := domain.Team{
		TeamName: teamName,
		Members: []domain.User{
			oldUser,
			{UserID: "u1", TeamName: teamName, IsActive: true},
			{UserID: "u3", TeamName: teamName, IsActive: true},
			{UserID: newCandidateID, TeamName: teamName, IsActive: true},
		},
	}

	mockTeamRepo := newMockTeamRepo()
	mockTeamRepo.GetUserByIDFn = func(ctx context.Context, userID string) (domain.User, error) { return oldUser, nil }
	mockTeamRepo.GetTeamByNameFn = func(ctx context.Context, teamName string) (domain.Team, error) { return team, nil }

	mockPRRepo := newMockPRRepo()
	mockPRRepo.GetPullRequestByIDFn = func(ctx context.Context, id string) (domain.PullRequest, error) {
		return openPR, nil
	}
	mockPRRepo.UpdatePullRequestFn = func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
		if !stringSliceContains(pr.AssignedReviewers, newCandidateID) || stringSliceContains(pr.AssignedReviewers, oldUserID) {
			t.Fatalf("Reassignment failed: Expected %s assigned, %s removed. Got %v", newCandidateID, oldUserID, pr.AssignedReviewers)
		}
		return pr, nil
	}

	prService := service.NewPRService(mockPRRepo, mockTeamRepo)

	updatedPR, newUserID, err := prService.ReassignReviewer(ctx, prID, oldUserID)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if newUserID != newCandidateID {
		t.Errorf("Expected new user ID '%s', got %s", newCandidateID, newUserID)
	}
	if !stringSliceContains(updatedPR.AssignedReviewers, newCandidateID) || stringSliceContains(updatedPR.AssignedReviewers, oldUserID) {
		t.Errorf("Updated PR does not contain the new reviewer or still contains the old one")
	}
}

func TestReassignReviewer_PRMerged(t *testing.T) {
	ctx := context.Background()
	prID := "pr-merged-err"

	mergedPR := domain.PullRequest{
		PullRequestID: prID,
		Status:        domain.StatusMerged,
	}

	mockPRRepo := newMockPRRepo()
	mockPRRepo.GetPullRequestByIDFn = func(ctx context.Context, id string) (domain.PullRequest, error) {
		return mergedPR, nil
	}
	mockPRRepo.UpdatePullRequestFn = func(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
		t.Fatal("UpdatePullRequest should NOT be called")
		return pr, nil
	}

	prService := service.NewPRService(mockPRRepo, newMockTeamRepo())

	_, _, err := prService.ReassignReviewer(ctx, prID, "u2")

	var businessErr *domain.BusinessError
	if !errors.As(err, &businessErr) || businessErr.Code != domain.ErrPRMerged {
		t.Errorf("Expected error code %s, got %v", domain.ErrPRMerged, err)
	}
}

func TestUserService_GetReviewPRsByUserID_Success(t *testing.T) {
	ctx := context.Background()
	userID := "reviewer-id"

	mockPRs := []domain.PullRequestShort{
		{PullRequestID: "pr-1", Status: domain.StatusOpen},
		{PullRequestID: "pr-2", Status: domain.StatusOpen},
	}

	mockTeamRepo := newMockTeamRepo()
	mockTeamRepo.GetUserByIDFn = func(ctx context.Context, id string) (domain.User, error) {
		return domain.User{UserID: userID}, nil
	}

	mockPRRepo := newMockPRRepo()
	mockPRRepo.GetPRsByReviewerIDFn = func(ctx context.Context, id string) ([]domain.PullRequestShort, error) {
		if id == userID {
			return mockPRs, nil
		}
		return nil, nil
	}

	userService := service.NewUserService(mockTeamRepo, mockPRRepo)

	prs, err := userService.GetReviewPRsByUserID(ctx, userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(prs) != 2 {
		t.Errorf("Expected 2 PRs, got %d", len(prs))
	}
}

func TestUserService_GetReviewPRsByUserID_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userID := "unknown-id"

	mockTeamRepo := newMockTeamRepo()
	mockTeamRepo.GetUserByIDFn = func(ctx context.Context, id string) (domain.User, error) {
		return domain.User{}, &domain.BusinessError{Code: domain.ErrNotFound}
	}

	userService := service.NewUserService(mockTeamRepo, newMockPRRepo())

	_, err := userService.GetReviewPRsByUserID(ctx, userID)

	var businessErr *domain.BusinessError
	if !errors.As(err, &businessErr) || businessErr.Code != domain.ErrNotFound {
		t.Errorf("Expected error code %s, got %v", domain.ErrNotFound, err)
	}
}
