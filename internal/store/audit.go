package store

func AuditHead(d Data, jobID string) string {
	for i := len(d.Events) - 1; i >= 0; i-- {
		if d.Events[i].JobID == jobID {
			return d.Events[i].Hash
		}
	}
	return ""
}

type AuditVerification struct {
	Passed                   bool
	RecalculatedHead, Reason string
}

func VerifyAuditEvidence(d Data, jobID, recordedHead string) AuditVerification {
	publication := -1
	for i, event := range d.Events {
		if event.JobID == jobID && event.Type == "manifest.published" {
			publication = i
			break
		}
	}
	if publication < 0 {
		return AuditVerification{Reason: "publication_event_missing"}
	}
	expectedHead := d.Events[publication].PrevHash
	if expectedHead != recordedHead {
		return AuditVerification{RecalculatedHead: expectedHead, Reason: "audit_head_mismatch"}
	}
	prev := ""
	for i := 0; i < publication; i++ {
		e := d.Events[i]
		if e.Seq != uint64(i+1) || e.PrevHash != prev || eventHash(e) != e.Hash {
			return AuditVerification{RecalculatedHead: expectedHead, Reason: "audit_chain_break"}
		}
		if _, ok := d.Jobs[e.JobID]; !ok {
			return AuditVerification{RecalculatedHead: expectedHead, Reason: "audit_job_mismatch"}
		}
		prev = e.Hash
	}
	if prev != recordedHead {
		return AuditVerification{RecalculatedHead: prev, Reason: "audit_head_mismatch"}
	}
	return AuditVerification{Passed: true, RecalculatedHead: prev}
}

func AuditEvents(d Data, jobID string) int {
	n := 0
	for _, event := range d.Events {
		if event.JobID == jobID {
			n++
		}
	}
	return n
}
