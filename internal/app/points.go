package app

import (
	"fmt"
	"map-registration-gate/internal/domain"
	"map-registration-gate/internal/store"
)

func (s *Service) AddPoint(c PointCommand) (Result, error) {
	if c.PointID == "" {
		c.PointID = stableID("point", c.RequestID)
	}
	return s.writePoint(c, "point.added", "")
}
func (s *Service) writePoint(c PointCommand, event, supersedes string) (Result, error) {
	if e := validRequest(c.RequestID); e != nil {
		return Result{}, e
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	var out Result
	err := s.store.Update(func(d *store.Data) error {
		if r, ok, e := replay(d, c.RequestID, c); ok || e != nil {
			out = r
			return e
		}
		j, ok := d.Jobs[c.JobID]
		if !ok {
			return domain.ErrNotFound
		}
		if e := checkRevision(j, c.ExpectedRevision); e != nil {
			return e
		}
		if !j.State.AllowsPointMaintenance() {
			return domain.ErrInvalidState
		}
		if e := domain.ValidateOperator(j, c.ActorID); e != nil {
			return e
		}
		for _, p := range d.Points[c.JobID] {
			if p.PointID == c.PointID {
				return fmt.Errorf("%w: duplicate point_id", domain.ErrRule)
			}
		}
		p := domain.ControlPoint{PointID: c.PointID, JobID: c.JobID, PixelX: c.PixelX, PixelY: c.PixelY, MapX: c.MapX, MapY: c.MapY, EvidenceNote: c.EvidenceNote, Active: true, SupersedesPointID: supersedes, CreatedBy: c.ActorID, CreatedAt: s.now()}
		if e := domain.ValidatePoint(j, d.Points[c.JobID], p); e != nil {
			return e
		}
		d.Points[c.JobID] = append(d.Points[c.JobID], p)
		j.CurrentEvaluationID = ""
		j.ReviewSampleDigest = ""
		delete(d.Reviews, c.JobID)
		j.Revision++
		if domain.Distribution(j, d.Points[c.JobID]) {
			j.State = domain.Solvable
		} else {
			j.State = domain.Frozen
		}
		d.Jobs[c.JobID] = j
		store.AddEvent(d, j.JobID, event, p.PointID)
		out = Result{j.JobID, j.Revision, string(j.State), false}
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}
func (s *Service) ReplacePoint(c ReplacePointCommand) (Result, error) {
	if c.NewPointID == "" {
		c.NewPointID = stableID("point", c.RequestID)
	}
	if c.Reason == "" || c.EvidenceNote == "" {
		return Result{}, fmt.Errorf("%w: remediation evidence required", domain.ErrRule)
	}
	unlock := s.lock(c.JobID)
	defer unlock()
	var out Result
	err := s.store.Update(func(d *store.Data) error {
		if r, ok, e := replay(d, c.RequestID, c); ok || e != nil {
			out = r
			return e
		}
		j, ok := d.Jobs[c.JobID]
		if !ok {
			return domain.ErrNotFound
		}
		if e := checkRevision(j, c.ExpectedRevision); e != nil {
			return e
		}
		if j.State != domain.NeedsFix {
			return domain.ErrInvalidState
		}
		if e := domain.ValidateOperator(j, c.ActorID); e != nil {
			return e
		}
		itemIndex := -1
		itemID := c.RemediationItemID
		for i := range d.RemediationItems[c.JobID] {
			item := d.RemediationItems[c.JobID][i]
			if item.Status != "closed" && item.PointID == c.OldPointID && (itemID == "" || itemID == item.ItemID) {
				itemIndex = i
				itemID = item.ItemID
				break
			}
		}
		if itemIndex < 0 {
			return fmt.Errorf("%w: open remediation item required", domain.ErrRule)
		}
		preview, e := buildReplacementPreview(j, d.Points[c.JobID], d.Evals[c.JobID], c)
		if e != nil {
			return e
		}
		if c.PreviewDigest == "" || preview.PreviewDigest != c.PreviewDigest {
			return fmt.Errorf("%w: replacement preview expired", domain.ErrConflict)
		}
		if !preview.WouldPass {
			return fmt.Errorf("%w: replacement preview still fails quality rules", domain.ErrRule)
		}
		pts := d.Points[c.JobID]
		found := false
		for i := range pts {
			if pts[i].PointID == c.OldPointID && pts[i].Active {
				pts[i].Active = false
				found = true
			}
		}
		if !found {
			return domain.ErrNotFound
		}
		p := domain.ControlPoint{PointID: c.NewPointID, JobID: c.JobID, PixelX: c.PixelX, PixelY: c.PixelY, MapX: c.MapX, MapY: c.MapY, EvidenceNote: c.EvidenceNote, Active: true, SupersedesPointID: c.OldPointID, CreatedBy: c.ActorID, CreatedAt: s.now()}
		if e := domain.ValidatePoint(j, pts, p); e != nil {
			return e
		}
		d.Points[c.JobID] = append(pts, p)
		j.CurrentEvaluationID = ""
		j.ReviewSampleDigest = ""
		delete(d.Reviews, c.JobID)
		d.Remediations[c.JobID] = append(d.Remediations[c.JobID], domain.Remediation{RemediationID: stableID("remediation", c.RequestID), JobID: c.JobID, ItemID: itemID, OldPointID: c.OldPointID, NewPointID: c.NewPointID, Reason: c.Reason, ReplacementEvidence: c.EvidenceNote, ActorID: c.ActorID, CreatedAt: s.now()})
		items := d.RemediationItems[c.JobID]
		items[itemIndex].ReplacementPointID = c.NewPointID
		items[itemIndex].UpdatedAt = s.now()
		d.RemediationItems[c.JobID] = items
		instructions := d.ReturnInstructions[c.JobID]
		for i := range instructions {
			if instructions[i].PointID == c.OldPointID && instructions[i].Status == "open" {
				instructions[i].Status = "addressed"
			}
		}
		d.ReturnInstructions[c.JobID] = instructions
		j.Revision++
		d.Jobs[c.JobID] = j
		store.AddEvent(d, j.JobID, "point.remediated", c.OldPointID+":"+c.Reason)
		out = Result{j.JobID, j.Revision, string(j.State), false}
		remember(d, c.RequestID, c, out)
		return nil
	})
	return out, err
}

func (s *Service) PreviewReplacement(c ReplacePointCommand) (ReplacementPreview, error) {
	unlock := s.lock(c.JobID)
	defer unlock()
	d := s.store.Snapshot()
	j, ok := d.Jobs[c.JobID]
	if !ok {
		return ReplacementPreview{}, domain.ErrNotFound
	}
	if err := checkRevision(j, c.ExpectedRevision); err != nil {
		return ReplacementPreview{}, err
	}
	if j.State != domain.NeedsFix {
		return ReplacementPreview{}, domain.ErrInvalidState
	}
	if err := domain.ValidateOperator(j, c.ActorID); err != nil {
		return ReplacementPreview{}, err
	}
	found := false
	for _, item := range d.RemediationItems[c.JobID] {
		if item.Status != "closed" && item.PointID == c.OldPointID && (c.RemediationItemID == "" || item.ItemID == c.RemediationItemID) {
			found = true
			break
		}
	}
	if !found {
		return ReplacementPreview{}, fmt.Errorf("%w: open remediation item required", domain.ErrRule)
	}
	return buildReplacementPreview(j, d.Points[c.JobID], d.Evals[c.JobID], c)
}
