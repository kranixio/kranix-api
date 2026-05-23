package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kranix-io/kranix-packages/template"
	"github.com/kranix-io/kranix-packages/types"
)

func (s *Server) handleGenerateManifests(w http.ResponseWriter, r *http.Request) {
	var req types.KranixAppTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	resp, err := template.GenerateKranixApp(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.recordAudit(r, "manifest.generate", "template", resp.Parsed.Name, "success", "", map[string]interface{}{
		"format": resp.Format,
	})
	writeJSON(w, http.StatusOK, map[string]string{
		"manifest": resp.Manifest,
	})
}

func (s *Server) handleGenerateKranixAppTemplate(w http.ResponseWriter, r *http.Request) {
	var req types.KranixAppTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	resp, err := template.GenerateKranixApp(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.recordAudit(r, "template.generate", "kranixapp", resp.Parsed.Name, "success", "", map[string]interface{}{
		"format":     resp.Format,
		"confidence": resp.Confidence,
	})
	writeJSON(w, http.StatusOK, resp)
}
