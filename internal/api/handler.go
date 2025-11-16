package api

import (
	"Backend/internal/domain"
	"Backend/internal/service"
	"context"
	"encoding/json"
	"net/http"
)

func sendJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func handleServiceError(w http.ResponseWriter, err error) {
	if bErr, ok := err.(*domain.BusinessError); ok {
		var status int
		switch bErr.Code {
		case domain.ErrNotFound:
			status = http.StatusNotFound // 404
		case domain.ErrPRExists, domain.ErrPRMerged, domain.ErrNotAssigned, domain.ErrNoCandidate, domain.ErrTeamExists:
			status = http.StatusConflict // 409
		default:
			status = http.StatusBadRequest // 400
		}

		resp := domain.ErrorResponse{
			Error: struct {
				Code    domain.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{Code: bErr.Code, Message: bErr.Message},
		}
		sendJSONResponse(w, status, resp)
		return
	}
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

type PRHandler struct{ prService service.PRService }

func NewPRHandler(prService service.PRService) *PRHandler { return &PRHandler{prService: prService} }

func (h *PRHandler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var reqBody PullRequestCreateRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	pr, err := h.prService.CreateAndAssignReviewers(context.Background(), reqBody.PullRequestID, reqBody.PullRequestName, reqBody.AuthorID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	sendJSONResponse(w, http.StatusCreated, pr)
}

func (h *PRHandler) MergePR(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		PullRequestID string `json:"pull_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	pr, err := h.prService.MergePullRequest(context.Background(), reqBody.PullRequestID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	sendJSONResponse(w, http.StatusOK, pr)
}

func (h *PRHandler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var reqBody PullRequestReassignRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	pr, newReviewerID, err := h.prService.ReassignReviewer(context.Background(), reqBody.PullRequestID, reqBody.OldUserID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewerID,
	})
}

type TeamHandler struct{ teamService service.TeamService }

func NewTeamHandler(teamService service.TeamService) *TeamHandler {
	return &TeamHandler{teamService: teamService}
}

func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var reqBody TeamRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var domainUsers []domain.User
	for _, memberDTO := range reqBody.Members {
		domainUsers = append(domainUsers, domain.User{
			UserID:   memberDTO.UserID,
			Username: memberDTO.Username,
			TeamName: reqBody.TeamName,
			IsActive: memberDTO.IsActive,
		})
	}

	domainTeam := domain.Team{
		TeamName: reqBody.TeamName,
		Members:  domainUsers,
	}

	team, err := h.teamService.CreateOrUpdateTeam(context.Background(), domainTeam)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	var memberDTOs []TeamMemberDTO
	for _, member := range team.Members {
		memberDTOs = append(memberDTOs, TeamMemberDTO{
			UserID:   member.UserID,
			Username: member.Username,
			IsActive: member.IsActive,
		})
	}
	response := TeamResponseDTO{
		TeamName: team.TeamName,
		Members:  memberDTOs,
	}

	sendJSONResponse(w, http.StatusOK, response)
}

type UserHandler struct{ userService service.UserService }

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) SetUserIsActive(w http.ResponseWriter, r *http.Request) {
	var reqBody UserIsActiveRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.userService.SetUserIsActive(context.Background(), reqBody.UserID, reqBody.IsActive)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	sendJSONResponse(w, http.StatusOK, user)
}

func (h *UserHandler) GetReviewPRs(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "Missing user_id query parameter", http.StatusBadRequest)
		return
	}

	prs, err := h.userService.GetReviewPRsByUserID(context.Background(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response := struct {
		UserID       string                    `json:"user_id"`
		PullRequests []domain.PullRequestShort `json:"pull_requests"`
	}{
		UserID:       userID,
		PullRequests: prs,
	}

	sendJSONResponse(w, http.StatusOK, response)
}
