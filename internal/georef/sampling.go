package georef

import (
	"crypto/sha256"
	"map-registration-gate/internal/domain"
	"sort"
	"sync"
)

type rankedPoint struct {
	id   string
	rank [32]byte
	zone int
}

var reviewSampleCache sync.Map

func Sample(points []domain.ControlPoint, seed string) []string {
	key := seed + "\x00" + domain.ControlSetDigest(points)
	if cached, ok := reviewSampleCache.Load(key); ok {
		return cached.([]string)
	}
	out := calculateSample(points, seed)
	reviewSampleCache.Store(key, out)
	return out
}

func calculateSample(points []domain.ControlPoint, seed string) []string {
	active := make([]domain.ControlPoint, 0, len(points))
	for _, p := range points {
		if p.Active {
			active = append(active, p)
		}
	}
	if len(active) <= 3 {
		out := make([]string, 0, len(active))
		for _, p := range active {
			out = append(out, p.PointID)
		}
		sort.Strings(out)
		return out
	}
	minX, maxX, minY, maxY := active[0].PixelX, active[0].PixelX, active[0].PixelY, active[0].PixelY
	for _, p := range active[1:] {
		if p.PixelX < minX {
			minX = p.PixelX
		}
		if p.PixelX > maxX {
			maxX = p.PixelX
		}
		if p.PixelY < minY {
			minY = p.PixelY
		}
		if p.PixelY > maxY {
			maxY = p.PixelY
		}
	}
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	ranked := make([]rankedPoint, 0, len(active))
	for _, p := range active {
		zone := 0
		if p.PixelX >= cx {
			zone++
		}
		if p.PixelY >= cy {
			zone += 2
		}
		ranked = append(ranked, rankedPoint{p.PointID, sha256.Sum256([]byte(seed + "\x00" + p.PointID)), zone})
	}
	sort.Slice(ranked, func(i, j int) bool { return string(ranked[i].rank[:]) < string(ranked[j].rank[:]) })
	chosen := map[int]string{}
	for _, p := range ranked {
		if _, ok := chosen[p.zone]; !ok {
			chosen[p.zone] = p.id
		}
	}
	out := make([]string, 0, len(chosen))
	for _, id := range chosen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
