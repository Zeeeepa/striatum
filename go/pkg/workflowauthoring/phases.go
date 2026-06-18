package workflowauthoring

import "strings"

// PhaseIndex captures the phase layout of a workflow: which phases exist, their
// order, which phase each job belongs to, and the single phase_synthesis job
// that closes each phase.
//
// The phase-shape rules enforced here are the SINGLE SOURCE OF TRUTH shared by
// `workflow validate` (pkg/workflowauthoring.Validate) and `run.prepare`
// (pkg/mutations.validateWorkflowForPrepare). Keep the error strings here in
// sync with pkg/mutations/run.go so operators see the same diagnostic from
// local validation and from the daemon at launch.
type PhaseIndex struct {
	Declared         bool
	PhaseOrder       []string
	PhasePosition    map[string]int
	JobPhase         map[string]string
	SynthesisByPhase map[string]string
}

func emptyPhaseIndex() PhaseIndex {
	return PhaseIndex{
		PhasePosition:    map[string]int{},
		JobPhase:         map[string]string{},
		SynthesisByPhase: map[string]string{},
	}
}

// ValidatePhaseShapes enforces the phase-shape rules that `run.prepare`
// requires, so `workflow validate` rejects the same shapes locally instead of
// passing then failing at launch (GH #66). It mirrors workflowPhaseIndex and
// validatePhaseEdges in pkg/mutations/run.go.
//
// Rules:
//   - schema v1 workflows must not declare phases, phase_id, or phase_synthesis.
//   - phase_id / phase_synthesis are only allowed when phases are declared.
//   - phase ids must be unique and non-empty.
//   - every job's phase_id must reference a declared phase.
//   - each phase declares exactly one phase_synthesis job, with at least one peer.
//   - a phase's declared synthesis_job_id must name that phase's phase_synthesis job.
//   - cross-phase edges may target only the immediate next phase, must originate
//     from the source phase's synthesis job, and may not target a later
//     phase_synthesis job.
func ValidatePhaseShapes(workflow map[string]any) error {
	schemaVersion := stringValue(workflow["schema_version"])
	jobs := jobDefinitions(workflow)
	index, err := buildPhaseIndex(workflow, jobs, schemaVersion)
	if err != nil {
		return err
	}
	if err := validatePhaseEdges(workflow, index); err != nil {
		return err
	}
	return validatePhaseGateSequencing(workflow, jobs)
}

