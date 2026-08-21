// Package model holds the domain types shared across the batch-reactor
// kinetics engine: reactors, recipes (reaction kinetics + thermal safety
// parameters), the cross-contamination cleaning matrix, campaigns and their
// items, batches, batch lifecycle events and the derived safety result.
//
// All temperatures are stored in Kelvin, concentrations in mol/m³, energy in
// joules. Float64 fields hold the physical magnitudes; integer code/enum fields
// hold categorical state. The store layer maps these to/from SQLite rows.
package model

// ReactorStatus is the operational availability of a reactor.
type ReactorStatus string

const (
	ReactorAvailable   ReactorStatus = "available"
	ReactorBusy        ReactorStatus = "busy"
	ReactorMaintenance ReactorStatus = "maintenance"
)

// Reactor is a single batch reaction vessel.
type Reactor struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Volume           float64       `json:"volume"`             // m³
	HeatTransferU    float64       `json:"heat_transfer_u"`    // W/(m²·K)
	HeatTransferArea float64       `json:"heat_transfer_area"` // m²
	MaxOperatingTemp float64       `json:"max_operating_temp"` // K
	MaxPressure      float64       `json:"max_pressure"`       // bar
	Material         string        `json:"material"`
	Status           ReactorStatus `json:"status"`
	CreatedAt        int64         `json:"created_at"`
}

// ReactionOrder is the kinetic order w.r.t. the limiting reactant A (1 or 2).
type ReactionOrder int

const (
	OrderFirst  ReactionOrder = 1
	OrderSecond ReactionOrder = 2
)

// Recipe defines a reaction's kinetics, thermo-physical properties and the
// process temperature program plus product-quality / safety limits.
type Recipe struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Product       string        `json:"product"`
	K0            float64       `json:"k0"`             // pre-exponential, 1/s
	Ea            float64       `json:"ea"`             // J/mol
	Order         ReactionOrder `json:"order"`          // 1 or 2
	CA0           float64       `json:"ca0"`            // initial concentration, mol/m³
	DeltaHrx      float64       `json:"delta_h_rx"`     // J/mol (negative = exothermic)
	Rho           float64       `json:"rho"`            // kg/m³
	Cp            float64       `json:"cp"`             // J/(kg·K)
	T0            float64       `json:"t0"`             // initial / process temperature, K
	JacketTemp    float64       `json:"jacket_temp"`    // K
	ThermalLimit  float64       `json:"thermal_limit"`  // decomposition onset, K
	MinConversion float64       `json:"min_conversion"` // 0–1
	Duration      float64       `json:"duration"`       // target reaction time, s
	CreatedAt     int64         `json:"created_at"`
}

// CleaningSeverity is the cross-contamination class between two products.
type CleaningSeverity int

const (
	CleaningNone   CleaningSeverity = 0
	CleaningLight  CleaningSeverity = 1
	CleaningMedium CleaningSeverity = 2
	CleaningHeavy  CleaningSeverity = 3
)

// CleaningDuration returns the cleaning step duration (seconds) for a severity.
// Light cleaning is a real changeover step: it is short, but a crew must still
// hold the reactor for the rinse. Returning 0 would collapse that visible gap
// in the plan and let the next batch's planned start land inside the cleaning
// window, so each severity — including light — contributes a non-zero duration.
func (s CleaningSeverity) CleaningDuration() float64 {
	switch s {
	case CleaningLight:
		return 600
	case CleaningMedium:
		return 1800
	case CleaningHeavy:
		return 3600
	default:
		return 0
	}
}

// CleaningEntry is one cell of the cross-contamination cleaning matrix.
type CleaningEntry struct {
	FromProduct string           `json:"from_product"`
	ToProduct   string           `json:"to_product"`
	Severity    CleaningSeverity `json:"severity"`
}

// CampaignStatus is the lifecycle state of a campaign.
type CampaignStatus string

const (
	CampaignDraft     CampaignStatus = "draft"
	CampaignPlanned   CampaignStatus = "planned"
	CampaignRunning   CampaignStatus = "running"
	CampaignCompleted CampaignStatus = "completed"
	CampaignCancelled CampaignStatus = "cancelled"
)

// Campaign is an ordered set of campaign items to be run across reactors.
type Campaign struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    CampaignStatus `json:"status"`
	CreatedAt int64          `json:"created_at"`
}

// ItemStatus is the lifecycle state of a campaign item.
type ItemStatus string

const (
	ItemPending   ItemStatus = "pending"
	ItemScheduled ItemStatus = "scheduled"
	ItemRunning   ItemStatus = "running"
	ItemDone      ItemStatus = "done"
)

// CampaignItem is one (recipe, reactor, count) line in a campaign.
type CampaignItem struct {
	CampaignID string     `json:"campaign_id"`
	Seq        int        `json:"seq"`
	RecipeID   string     `json:"recipe_id"`
	ReactorID  string     `json:"reactor_id"`
	BatchCount int        `json:"batch_count"`
	Status     ItemStatus `json:"status"`
}

