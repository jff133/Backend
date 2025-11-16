package api

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func NewRouter(prH *PRHandler, teamH *TeamHandler, userH *UserHandler) http.Handler {
	r := mux.NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Request: %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Teams
	r.HandleFunc("/team/add", teamH.CreateTeam).Methods("POST")

	// Users
	r.HandleFunc("/users/setIsActive", userH.SetUserIsActive).Methods("POST")
	r.HandleFunc("/users/getReview", userH.GetReviewPRs).Methods("GET").Queries("user_id", "{user_id}")

	// PullRequests
	r.HandleFunc("/pullRequest/create", prH.CreatePR).Methods("POST")
	r.HandleFunc("/pullRequest/merge", prH.MergePR).Methods("POST") // Используем body для PR_ID
	r.HandleFunc("/pullRequest/reassign", prH.ReassignReviewer).Methods("POST")

	return r
}
