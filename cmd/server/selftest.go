package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type smokeResult struct {
	JobID    string `json:"job_id"`
	Revision uint64 `json:"revision"`
	State    string `json:"state"`
}
type smokeView struct {
	Job struct {
		Revision           uint64
		State              string
		ReviewSampleDigest string
	}
	Samples          []string
	RemediationItems []struct{ PointID, Status, ReplacementPointID string } `json:"remediation_items"`
}

func runSelftest(server *http.Server, ln net.Listener) error {
	done := make(chan error, 1)
	go func() { done <- server.Serve(ln) }()
	base := "http://" + ln.Addr().String()
	client := newSmokeClient(base)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if e := client.request("GET", "/healthz", nil, &map[string]any{}); e == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("自检服务未就绪")
		}
		time.Sleep(20 * time.Millisecond)
	}
	e := exercise(client)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(ctx)
	serveErr := <-done
	if serveErr == http.ErrServerClosed {
		serveErr = nil
	}
	if e != nil {
		return fmt.Errorf("HTTP 自检失败: %w", e)
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if serveErr != nil {
		return serveErr
	}
	fmt.Println("自检通过：超限整改、独立复核、发布冻结和清单摘要校验均成功")
	return nil
}
func exercise(c *smokeClient) error {
	var r smokeResult
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	create := map[string]any{"RequestID": c.id(), "JobID": "selftest-job", "Title": "民国城区图", "MapYear": 1936, "ImageWidth": 1000, "ImageHeight": 800, "ImageSHA256": sha, "TargetCRS": "EPSG:3857", "RMSELimit": 3.5, "OperatorID": "operator-a"}
	if e := c.request("POST", "/api/jobs", create, &r); e != nil {
		return e
	}
	if e := revision(c, "freeze", &r); e != nil {
		return e
	}
	points := []map[string]any{{"id": "p1", "px": 100, "py": 100, "mx": 210, "my": 320}, {"id": "p2", "px": 500, "py": 100, "mx": 1010, "my": 320}, {"id": "p3", "px": 900, "py": 100, "mx": 1810, "my": 320}, {"id": "p4", "px": 900, "py": 700, "mx": 1820, "my": 2140}, {"id": "p5", "px": 500, "py": 700, "mx": 1010, "my": 2120}, {"id": "p6", "px": 100, "py": 700, "mx": 210, "my": 2120}, {"id": "p7", "px": 100, "py": 400, "mx": 210, "my": 1220}, {"id": "p8", "px": 500, "py": 400, "mx": 1010, "my": 1220}, {"id": "p9", "px": 900, "py": 400, "mx": 1810, "my": 1220}}
	for _, p := range points {
		body := map[string]any{"request_id": c.id(), "expected_revision": r.Revision, "point_id": p["id"], "pixel_x": p["px"], "pixel_y": p["py"], "map_x": p["mx"], "map_y": p["my"], "evidence_note": "自检基准点", "actor_id": "operator-a"}
		if e := c.request("POST", "/api/jobs/selftest-job/points", body, &r); e != nil {
			return e
		}
	}
	if e := revision(c, "evaluations", &r); e != nil {
		return e
	}
	if r.State != "needs_fix" {
		return fmt.Errorf("预期首次求解进入待整改，实际 %s", r.State)
	}
	fix := map[string]any{"request_id": c.id(), "expected_revision": r.Revision, "old_point_id": "p4", "new_point_id": "p4-r", "pixel_x": 900, "pixel_y": 700, "map_x": 1810, "map_y": 2120, "reason": "残差超限", "evidence_note": "复核稳定地物后更正", "actor_id": "operator-a"}
	var preview struct {
		PreviewDigest string `json:"preview_digest"`
		WouldPass     bool   `json:"would_pass"`
	}
	if e := c.request("POST", "/api/jobs/selftest-job/remediations/preview", fix, &preview); e != nil {
		return e
	}
	if !preview.WouldPass {
		return fmt.Errorf("整改候选预演未通过")
	}
	fix["preview_digest"] = preview.PreviewDigest
	if e := c.request("POST", "/api/jobs/selftest-job/remediations", fix, &r); e != nil {
		return e
	}
	if e := revision(c, "evaluations", &r); e != nil {
		return e
	}
	if r.State != "solvable" {
		return fmt.Errorf("整改后未通过")
	}
	var closedView smokeView
	if e := c.request("GET", "/api/jobs/selftest-job", nil, &closedView); e != nil {
		return e
	}
	for _, item := range closedView.RemediationItems {
		if item.Status != "closed" {
			return fmt.Errorf("整改事项 %s 未关闭（替换点 %s）", item.PointID, item.ReplacementPointID)
		}
	}
	if e := revision(c, "submit-review", &r); e != nil {
		return e
	}
	var view smokeView
	if e := c.request("GET", "/api/jobs/selftest-job", nil, &view); e != nil {
		return e
	}
	samples := map[string]bool{}
	for _, id := range view.Samples {
		samples[id] = true
	}
	review := map[string]any{"request_id": c.id(), "expected_revision": r.Revision, "reviewer_id": "reviewer-b", "decision": "approve", "notes": "叠加样本一致", "samples": samples, "sample_digest": view.Job.ReviewSampleDigest}
	if e := c.request("POST", "/api/jobs/selftest-job/reviews", review, &r); e != nil {
		return e
	}
	if e := revision(c, "release", &r); e != nil {
		return e
	}
	var verify struct {
		Valid bool `json:"valid"`
	}
	if e := c.request("GET", "/api/jobs/selftest-job/manifest/verify", nil, &verify); e != nil {
		return e
	}
	if !verify.Valid {
		return fmt.Errorf("清单摘要不一致")
	}
	return nil
}
func revision(c *smokeClient, path string, r *smokeResult) error {
	return c.request("POST", "/api/jobs/selftest-job/"+path, map[string]any{"request_id": c.id(), "expected_revision": r.Revision}, r)
}