// BatchStatus is the lifecycle state of a batch.
type BatchStatus string

const (
	BatchQueued      BatchStatus = "queued"
	BatchCharging    BatchStatus = "charging"
	BatchReacting    BatchStatus = "reacting"
	BatchCooling     BatchStatus = "cooling"
	BatchDischarging BatchStatus = "discharging"
	BatchCleaning    BatchStatus = "cleaning"
	BatchDone        BatchStatus = "done"
	BatchFaulted     BatchStatus = "faulted"
	BatchAborted     BatchStatus = "aborted"
)

// IsTerminal reports whether the status is a terminal end-state.
func (s BatchStatus) IsTerminal() bool {
	return s == BatchDone || s == BatchFaulted || s == BatchAborted
}

// SafetyVerdict is the thermal-safety classification of a batch.
type SafetyVerdict string

const (
	VerdictSafe     SafetyVerdict = "safe"
	VerdictMarginal SafetyVerdict = "marginal"
	VerdictRunaway  SafetyVerdict = "runaway"
)

// Batch is one reaction run of a recipe in a reactor, with its lifecycle and
// the derived kinetics / safety result (populated after the reacting stage).
type Batch struct {
	ID              string        `json:"id"`
	CampaignID      string        `json:"campaign_id"`
	CampaignItemSeq int           `json:"campaign_item_seq"`
	ReactorID       string        `json:"reactor_id"`
	RecipeID        string        `json:"recipe_id"`
	Seq             int           `json:"seq"` // order on the reactor
	Status          BatchStatus   `json:"status"`
	PlannedStart    int64         `json:"planned_start"`
	StartedAt       int64         `json:"started_at"`
	EndedAt         int64         `json:"ended_at"`
	PeakTemp        float64       `json:"peak_temp"`
	Conversion      float64       `json:"conversion"`
	SafetyVerdict   SafetyVerdict `json:"safety_verdict"`
	StoesselClass   int           `json:"stoessel_class"`
	FaultReason     string        `json:"fault_reason"`
}

// EventType is the kind of batch lifecycle event recorded to the event log.
type EventType string

const (
	EventQueued      EventType = "queued"
	EventCharging    EventType = "charging"
	EventReacting    EventType = "reacting"
	EventCooling     EventType = "cooling"
	EventDischarging EventType = "discharging"
	EventCleaning    EventType = "cleaning"
	EventDone        EventType = "done"
	EventFaulted     EventType = "faulted"
	EventAborted     EventType = "aborted"
)

// BatchEvent is one append-only lifecycle record used for audit and recovery.
type BatchEvent struct {
	ID        int64     `json:"id"`
	BatchID   string    `json:"batch_id"`
	EventType EventType `json:"event_type"`
	At        int64     `json:"at"`
	Payload   string    `json:"payload"` // JSON blob
}

// SafetyResult is the derived kinetics + thermal-safety figure set for a batch,
// recomputable from the stored recipe + reactor inputs.
type SafetyResult struct {
	BatchID       string        `json:"batch_id"`
	DeltaTad      float64       `json:"delta_t_ad"` // K
	MTSR          float64       `json:"mtsr"`       // K
	StoesselClass int           `json:"stoessel_class"`
	TMRSeconds    float64       `json:"tmr_seconds"`
	PeakTemp      float64       `json:"peak_temp"` // K
	Conversion    float64       `json:"conversion"`
	Verdict       SafetyVerdict `json:"verdict"`
}

// KineticsInput is the input bundle for a kinetics simulation.
type KineticsInput struct {
	Recipe  Recipe  `json:"recipe"`
	Reactor Reactor `json:"reactor"`
	Dt      float64 `json:"dt"` // integration step, s (0 → default)
}

// KineticsResult is the output of a kinetics simulation run.
type KineticsResult struct {
	Conversion    float64       `json:"conversion"`
	PeakTemp      float64       `json:"peak_temp"`
	Duration      float64       `json:"duration"`
	QGenMax       float64       `json:"q_gen_max"`
	QRemAtPeak    float64       `json:"q_rem_at_peak"`
	DeltaTad      float64       `json:"delta_t_ad"`
	MTSR          float64       `json:"mtsr"`
	StoesselClass int           `json:"stoessel_class"`
	TMRSeconds    float64       `json:"tmr_seconds"`
	Verdict       SafetyVerdict `json:"verdict"`
	Steps         int           `json:"steps"`
	Isothermal    bool          `json:"isothermal"`
}

// CampaignPlan is the resolved schedule for a campaign: the batches to create
// per reactor with their planned start times, including cleaning gaps.
type CampaignPlan struct {
	CampaignID string         `json:"campaign_id"`
	Items      []PlannedItem  `json:"items"`
	Batches    []PlannedBatch `json:"batches"`
	Errors     []PlanError    `json:"errors"`
}

// PlannedItem is a campaign item with its resolved schedule per reactor.
type PlannedItem struct {
	Seq        int    `json:"seq"`
	RecipeID   string `json:"recipe_id"`
	ReactorID  string `json:"reactor_id"`
	BatchCount int    `json:"batch_count"`
}

