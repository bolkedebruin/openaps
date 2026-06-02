package codec

// NameplateWattsForModel returns the rated AC output (watts) for the
// given inverter model code, or 0 if the model is unknown.
//
// Values are AC nameplate per the manufacturer's datasheet — the
// ceiling at which the inverter's AC output stage is rated to operate.
// Fleet sums are useful for SunSpec Model 120 (Nameplate Ratings).
//
// Authoritative entries are pinned to APsystems' own model→watts
// table. Defaults for the rest are derived from product datasheets
// and may need adjustment for region-specific submodels.
func NameplateWattsForModel(modelCode uint8) uint32 {
	switch modelCode {
	case ModelYC600:
		return 600
	case ModelQS1:
		return 1200
	case ModelQS1A:
		return 1600
	case ModelDS3:
		return 750
	case ModelDS3H:
		return 880
	case ModelDS3L:
		return 600
	case ModelExt36:
		return 750
	case ModelQT2:
		return 1800
	case ModelExt29, ModelExt30, ModelExt31:
		return 0 // three-phase variants, AC rating not yet pinned
	}
	return 0
}
