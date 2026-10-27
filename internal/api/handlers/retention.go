package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"fusion-platform/fusion-index/internal/api/dto"
	db "fusion-platform/fusion-index/internal/db/sqlc"
)

// RetentionHandler stores retention rules per artifact or per type and reports the
// effective rule of an artifact. Applying the rules is not done here: an external
// runner (planned maintenance pod) calls the bulk version delete with VersionFilter.
type RetentionHandler struct {
	queries *db.Queries
}

func NewRetentionHandler(q *db.Queries) *RetentionHandler {
	return &RetentionHandler{queries: q}
}

// mergeRetention combines the policies of an artifact's types conservatively: a rule
// deletes only what is outside keepLast AND older than maxAgeDays, so taking the
// maximum of each field (ignoring unset ones) deletes only what every policy would.
func mergeRetention(policies []db.RegistryRetentionPolicy) *dto.RetentionRule {
	if len(policies) == 0 {
		return nil
	}
	var rule dto.RetentionRule
	for _, p := range policies {
		if p.KeepLast != nil && (rule.KeepLast == nil || *p.KeepLast > *rule.KeepLast) {
			rule.KeepLast = p.KeepLast
		}
		if p.MaxAgeDays != nil && (rule.MaxAgeDays == nil || *p.MaxAgeDays > *rule.MaxAgeDays) {
			rule.MaxAgeDays = p.MaxAgeDays
		}
	}
	return &rule
}

// versionFilter renders a rule as the query string of the version list / bulk delete.
func versionFilter(rule *dto.RetentionRule, now time.Time) string {
	if rule == nil {
		return ""
	}
	q := url.Values{}
	if rule.KeepLast != nil {
		q.Set("keepLast", strconv.Itoa(int(*rule.KeepLast)))
	}
	if rule.MaxAgeDays != nil {
		q.Set("olderThan", now.UTC().AddDate(0, 0, -int(*rule.MaxAgeDays)).Truncate(time.Second).Format(time.RFC3339))
	}
	return q.Encode()
}

func bindRetentionRequest(c *gin.Context) (dto.SetRetentionRequest, bool) {
	var req dto.SetRetentionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return req, false
	}
	if req.KeepLast == nil && req.MaxAgeDays == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of keepLast, maxAgeDays is required"})
		return req, false
	}
	return req, true
}

func (h *RetentionHandler) GetForArtifact(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetRegistryArtifact(c, id); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}
	resp := dto.EffectiveRetentionResponse{Source: "none", FromTypes: []dto.RetentionPolicyResponse{}}

	own, err := h.queries.GetArtifactRetentionPolicy(c, &id)
	switch {
	case err == nil:
		r := dto.ToRetentionPolicyResponse(own)
		resp.Own = &r
		resp.Source = "artifact"
		resp.Rule = mergeRetention([]db.RegistryRetentionPolicy{own})
	case !isNotFound(err):
		internalError(c, err)
		return
	}

	fromTypes, err := h.queries.ListTypeRetentionPoliciesForArtifact(c, id)
	if err != nil {
		internalError(c, err)
		return
	}
	for _, p := range fromTypes {
		resp.FromTypes = append(resp.FromTypes, dto.ToRetentionPolicyResponse(p))
	}
	if resp.Own == nil && len(fromTypes) > 0 {
		resp.Source = "types"
		resp.Rule = mergeRetention(fromTypes)
	}
	resp.VersionFilter = versionFilter(resp.Rule, time.Now())
	c.JSON(http.StatusOK, resp)
}

func (h *RetentionHandler) PutForArtifact(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	req, ok := bindRetentionRequest(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetRegistryArtifact(c, id); err != nil {
		notFoundOrInternal(c, err, "artifact not found")
		return
	}
	p, err := h.queries.UpsertArtifactRetentionPolicy(c, db.UpsertArtifactRetentionPolicyParams{
		ArtifactID: &id, KeepLast: req.KeepLast, MaxAgeDays: req.MaxAgeDays,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToRetentionPolicyResponse(p))
}

func (h *RetentionHandler) DeleteForArtifact(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	n, err := h.queries.DeleteArtifactRetentionPolicy(c, &id)
	if err != nil {
		internalError(c, err)
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "retention policy not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RetentionHandler) GetForType(c *gin.Context) {
	id, ok := pathTypeID(c)
	if !ok {
		return
	}
	p, err := h.queries.GetTypeRetentionPolicy(c, &id)
	if err != nil {
		notFoundOrInternal(c, err, "retention policy not found")
		return
	}
	c.JSON(http.StatusOK, dto.ToRetentionPolicyResponse(p))
}

func (h *RetentionHandler) PutForType(c *gin.Context) {
	id, ok := pathTypeID(c)
	if !ok {
		return
	}
	req, ok := bindRetentionRequest(c)
	if !ok {
		return
	}
	if _, err := h.queries.GetArtifactType(c, id); err != nil {
		notFoundOrInternal(c, err, "type not found")
		return
	}
	p, err := h.queries.UpsertTypeRetentionPolicy(c, db.UpsertTypeRetentionPolicyParams{
		TypeID: &id, KeepLast: req.KeepLast, MaxAgeDays: req.MaxAgeDays,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToRetentionPolicyResponse(p))
}

func (h *RetentionHandler) DeleteForType(c *gin.Context) {
	id, ok := pathTypeID(c)
	if !ok {
		return
	}
	n, err := h.queries.DeleteTypeRetentionPolicy(c, &id)
	if err != nil {
		internalError(c, err)
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "retention policy not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// List returns every stored policy with the name of its artifact or type.
func (h *RetentionHandler) List(c *gin.Context) {
	rows, err := h.queries.ListRetentionPolicies(c)
	if err != nil {
		internalError(c, err)
		return
	}
	items := make([]dto.RetentionPolicyResponse, len(rows))
	for i, r := range rows {
		items[i] = dto.ToRetentionPolicyResponse(db.RegistryRetentionPolicy{
			ID: r.ID, ArtifactID: r.ArtifactID, TypeID: r.TypeID,
			KeepLast: r.KeepLast, MaxAgeDays: r.MaxAgeDays,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
		items[i].ArtifactName = r.ArtifactName
		items[i].TypeName = r.TypeName
	}
	c.JSON(http.StatusOK, items)
}