// PlannedBatch is one batch in the plan with its reactor-relative sequence,
// planned start (epoch) and any preceding cleaning step.
type PlannedBatch struct {
	ReactorID       string  `json:"reactor_id"`
	RecipeID        string  `json:"recipe_id"`
	Seq             int     `json:"seq"`
	PlannedStart    int64   `json:"planned_start"`
	CleaningBefore  float64 `json:"cleaning_before"` // seconds
	CleaningProduct string  `json:"cleaning_product"`
}

// PlanError is a scheduling violation (e.g. thermal incompatibility) found while
// planning.
type PlanError struct {
	Seq      int    `json:"seq"`
	RecipeID string `json:"recipe_id"`
	Reactor  string `json:"reactor_id"`
	Reason   string `json:"reason"`
}

// RiskBand is the operator-facing severity used by forecasts and campaign
// reports. It deliberately separates current lifecycle state from thermal
// risk: a queued batch can already be critical because of its recipe.
type RiskBand string

const (
	RiskNormal   RiskBand = "normal"
	RiskWatch    RiskBand = "watch"
	RiskCritical RiskBand = "critical"
)

// ForecastPhase describes which operational milestone a batch is expected to
// reach next. It is derived from status and recipe duration, never persisted.
type ForecastPhase string

const (
	ForecastWaiting   ForecastPhase = "waiting"
	ForecastCharge    ForecastPhase = "charge"
	ForecastReact     ForecastPhase = "react"
	ForecastCool      ForecastPhase = "cool"
	ForecastDischarge ForecastPhase = "discharge"
	ForecastCleaning  ForecastPhase = "cleaning"
	ForecastTerminal  ForecastPhase = "terminal"
)

// BatchForecast is a deterministic operations view used by the reactor desk.
// Estimated timestamps are only planning aids: the authoritative timestamps
// remain the lifecycle event stream on Batch.
type BatchForecast struct {
	BatchID         string        `json:"batch_id"`
	ReactorID       string        `json:"reactor_id"`
	RecipeID        string        `json:"recipe_id"`
	Status          BatchStatus   `json:"status"`
	Phase           ForecastPhase `json:"phase"`
	Risk            RiskBand      `json:"risk"`
	PlannedStart    int64         `json:"planned_start"`
	EstimatedStart  int64         `json:"estimated_start"`
	EstimatedFinish int64         `json:"estimated_finish"`
	ScheduleSlip    int64         `json:"schedule_slip"`
	ThermalHeadroom float64       `json:"thermal_headroom"`
	ConversionGap   float64       `json:"conversion_gap"`
	TMRSeconds      float64       `json:"tmr_seconds"`
	NextAction      string        `json:"next_action"`
	Reasons         []string      `json:"reasons"`
}

// ReactorLoad describes planned and live load for one vessel. Planned seconds
// are accumulated from recipe durations; cleaning seconds are tracked
// separately so a planner can identify avoidable changeover time.
type ReactorLoad struct {
	ReactorID         string        `json:"reactor_id"`
	ReactorName       string        `json:"reactor_name"`
	Status            ReactorStatus `json:"status"`
	BatchCount        int           `json:"batch_count"`
	ActiveCount       int           `json:"active_count"`
	TerminalCount     int           `json:"terminal_count"`
	FaultCount        int           `json:"fault_count"`
	PlannedSeconds    float64       `json:"planned_seconds"`
	CleaningSeconds   float64       `json:"cleaning_seconds"`
	Utilization       float64       `json:"utilization"`
	FirstPlannedStart int64         `json:"first_planned_start"`
	LastPlannedFinish int64         `json:"last_planned_finish"`
}

// CampaignAnalytics is a read-only operational report assembled from campaign
// items, batches, recipes, reactors and derived safety results. It is designed
// to remain meaningful after a restart because every field is re-derived.
type CampaignAnalytics struct {
	CampaignID      string          `json:"campaign_id"`
	CampaignName    string          `json:"campaign_name"`
	CampaignStatus  CampaignStatus  `json:"campaign_status"`
	GeneratedAt     int64           `json:"generated_at"`
	BatchCount      int             `json:"batch_count"`
	QueuedCount     int             `json:"queued_count"`
	ActiveCount     int             `json:"active_count"`
	DoneCount       int             `json:"done_count"`
	FaultCount      int             `json:"fault_count"`
	AbortedCount    int             `json:"aborted_count"`
	SafeCount       int             `json:"safe_count"`
	MarginalCount   int             `json:"marginal_count"`
	RunawayCount    int             `json:"runaway_count"`
	EarliestStart   int64           `json:"earliest_start"`
	LatestFinish    int64           `json:"latest_finish"`
	CriticalBatches []string        `json:"critical_batches"`
	ReactorLoads    []ReactorLoad   `json:"reactor_loads"`
	Forecasts       []BatchForecast `json:"forecasts"`
	Warnings        []string        `json:"warnings"`
}
