package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func Digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func BaselineDigest(j RegistrationJob) string {
	return Digest(struct {
		Title  string  `json:"title"`
		Year   int     `json:"year"`
		Width  int     `json:"width"`
		Height int     `json:"height"`
		SHA    string  `json:"sha256"`
		CRS    string  `json:"crs"`
		Limit  float64 `json:"limit"`
	}{j.Title, j.MapYear, j.ImageWidth, j.ImageHeight, j.ImageSHA256, j.TargetCRS, j.RMSELimit})
}

func ActivePoints(points []ControlPoint) []ControlPoint {
	out := make([]ControlPoint, 0, len(points))
	for _, p := range points {
		if p.Active {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, k int) bool { return out[i].PointID < out[k].PointID })
	return out
}

func ControlSetDigest(points []ControlPoint) string {
	type canonicalPoint struct {
		ID       string  `json:"id"`
		PixelX   float64 `json:"pixel_x"`
		PixelY   float64 `json:"pixel_y"`
		MapX     float64 `json:"map_x"`
		MapY     float64 `json:"map_y"`
		Evidence string  `json:"evidence"`
		Creator  string  `json:"creator"`
	}
	active := ActivePoints(points)
	out := make([]canonicalPoint, 0, len(active))
	for _, p := range active {
		out = append(out, canonicalPoint{p.PointID, p.PixelX, p.PixelY, p.MapX, p.MapY, p.EvidenceNote, p.CreatedBy})
	}
	return Digest(out)
}

func ReviewDigest(r Review) string {
	items := append([]ReviewItem(nil), r.Items...)
	sort.Slice(items, func(i, k int) bool { return items[i].PointID < items[k].PointID })
	return Digest(struct {
		ReviewerID, Decision, Notes, SampleDigest string
		Items                                     []ReviewItem
	}{r.ReviewerID, r.Decision, r.Notes, r.SampleDigest, items})
}