// validatePhaseGateSequencing enforces RFC 0094 §2 work-packet *type* sequencing.
//
// A phase may declare a `gate` that withholds the *emission of work packets of a
// given type* until a named adjudicator/verdict job clears:
//
//	phases:
//	  - id: reconstruct
//	    gate:
//	      withhold_packet_types: [proposal]      # types unreachable until...
//	      until_verdict_clears: coverage_gate    # ...this verdict job clears
//
// This is a *generator-authoring* capability, not a daemon-authority gap: it
// compiles to ordinary RFC 0045 cross-phase dependencies (every withheld-type
// job depends, via a declared `completed` edge, on the gate job's clearing
// verdict — exactly as a commit phase depends on a review verdict today). The
// daemon already withholds jobs behind dependency gates, so no new RPC method or
// route is added. We validate the declaration here so the misconfiguration
// surfaces at author time rather than producing a graph that silently fails to
// gate.
//
// Rules:
//   - the gate's `until_verdict_clears` must name a real job whose `type` emits a
//     verdict (so a clearing verdict actually exists to gate on);
//   - `withhold_packet_types` must be a non-empty list of unique, non-empty type
//     names, each naming a type that at least one job in the workflow declares (a
//     withhold set that gates nothing is a configuration error);
//   - the gate job must not itself be of a withheld type (it could never clear);
//   - every withheld-type job must declare a dependency edge from the gate job
//     (the compiled cross-phase dependency), so the withholding is real and not
//     merely advisory.
func validatePhaseGateSequencing(workflow map[string]any, jobs []map[string]any) error {
	phases := anySlice(workflow["phases"])
	if len(phases) == 0 {
		return nil
	}
	jobTypes := map[string]string{}
	typeCount := map[string]int{}
	for _, job := range jobs {
		id := stringValue(job["id"])
		jobType := defaultString(job["type"], "generic")
		jobTypes[id] = jobType
		typeCount[jobType]++
	}
	dependsOn := map[string]map[string]bool{}
	for _, edge := range phaseDeclaredEdges(workflow) {
		from, to := edge[0], edge[1]
		if dependsOn[to] == nil {
			dependsOn[to] = map[string]bool{}
		}
		dependsOn[to][from] = true
	}
	for _, phaseValue := range phases {
		phase, _ := phaseValue.(map[string]any)
		gate, ok := phase["gate"].(map[string]any)
		if !ok {
			continue
		}
		phaseID := stringValue(phase["id"])
		withholdRaw, declared := gate["withhold_packet_types"]
		if !declared {
			// A gate without withhold_packet_types carries no type-sequencing
			// claim; leave it for other phase-gate semantics to interpret.
			continue
		}
		gateJobID := stringValue(gate["until_verdict_clears"])
		if gateJobID == "" {
			return errf("phase %q gate.withhold_packet_types requires gate.until_verdict_clears naming the gate job", phaseID)
		}
		gateType, gateExists := jobTypes[gateJobID]
		if !gateExists {
			return errf("phase %q gate.until_verdict_clears %q does not name a declared job", phaseID, gateJobID)
		}
		if !verdictJobTypes[gateType] {
			return errf("phase %q gate.until_verdict_clears %q must name a verdict-emitting job (type review or phase_synthesis), got %q", phaseID, gateJobID, gateType)
		}
		withheld := stringsFromSlice(withholdRaw)
		if len(withheld) == 0 {
			return errf("phase %q gate.withhold_packet_types must list at least one work-packet type", phaseID)
		}
		seen := map[string]bool{}
		for _, packetType := range withheld {
			if strings.TrimSpace(packetType) == "" {
				return errf("phase %q gate.withhold_packet_types entries must be non-empty type names", phaseID)
			}
			if seen[packetType] {
				return errf("phase %q gate.withhold_packet_types lists %q more than once", phaseID, packetType)
			}
			seen[packetType] = true
			if typeCount[packetType] == 0 {
				return errf("phase %q gate.withhold_packet_types references type %q that no job declares", phaseID, packetType)
			}
			if gateType == packetType {
				return errf("phase %q gate job %q cannot itself be of withheld type %q (it could never clear)", phaseID, gateJobID, packetType)
			}
		}
		// Every job of a withheld type must depend on the gate job, so the
		// withholding compiles to a real RFC 0045 dependency the daemon enforces.
		for _, job := range jobs {
			jobID := stringValue(job["id"])
			if jobID == gateJobID || !seen[jobTypes[jobID]] {
				continue
			}
			if !dependsOn[jobID][gateJobID] {
				return errf("phase %q withholds type %q until %q clears, but job %q does not declare a dependency edge from %q", phaseID, jobTypes[jobID], gateJobID, jobID, gateJobID)
			}
		}
	}
	return nil
}

func jobDefinitions(workflow map[string]any) []map[string]any {
	result := []map[string]any{}
	for _, item := range anySlice(workflow["jobs"]) {
		if job, ok := item.(map[string]any); ok {
			result = append(result, job)
		}
	}
	return result
}

