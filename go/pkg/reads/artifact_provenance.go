package reads

import "strings"

const (
	artifactProvenanceAttestedLane        = "attested_supervised_lane"
	artifactProvenanceUnattestedSession   = "unattested_no_supervisor_session"
	artifactProvenanceAutoFinalized       = "daemon_auto_finalized_from_artifact"
	artifactProvenancePublishedOnBehalf   = "operator_published_on_behalf"
	artifactProvenanceOperatorSelf        = "operator_self_declared"
	artifactProvenanceRecoveryAuthored    = "recovery_authored"
	artifactProvenanceRunLevelOperator    = "operator_authored"
	artifactProvenanceUnknown             = "unknown"
	artifactEventAutoFinalized            = "artifact.auto_finalized"
	artifactEventPublishWithoutProcess    = "provenance.publish_without_process_execution"
	artifactEventRecoveryAutoPublished    = "recovery.auto_published"
	artifactEvidenceSupervisorObservation = "supervisor_artifact_observed"
	artifactEvidenceCleanProcessExecution = "clean_process_execution"
)

const artifactProvenanceColumns = `,
		        a.attestation_override_rationale AS _provenance_override_rationale,
		        s.operator_label AS _provenance_operator_label,
		        s.state AS _provenance_session_state,
		        ps.supervisor_id AS _provenance_supervisor_id,
		        ps.state AS _provenance_supervisor_state,
		        observed.event_id AS _provenance_observed_event_id,
		        pe.process_id AS _provenance_process_id,
		        direct_event.event_type AS _provenance_event_type,
		        direct_event.payload_json AS _provenance_event_payload,
		        recovery_event.event_type AS _provenance_recovery_event_type,
		        recovery_event.payload_json AS _provenance_recovery_event_payload`

const artifactProvenanceJoins = `
		   LEFT JOIN striatumd.sessions s
		     ON s.repository_id = a.repository_id
		    AND s.session_id = a.session_id
		   LEFT JOIN LATERAL (
		     SELECT ps.supervisor_id, ps.state
		       FROM striatumd.process_supervisors ps
		      WHERE ps.repository_id = a.repository_id
		        AND ps.run_id = a.run_id
		        AND ps.session_id = a.session_id
		        AND ps.state <> 'starting'
		        AND ps.started_at <= a.created_at
		        AND (ps.ended_at IS NULL OR ps.ended_at >= a.created_at)
		      ORDER BY ps.started_at DESC, ps.supervisor_id DESC
		      LIMIT 1
		   ) ps ON true
		   LEFT JOIN LATERAL (
		     SELECT e.event_id
		       FROM striatumd.events e
		      WHERE e.repository_id = a.repository_id
		        AND e.run_id = a.run_id
		        AND e.actor_session_id = a.session_id
		        AND e.event_type = 'supervisor.artifact_observed'
		        AND (
		          e.payload_json #>> '{payload,artifact_id}' = a.artifact_id
		          OR e.payload_json #>> '{payload,path}' = a.repo_path
		          OR e.payload_json->>'artifact_id' = a.artifact_id
		          OR e.payload_json->>'path' = a.repo_path
		        )
		      ORDER BY e.event_id DESC
		      LIMIT 1
		   ) observed ON true
		   LEFT JOIN LATERAL (
		     SELECT pe.process_id
		       FROM striatumd.process_executions pe
		      WHERE pe.repository_id = a.repository_id
		        AND pe.run_id = a.run_id
		        AND pe.job_id = a.job_id
		        AND pe.session_id = a.session_id
		        AND pe.state = 'exited'
		        AND pe.exit_code = 0
		        AND pe.started_at <= a.created_at
		      ORDER BY pe.ended_at DESC NULLS LAST, pe.process_id DESC
		      LIMIT 1
		   ) pe ON true
		   LEFT JOIN LATERAL (
		     SELECT e.event_type, e.payload_json
		       FROM striatumd.events e
		      WHERE e.repository_id = a.repository_id
		        AND e.run_id = a.run_id
		        AND e.event_type IN (
		          'artifact.auto_finalized',
		          'provenance.publish_without_process_execution'
		        )
		        AND (
		          e.artifact_id = a.artifact_id
		          OR e.payload_json->>'artifact_id' = a.artifact_id
		        )
		      ORDER BY CASE e.event_type
		                 WHEN 'provenance.publish_without_process_execution' THEN 0
		                 WHEN 'artifact.auto_finalized' THEN 1
		                 ELSE 2
		               END,
		               e.event_id DESC
		      LIMIT 1
		   ) direct_event ON true
		   LEFT JOIN LATERAL (
		     SELECT e.event_type, e.payload_json
		       FROM striatumd.events e
		      WHERE e.repository_id = a.repository_id
		        AND e.run_id = a.run_id
		        AND e.event_type = 'recovery.auto_published'
		        AND (
		          e.artifact_id = a.artifact_id
		          OR (
		            e.job_id IS NOT DISTINCT FROM a.job_id
		            AND e.actor_session_id IS NOT DISTINCT FROM a.session_id
		            AND COALESCE(e.payload_json->'artifacts', '[]'::jsonb) ? a.repo_path
		          )
		        )
		      ORDER BY e.event_id DESC
		      LIMIT 1
		   ) recovery_event ON true
`

