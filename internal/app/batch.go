package app

import (
	"context"
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/georef"
	"map-registration-gate/internal/store"
	"math"
	"sort"
)

func (s *Service) PreviewBatch(c BatchPointsCommand) (BatchPreview, error) {
	unlock := s.lock(c.JobID)
	defer unlock()
	d := s.store.Snapshot()
	j, ok := d.Jobs[c.JobID]
	if !ok {
		return BatchPreview{}, domain.ErrNotFound
	}
	if err := checkRevision(j, c.ExpectedRevision); err != nil {
		return BatchPreview{}, err
	}
	if !j.State.AllowsPointMaintenance() || j.State == domain.NeedsFix {
		return BatchPreview{}, domain.ErrInvalidState
	}
	return buildBatchPreview(j, d.Points[c.JobID], c), nil
}

func buildBatchPreview(j domain.RegistrationJob, existing []domain.ControlPoint, c BatchPointsCommand) BatchPreview {
	out := BatchPreview{}
	if len(c.Points) == 0 || len(c.Points) > 500 {
		out.Errors = append(out.Errors, RowError{0, "batch_size", "批次必须包含 1 至 500 条点位"})
	}
	existingIDs, existingPixels := map[string]bool{}, map[string]bool{}
	for _, p := range existing {
		existingIDs[p.PointID] = true
		if p.Active {
			existingPixels[pixelKey(p.PixelX, p.PixelY)] = true
		}
	}
	ids, pixels := map[string][]int{}, map[string][]int{}
	validRows := make([]bool, len(c.Points))
	for i, p := range c.Points {
		row := i + 1
		validRows[i] = true
		bad := func(code, message string) {
			out.Errors = append(out.Errors, RowError{row, code, message})
			validRows[i] = false
		}
		if p.PointID == "" {
			bad("point_id_required", "点位标识不能为空")
		}
		if existingIDs[p.PointID] {
			bad("point_id_conflict", "点位标识已存在")
		}
		if p.ActorID == "" || p.ActorID != j.OperatorID {
			bad("operator_invalid", "操作者必须是任务数字化员")
		}
		if p.EvidenceNote == "" {
			bad("evidence_required", "证据说明不能为空")
		}
		if !finiteCoordinates(p.PixelX, p.PixelY, p.MapX, p.MapY) {
			bad("coordinate_invalid", "坐标必须是有限数值")
		} else if p.PixelX < 0 || p.PixelY < 0 || p.PixelX > float64(j.ImageWidth) || p.PixelY > float64(j.ImageHeight) {
			bad("pixel_out_of_bounds", "像素坐标超出底图边界")
		}
		ids[p.PointID] = append(ids[p.PointID], row)
		key := pixelKey(p.PixelX, p.PixelY)
		pixels[key] = append(pixels[key], row)
		if existingPixels[key] {
			bad("pixel_conflict", "像素位置与当前有效点重复")
		}
	}
	for _, rows := range ids {
		if len(rows) > 1 {
			for _, row := range rows {
				out.Errors = append(out.Errors, RowError{row, "batch_point_id_duplicate", "批次内点位标识重复"})
				validRows[row-1] = false
			}
		}
	}
	for _, rows := range pixels {
		if len(rows) > 1 {
			for _, row := range rows {
				out.Errors = append(out.Errors, RowError{row, "batch_pixel_duplicate", "批次内像素位置重复"})
				validRows[row-1] = false
			}
		}
	}
	sort.Slice(out.Errors, func(i, k int) bool {
		if out.Errors[i].Row == out.Errors[k].Row {
			return out.Errors[i].Code < out.Errors[k].Code
		}
		return out.Errors[i].Row < out.Errors[k].Row
	})
	projected := append([]domain.ControlPoint(nil), existing...)
	for i, p := range c.Points {
		if validRows[i] {
			projected = append(projected, domain.ControlPoint{PointID: p.PointID, JobID: j.JobID, PixelX: p.PixelX, PixelY: p.PixelY, MapX: p.MapX, MapY: p.MapY, EvidenceNote: p.EvidenceNote, CreatedBy: p.ActorID, Active: true})
		}
	}
	diagnosis := georef.Diagnose(j, projected)
	out.ActiveAfter, out.QuadrantCounts, out.MissingQuadrants = diagnosis.Active, diagnosis.QuadrantCounts, diagnosis.MissingQuadrants
	out.Solvable = len(out.Errors) == 0 && diagnosis.Ready
	out.PreviewDigest = requestHash(struct {
		JobID    string
		Revision uint64
		Points   []BatchPoint
		Errors   []RowError
	}{j.JobID, j.Revision, c.Points, out.Errors})
	return out
}

func (s *Service) ImportBatch(c BatchPointsCommand) (Result, error) {
	return s.ImportBatchContext(context.Background(), c)
}

func (s *Service) ImportBatchContext(ctx context.Context, c BatchPointsCommand) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := validRequest(c.RequestID); err != nil {
		return Result{}, err
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var out Result
	err := s.store.UpdateContext(ctx, func(d *store.Data) error {
		if r, ok, err := replay(d, c.RequestID, c); ok || err != nil {
			out = r
			return err
		}
		j, ok := d.Jobs[c.JobID]
		if !ok {
			return domain.ErrNotFound
		}
		if err := checkRevision(j, c.ExpectedRevision); err != nil {
			return err
		}
		if !j.State.AllowsPointMaintenance() || j.State == domain.NeedsFix {
			return domain.ErrInvalidState
		}
		preview := buildBatchPreview(j, d.Points[c.JobID], c)
		if len(preview.Errors) > 0 {
			return fmt.Errorf("%w: batch contains invalid rows", domain.ErrRule)
		}
		if c.PreviewDigest == "" || c.PreviewDigest != preview.PreviewDigest {
			return fmt.Errorf("%w: batch preview expired", domain.ErrConflict)
		}
		created := s.now()
		for _, p := range c.Points {
			d.Points[c.JobID] = append(d.Points[c.JobID], domain.ControlPoint{PointID: p.PointID, JobID: j.JobID, PixelX: p.PixelX, PixelY: p.PixelY, MapX: p.MapX, MapY: p.MapY, EvidenceNote: p.EvidenceNote, CreatedBy: p.ActorID, CreatedAt: created, Active: true})
		}
		invalidateEvaluation(d, j.JobID)
		advancePointRevision(&j, d.Points[j.JobID])
		d.Jobs[j.JobID] = j
		store.AddEvent(d, j.JobID, "points.batch_imported", fmt.Sprintf("count=%d,digest=%s", len(c.Points), preview.PreviewDigest))
		out = resultFor(j)
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}

func pixelKey(x, y float64) string { return fmt.Sprintf("%g/%g", x, y) }
func finiteCoordinates(v ...float64) bool {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}
