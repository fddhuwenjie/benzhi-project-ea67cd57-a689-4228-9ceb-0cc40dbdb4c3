package domain

func (s State) Terminal() bool               { return s == Published }
func (s State) AllowsPointMaintenance() bool { return s == Frozen || s == Solvable || s == NeedsFix }
func (s State) Known() bool {
	switch s {
	case Draft, Frozen, Solvable, NeedsFix, PendingReview, PendingRelease, Published:
		return true
	}
	return false
}