func decorateArtifactProvenance(rows []map[string]any) {
	for _, row := range rows {
		row["provenance"] = artifactProvenance(row)
		removeArtifactProvenanceScratch(row)
	}
}

func artifactProvenanceCounts(rows []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		provenance, ok := row["provenance"].(map[string]any)
		if !ok {
			provenance = artifactProvenance(row)
		}
		category := stringValue(provenance["category"])
		if category == "" {
			category = artifactProvenanceUnknown
		}
		counts[category]++
	}
	return counts
}

func artifactProvenance(row map[string]any) map[string]any {
	category, source := artifactProvenanceCategory(row)
	attestation, reason := artifactLaneAttestation(row)
	return map[string]any{
		"category":                 category,
		"source":                   source,
		"lane_attestation":         attestation,
		"lane_attestation_reason":  reason,
		"supervisor_id":            nullableStringValue(row["_provenance_supervisor_id"]),
		"operator_label":           artifactOperatorLabel(row),
		"process_evidence":         artifactProcessEvidence(row),
		"event_type":               artifactProvenanceEventType(row),
		"override_rationale":       artifactOverrideRationale(row),
		"recovery_event_type":      nullableStringValue(row["_provenance_recovery_event_type"]),
		"session_state_at_read":    nullableStringValue(row["_provenance_session_state"]),
		"supervisor_state_at_read": nullableStringValue(row["_provenance_supervisor_state"]),
	}
}

func artifactProvenanceCategory(row map[string]any) (string, string) {
	switch {
	case stringFrom(row, "_provenance_event_type") == artifactEventAutoFinalized:
		return artifactProvenanceAutoFinalized, "artifact_auto_finalized_event"
	case stringFrom(row, "_provenance_event_type") == artifactEventPublishWithoutProcess || artifactOverrideRationale(row) != nil:
		return artifactProvenancePublishedOnBehalf, "publish_without_process_execution"
	case stringFrom(row, "_provenance_recovery_event_type") == artifactEventRecoveryAutoPublished:
		return artifactProvenanceRecoveryAuthored, "recovery_auto_published_event"
	case artifactHasSupervisorEvidence(row):
		return artifactProvenanceAttestedLane, "supervisor_evidence"
	case artifactOperatorLabel(row) != nil:
		return artifactProvenanceOperatorSelf, "session_operator_label"
	case stringFrom(row, "session_id") == "" && stringFrom(row, "job_id") == "":
		return artifactProvenanceRunLevelOperator, "run_level_artifact"
	case stringFrom(row, "session_id") != "":
		return artifactProvenanceUnattestedSession, "missing_supervisor_evidence"
	default:
		return artifactProvenanceUnknown, "insufficient_artifact_context"
	}
}

func artifactLaneAttestation(row map[string]any) (any, any) {
	if stringFrom(row, "session_id") == "" {
		return nil, nil
	}
	if artifactHasSupervisorEvidence(row) {
		return "attested", nil
	}
	return "unattested", "no_attached_supervisor"
}

func artifactHasSupervisorEvidence(row map[string]any) bool {
	return stringFrom(row, "_provenance_supervisor_id") != "" || row["_provenance_observed_event_id"] != nil
}

func artifactProcessEvidence(row map[string]any) any {
	if row["_provenance_observed_event_id"] != nil {
		return artifactEvidenceSupervisorObservation
	}
	if stringFrom(row, "_provenance_process_id") != "" {
		return artifactEvidenceCleanProcessExecution
	}
	return nil
}

func artifactProvenanceEventType(row map[string]any) any {
	return nullableStringValue(row["_provenance_event_type"])
}

func artifactOverrideRationale(row map[string]any) any {
	if value := nullableStringValue(row["_provenance_override_rationale"]); value != nil {
		return value
	}
	payload := objectOrEmpty(row["_provenance_event_payload"])
	for _, key := range []string{"override_rationale", "rationale"} {
		if value := nullableStringValue(payload[key]); value != nil {
			return value
		}
	}
	return nil
}

func artifactOperatorLabel(row map[string]any) any {
	if value := nullableStringValue(row["_provenance_operator_label"]); value != nil {
		return value
	}
	byline := artifactByline(row)
	const prefix = "author: operator-self-declared-"
	if strings.HasPrefix(byline, prefix) {
		return strings.TrimPrefix(byline, prefix)
	}
	return nil
}

func artifactByline(row map[string]any) string {
	if byline := stringFrom(row, "byline"); byline != "" {
		return byline
	}
	return stringFrom(row, "author_line")
}

func removeArtifactProvenanceScratch(row map[string]any) {
	for _, key := range []string{
		"_provenance_override_rationale",
		"_provenance_operator_label",
		"_provenance_session_state",
		"_provenance_supervisor_id",
		"_provenance_supervisor_state",
		"_provenance_observed_event_id",
		"_provenance_process_id",
		"_provenance_event_type",
		"_provenance_event_payload",
		"_provenance_recovery_event_type",
		"_provenance_recovery_event_payload",
	} {
		delete(row, key)
	}
}
