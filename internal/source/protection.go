package source

// ProtectionParams holds the per-inverter active grid-protection thresholds,
// sourced live from inv-driver's protection read (codec.DecodeProtectionReply
// → wire.Protection → protectionFromWire). The field set mirrors the
// protection_parameters60code columns.
//
// The 2-letter codes match protection_parameters60_info.parameter_code and
// MAIN_PROTOCOL.md §4.4. Values are in their native units:
//
//	V codes (AB, AC, AD, AH, AI, AQ, AY, BN, BO):  volts
//	Hz codes (AE, AF, AJ, AK, BP, BQ):             hertz
//	Time codes (AG=ReconnectS, AS=StartS,
//	            BB..BK clearance times):           seconds
//	CH (PFMode):                                   firmware-defined enum
//
// Has reports which codes the inverter firmware actually populated. Some
// firmware revisions / regulatory profiles leave cells NULL; consumers should
// emit "not implemented" sentinels for those.
type ProtectionParams struct {
	UVStg2  float64 // AC — under-voltage stage 2 (slow trip threshold)
	UVFast  float64 // AQ — under-voltage fast (deeper trip threshold)
	OVStg2  float64 // AD — over-voltage stage 2 (slow trip threshold)
	OVStg3  float64 // AY — over-voltage stage 3 (deeper trip threshold)
	AvgOV   float64 // AB — averaged over-voltage (10-min window)
	VWinLow float64 // AH — voltage window low bound
	VWinHi  float64 // AI — voltage window high bound

	UFSlow float64 // AE — under-frequency stage 2
	OFSlow float64 // AF — over-frequency stage 2
	UFFast float64 // AJ — under-frequency fast
	OFFast float64 // AK — over-frequency fast

	ReconnectS float64 // AG — grid recovery time
	StartS     float64 // AS — start time

	UV2ClrS float64 // BB — UV stage 2 trip clearance
	OV2ClrS float64 // BC — OV stage 2 trip clearance
	UV3ClrS float64 // BD — UV stage 3 / fast clearance
	OV3ClrS float64 // BE — OV stage 3 clearance
	UF1ClrS float64 // BH — UF fast clearance
	OF1ClrS float64 // BI — OF fast clearance
	UF2ClrS float64 // BJ — UF stage 2 clearance
	OF2ClrS float64 // BK — OF stage 2 clearance

	ReconnVLow float64 // BN
	ReconnVHi  float64 // BO
	ReconnFLow float64 // BP
	ReconnFHi  float64 // BQ

	PFMode float64 // CH — fixed power factor mode (enum, not numeric PF)

	// Frequency-Watt droop curve (over-frequency side). Maps the APsystems
	// Over_frequency_Watt_* settings exposed through 2-letter codes
	// observed in the gridProfile JSON and protection_parameters60code:
	//
	//	DC = OFDroopStart  (Over_frequency_Watt_Start_set, e.g. 50.2 Hz)
	//	CC = OFDroopEnd    (Over_frequency_Watt_High_set, e.g. 52.0 Hz — output → 0 here)
	//	DD = OFDroopSlope  (Over_Frequency_Watt_Slope_set, %P/Hz)
	//	CV = OFDroopMode   (Over_frequency_Watt_set; enum: 13 = AS/NZS,
	//	                    14 = other regions, 15 = disabled, others region-specific)
	//
	// These map directly to SunSpec Model 711 (DERFreqDroop) Ctl[].DbOf,
	// the curve endpoint, KOf slope, and Ena.
	OFDroopStart float64 // DC
	OFDroopEnd   float64 // CC
	OFDroopSlope float64 // DD
	OFDroopMode  float64 // CV

	// Curve-based freq-watt thresholds. Map onto SunSpec Model 134's
	// 4-point Freq-Watt curve (DH/DI under-freq, CB/CC over-freq).
	//
	//	DH = OFCurveUFLow   (Under_Frequency_Watt_Low_set,  W=0%)
	//	DI = OFCurveUFHigh  (Under_Frequency_Watt_High_set, W=100%)
	//	CB = OFCurveOFLow   (Over_frequency_Watt_Low_set,   W=100%)
	//	CC overlaps OFDroopEnd above (Over_frequency_Watt_High_set, W=0%).
	OFCurveUFLow  float64 // DH
	OFCurveUFHigh float64 // DI
	OFCurveOFLow  float64 // CB

	Has map[string]bool
}
