package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/example/complex-project/internal/models"
	"github.com/example/complex-project/pkg/database"
	"github.com/gorilla/mux"
)

// Handler handles HTTP requests for the API
type Handler struct {
	db     *database.DB
	router *mux.Router
}

// NewHandler creates a new API handler
func NewHandler(db *database.DB) *Handler {
	h := &Handler{
		db:     db,
		router: mux.NewRouter(),
	}

	h.setupRoutes()
	return h
}

// Router returns the configured router
func (h *Handler) Router() *mux.Router {
	return h.router
}

// setupRoutes configures all API routes
func (h *Handler) setupRoutes() {
	// User routes
	h.router.HandleFunc("/users", h.createUser).Methods("POST")
	h.router.HandleFunc("/users", h.getUsers).Methods("GET")
	h.router.HandleFunc("/users/{id}", h.getUser).Methods("GET")
	h.router.HandleFunc("/users/{id}", h.updateUser).Methods("PUT")
	h.router.HandleFunc("/users/{id}", h.deleteUser).Methods("DELETE")

	// Project routes
	h.router.HandleFunc("/projects", h.createProject).Methods("POST")
	h.router.HandleFunc("/projects", h.getProjects).Methods("GET")
	h.router.HandleFunc("/projects/{id}", h.getProject).Methods("GET")
	h.router.HandleFunc("/projects/{id}/users/{userId}", h.addUserToProject).Methods("POST")
}

// User handlers

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.db.CreateUser(req.Name, req.Email)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	h.writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) getUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.GetAllUsers()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to get users")
		return
	}

	h.writeJSON(w, http.StatusOK, users)
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := h.db.GetUser(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "User not found")
		return
	}

	h.writeJSON(w, http.StatusOK, user)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.db.UpdateUser(id, req.Name, req.Email)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	h.writeJSON(w, http.StatusOK, user)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	err = h.db.DeleteUser(id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Project handlers

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	var req models.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	project, err := h.db.CreateProject(req.Name, req.Description)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to create project")
		return
	}

	h.writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) getProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.db.GetAllProjects()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to get projects")
		return
	}

	h.writeJSON(w, http.StatusOK, projects)
}

func (h *Handler) getProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	project, err := h.db.GetProject(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Project not found")
		return
	}

	h.writeJSON(w, http.StatusOK, project)
}

func (h *Handler) addUserToProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectId, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	userId, err := strconv.Atoi(vars["userId"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	err = h.db.AddUserToProject(projectId, userId)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to add user to project")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Helper methods

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
