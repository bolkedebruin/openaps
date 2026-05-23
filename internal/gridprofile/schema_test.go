package gridprofile

import (
	"testing"
)

var validProfile = []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "EN50549-1",
  "vnom_v": 230,
  "source": {"system":"apsystems","ref":"TY=36"},
  "points": [
    {
      "model": 711,
      "group": "DERFreqDroop",
      "index": 0,
      "point": "KOf",
      "sf_ref": "K_SF",
      "sf": -2,
      "native": {"value": 16.7, "unit": "%Pref/Hz"},
      "sunspec": {"value": 1670},
      "range": {"min": 0, "max": 50},
      "apply": {"aps_code": "DD"}
    }
  ]
}`)

var validOverlay = []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "my-overlay",
  "uids": ["704000006835"],
  "points": [
    {
      "model": 134,
      "group": "CrvSet",
      "index": 0,
      "point": "Hz3",
      "sf_ref": "Hz_SF",
      "sf": -2,
      "native": {"value": 50.2, "unit": "Hz"},
      "apply": {"aps_code": "CB"}
    }
  ]
}`)

func TestValidate_ValidProfile(t *testing.T) {
	if err := ValidateProfile(validProfile); err != nil {
		t.Errorf("valid profile rejected: %v", err)
	}
}

func TestValidate_ValidOverlay(t *testing.T) {
	if err := ValidateOverlay(validOverlay); err != nil {
		t.Errorf("valid overlay rejected: %v", err)
	}
}

func TestValidate_BadSchema(t *testing.T) {
	bad := []byte(`{
  "schema": "wrong/version",
  "id": "x",
  "vnom_v": 230,
  "source": {"system":"a","ref":"b"},
  "points": [{"model":711,"group":"g","point":"Hz","native":{"value":1,"unit":"Hz"},"apply":{"aps_code":"CB"}}]
}`)
	if err := Validate(bad); err == nil {
		t.Error("bad schema version should be rejected")
	}
}

func TestValidate_MissingRequired(t *testing.T) {
	// Profile missing 'id'
	bad := []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "vnom_v": 230,
  "source": {"system":"a","ref":"b"},
  "points": [{"model":711,"group":"g","point":"Hz","native":{"value":1,"unit":"Hz"},"apply":{"aps_code":"CB"}}]
}`)
	if err := ValidateProfile(bad); err == nil {
		t.Error("missing 'id' should be rejected")
	}
}

func TestValidate_BadApsCode(t *testing.T) {
	// aps_code must match ^[A-Z]{2}$
	bad := []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "test",
  "vnom_v": 230,
  "source": {"system":"a","ref":"b"},
  "points": [{"model":711,"group":"g","point":"Hz","native":{"value":1,"unit":"Hz"},"apply":{"aps_code":"cb"}}]
}`)
	if err := ValidateProfile(bad); err == nil {
		t.Error("lowercase aps_code should be rejected")
	}
}

func TestValidate_BadModelEnum(t *testing.T) {
	// model must be in [134,703,705,706,707,708,709,710,711]
	bad := []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "test",
  "vnom_v": 230,
  "source": {"system":"a","ref":"b"},
  "points": [{"model":999,"group":"g","point":"Hz","native":{"value":1,"unit":"Hz"},"apply":{"aps_code":"CB"}}]
}`)
	if err := ValidateProfile(bad); err == nil {
		t.Error("invalid model enum should be rejected")
	}
}

func TestValidate_BadPointEnum(t *testing.T) {
	bad := []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "test",
  "vnom_v": 230,
  "source": {"system":"a","ref":"b"},
  "points": [{"model":711,"group":"g","point":"NotAPoint","native":{"value":1,"unit":"Hz"},"apply":{"aps_code":"CB"}}]
}`)
	if err := ValidateProfile(bad); err == nil {
		t.Error("invalid point enum should be rejected")
	}
}

func TestValidate_InvertedRange(t *testing.T) {
	// range.min > range.max — caught by validateRanges, not the JSON schema
	bad := []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "test",
  "vnom_v": 230,
  "source": {"system":"a","ref":"b"},
  "points": [
    {
      "model": 711,
      "group": "DERFreqDroop",
      "point": "KOf",
      "sf_ref": "K_SF",
      "sf": -2,
      "native": {"value": 16.7, "unit": "%Pref/Hz"},
      "range": {"min": 50, "max": 0},
      "apply": {"aps_code": "DD"}
    }
  ]
}`)
	if err := ValidateProfile(bad); err == nil {
		t.Error("inverted range should be rejected by ValidateProfile")
	}
}

func TestValidate_OverlayMissingUids(t *testing.T) {
	bad := []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "test",
  "points": [{"model":711,"group":"g","point":"Hz","native":{"value":1,"unit":"Hz"},"apply":{"aps_code":"CB"}}]
}`)
	if err := ValidateOverlay(bad); err == nil {
		t.Error("overlay missing uids should be rejected")
	}
}

func TestValidate_VnomNegative(t *testing.T) {
	bad := []byte(`{
  "schema": "invdriver.gridprofile/v1",
  "id": "test",
  "vnom_v": -10,
  "source": {"system":"a","ref":"b"},
  "points": [{"model":711,"group":"g","point":"Hz","native":{"value":1,"unit":"Hz"},"apply":{"aps_code":"CB"}}]
}`)
	if err := ValidateProfile(bad); err == nil {
		t.Error("negative vnom_v should be rejected")
	}
}

func TestSchemaBytes(t *testing.T) {
	b := SchemaBytes()
	if len(b) == 0 {
		t.Error("SchemaBytes returned empty")
	}
}
