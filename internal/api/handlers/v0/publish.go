// Package v0 contains API handlers for version 0 of the API
package v0

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/modelcontextprotocol/registry/internal/service"
)

// PublishHandler handles requests to publish new server details to the registry
func PublishHandler(registry service.RegistryService, authService auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Parse request body into PublishRequest struct
		var publishReq model.PublishRequest
		err = json.Unmarshal(body, &publishReq)
		if err != nil {
			http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Get server details from the request
		serverDetail := publishReq.ServerDetail

		// Validate required fields
		if serverDetail.Name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		// Version is required
		if serverDetail.VersionDetail.Version == "" {
			http.Error(w, "Version is required", http.StatusBadRequest)
			return
		}

		// Get auth token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		// Handle bearer token format (e.g., "Bearer xyz123")
		token := authHeader
		if len(authHeader) > 7 && strings.ToUpper(authHeader[:7]) == "BEARER " {
			token = authHeader[7:]
		}

		// Determine authentication method based on server name prefix
		var authMethod model.AuthMethod
		switch {
		case strings.HasPrefix(serverDetail.Name, "io.github"):
			authMethod = model.AuthMethodGitHub
		// Additional cases can be added here for other prefixes
		default:
			// Keep the default auth method as AuthMethodNone
			authMethod = model.AuthMethodNone
		}

		// Setup authentication info
		a := model.Authentication{
			Method:  authMethod,
			Token:   token,
			RepoRef: serverDetail.Name,
		}

		valid, err := authService.ValidateAuth(r.Context(), a)
		if err != nil {
			if err == auth.ErrAuthRequired {
				http.Error(w, "Authentication is required for publishing", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if !valid {
			http.Error(w, "Invalid authentication credentials", http.StatusUnauthorized)
			return
		}

		// Call the publish method on the registry service
		err = registry.Publish(&serverDetail)
		if err != nil {
			http.Error(w, "Failed to publish server details: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Server publication successful",
			"id":      serverDetail.ID,
		})
	}
}
