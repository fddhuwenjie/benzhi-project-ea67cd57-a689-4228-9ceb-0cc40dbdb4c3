package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"map-registration-gate/internal/domain"
	"net/http"
)

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		problem(w, http.StatusBadRequest, "invalid_request", "请求 JSON 无效")
		return false
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		problem(w, http.StatusBadRequest, "invalid_request", "请求只能包含一个 JSON 对象")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, msg string) {
	respond(w, status, map[string]string{"code": code, "message": msg})
}
func appError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded):
		problem(w, 499, "request_canceled", "请求已取消或超时")
	case errors.Is(e, domain.ErrNotFound):
		problem(w, 404, "not_found", "任务或记录不存在")
	case errors.Is(e, domain.ErrConflict):
		problem(w, 409, "revision_conflict", "页面版本已过期，请刷新后重试")
	case errors.Is(e, domain.ErrIdempotency):
		problem(w, 409, "idempotency_conflict", "request_id 已用于其他请求")
	case errors.Is(e, domain.ErrImmutable):
		problem(w, 409, "immutable", "当前记录已冻结，不可修改")
	case errors.Is(e, domain.ErrInvalidState):
		problem(w, 409, "invalid_state", "当前任务状态不允许该操作")
	default:
		problem(w, 422, "rule_violation", e.Error())
	}
}

type revisionBody struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision uint64 `json:"expected_revision"`
}