func buildPhaseIndex(workflow map[string]any, jobs []map[string]any, schemaVersion string) (PhaseIndex, error) {
	empty := emptyPhaseIndex()
	phases := anySlice(workflow["phases"])
	if schemaVersion == SchemaV1 {
		if len(phases) > 0 {
			return empty, errf("striatum.workflow.v1 workflows must not declare phases")
		}
		for _, job := range jobs {
			if _, ok := job["phase_id"]; ok {
				return empty, errf("striatum.workflow.v1 job %q must not declare phase_id", stringValue(job["id"]))
			}
			if stringValue(job["type"]) == "phase_synthesis" {
				return empty, errf("striatum.workflow.v1 job %q must not use type phase_synthesis", stringValue(job["id"]))
			}
		}
		return empty, nil
	}
	if len(phases) == 0 {
		for _, job := range jobs {
			if _, ok := job["phase_id"]; ok {
				return empty, errf("job %q may declare phase_id only when workflow phases are declared", stringValue(job["id"]))
			}
			if stringValue(job["type"]) == "phase_synthesis" {
				return empty, errf("job %q may use type phase_synthesis only when workflow phases are declared", stringValue(job["id"]))
			}
		}
		return empty, nil
	}
	index := PhaseIndex{
		Declared:         true,
		PhasePosition:    map[string]int{},
		JobPhase:         map[string]string{},
		SynthesisByPhase: map[string]string{},
	}
	phaseSeen := map[string]bool{}
	declaredSynthesis := map[string]string{}
	for pos, phaseValue := range phases {
		phase, _ := phaseValue.(map[string]any)
		phaseID := stringValue(phase["id"])
		if phaseID == "" || phaseSeen[phaseID] {
			return empty, errf("phase id must be unique and non-empty")
		}
		phaseSeen[phaseID] = true
		index.PhaseOrder = append(index.PhaseOrder, phaseID)
		index.PhasePosition[phaseID] = pos
		declaredSynthesis[phaseID] = stringValue(phase["synthesis_job_id"])
	}
	jobCountByPhase := map[string]int{}
	jobTypes := map[string]string{}
	for _, job := range jobs {
		jobID := stringValue(job["id"])
		phaseID := stringValue(job["phase_id"])
		jobTypes[jobID] = stringValue(job["type"])
		if phaseID == "" || !phaseSeen[phaseID] {
			return empty, errf("job %q references unknown phase_id %q", jobID, phaseID)
		}
		index.JobPhase[jobID] = phaseID
		jobCountByPhase[phaseID]++
		if stringValue(job["type"]) != "phase_synthesis" {
			continue
		}
		if existing := index.SynthesisByPhase[phaseID]; existing != "" {
			return empty, errf("phase %q has multiple phase_synthesis jobs: %s, %s", phaseID, existing, jobID)
		}
		index.SynthesisByPhase[phaseID] = jobID
	}
	for _, phaseID := range index.PhaseOrder {
		if index.SynthesisByPhase[phaseID] == "" {
			return empty, errf("phase %q must declare exactly one phase_synthesis job", phaseID)
		}
		if jobCountByPhase[phaseID] < 2 {
			return empty, errf("phase %q phase_synthesis job must have at least one peer job", phaseID)
		}
		// A phase's declared synthesis_job_id, when present, must name that
		// phase's phase_synthesis job (it belongs to the phase that names it).
		if declared := declaredSynthesis[phaseID]; declared != "" {
			if jobTypes[declared] != "phase_synthesis" || index.JobPhase[declared] != phaseID {
				return empty, errf("phase %q synthesis_job_id %q must name a phase_synthesis job in phase %q", phaseID, declared, phaseID)
			}
			if declared != index.SynthesisByPhase[phaseID] {
				return empty, errf("phase %q synthesis_job_id %q does not match its phase_synthesis job %q", phaseID, declared, index.SynthesisByPhase[phaseID])
			}
		}
	}
	return index, nil
}

// validatePhaseEdges mirrors validatePhaseEdges in pkg/mutations/run.go. Only
// "completed" edges declared directly on the workflow participate (the
// phase-materialized synthesis fan-in is not cross-phase and is excluded).
func validatePhaseEdges(workflow map[string]any, index PhaseIndex) error {
	if !index.Declared {
		return nil
	}
	jobTypes := map[string]string{}
	for _, job := range jobDefinitions(workflow) {
		jobTypes[stringValue(job["id"])] = stringValue(job["type"])
	}
	for _, edge := range phaseDeclaredEdges(workflow) {
		fromID := edge[0]
		toID := edge[1]
		fromPhase := index.JobPhase[fromID]
		toPhase := index.JobPhase[toID]
		if fromPhase == "" || toPhase == "" || fromPhase == toPhase {
			continue
		}
		fromPos := index.PhasePosition[fromPhase]
		toPos := index.PhasePosition[toPhase]
		if toPos < fromPos {
			return errf("workflow edge %q -> %q points from later phase %q to earlier phase %q", fromID, toID, fromPhase, toPhase)
		}
		if toPos != fromPos+1 {
			return errf("workflow edge %q -> %q skips phases; cross-phase edges may target only the immediate next phase", fromID, toID)
		}
		if index.SynthesisByPhase[fromPhase] != fromID {
			return errf("workflow edge %q -> %q crosses phases without using source phase %q synthesis job", fromID, toID, fromPhase)
		}
		if jobTypes[toID] == "phase_synthesis" {
			return errf("workflow edge %q -> %q cannot target a later phase_synthesis job", fromID, toID)
		}
	}
	return nil
}

func phaseDeclaredEdges(workflow map[string]any) [][2]string {
	pairs := [][2]string{}
	seen := map[string]bool{}
	for _, item := range anySlice(workflow["edges"]) {
		edge, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fromID := stringValue(edge["from"])
		toID := stringValue(edge["to"])
		if fromID == "" || toID == "" || edge["on"] != "completed" {
			continue
		}
		key := fromID + "\x00" + toID
		if seen[key] {
			continue
		}
		seen[key] = true
		pairs = append(pairs, [2]string{fromID, toID})
	}
	return pairs
}
