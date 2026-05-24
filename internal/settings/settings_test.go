package settings

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestOpenMissingFileIsDefaults(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get(); !reflect.DeepEqual(got, Settings{}) {
		t.Errorf("missing file should yield zero Settings, got %+v", got)
	}
}

func TestSaveThenReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{EcuID: "roof-1", MAC: "80:97:1b:03:0d:ce", PANOverride: "0DCE", ZigbeeType: "apsystems"}
	if err := st.Save(want); err != nil {
		t.Fatal(err)
	}
	if got := st.Get(); !reflect.DeepEqual(got, want) {
		t.Errorf("Get after Save = %+v, want %+v", got, want)
	}
	// Reopen from disk: the values persisted (and the dir was created).
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st2.Get(); !reflect.DeepEqual(got, want) {
		t.Errorf("reopened = %+v, want %+v", got, want)
	}
}
