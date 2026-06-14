package codec

// NameplateWattsForModel returns the rated AC output (watts) for the
// given inverter model code, or 0 if the model is unknown.
//
// Values are the "maximum continuous output power" from the APsystems
// datasheet for each model — the ceiling at which the AC output stage
// is rated to operate continuously, NOT the short-term peak and NOT a
// region-derated grid-code config limit. EMEA/global (230V-50Hz)
// figures are used; a few region SKUs differ slightly (e.g. NA YC600
// is 548 vs EMEA 550). Fleet sums feed SunSpec Model 120 (Nameplate
// Ratings) and the value also caps the per-inverter power slider.
//
// The DS3 family is reported under a single wire code 0x20 unless a
// unit is provisioned to distinguish the H/L variant, so a base-DS3
// reading uses the 880 VA hardware rating.
func NameplateWattsForModel(modelCode uint8) uint32 {
	switch modelCode {
	case ModelYC600Old, ModelYC600, ModelYC600B:
		return 550 // YC600 family: 550 VA continuous (600 VA peak)
	case ModelYC1000:
		return 900 // YC1000-3, legacy true three-phase quad
	case ModelQS1:
		return 1200
	case ModelQS1A:
		return 1500
	case ModelDS3:
		return 880
	case ModelDS3H:
		return 960
	case ModelDS3L:
		return 730
	case ModelQS2:
		return 2200 // single-phase 4-channel, EMEA 230V-50Hz
	case ModelQT2:
		return 2000 // EMEA 3/N/PE 400V (NA 480V SKU is 1800)
	case ModelExt29, ModelExt30, ModelExt31:
		return 0 // three-phase variants, AC rating not yet pinned
	}
	return 0
}
