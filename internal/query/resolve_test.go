package query

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeDatasetPage(dir, slug, storage, format, dataPath, body string) {
	content := "---\ntitle: \"" + slug + "\"\nstorage: " + storage + "\nformat: " + format + "\n"
	if dataPath != "" {
		content += "data_path: " + dataPath + "\n"
	}
	content += "---\n\n# " + slug + "\n\n## Data\n```" + format + "\n" + body + "\n```\n"
	os.WriteFile(filepath.Join(dir, slug+".md"), []byte(content), 0644)
}

func TestResolveSQL_FileStorageCSV(t *testing.T) {
	tmp := t.TempDir()
	datasetsDir := filepath.Join(tmp, "content", "datasets")
	os.MkdirAll(datasetsDir, 0755)

	// Create a CSV data file.
	csvPath := filepath.Join(tmp, "mydata.csv")
	os.WriteFile(csvPath, []byte("a,b\n1,2\n"), 0644)

	makeDatasetPage(datasetsDir, "mydata", "file", "csv", csvPath, "")

	sql := "SELECT * FROM mydata ORDER BY a"
	rewritten, temps, err := resolveSQLWithDir(datasetsDir, tmp, sql)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupTemps(temps)

	if len(temps) != 0 {
		t.Errorf("file storage should produce no temp files, got %d", len(temps))
	}
	if !strings.Contains(rewritten, "CREATE VIEW mydata") {
		t.Errorf("expected CREATE VIEW in rewritten SQL: %q", rewritten)
	}
	if !strings.Contains(rewritten, "read_csv") {
		t.Errorf("expected read_csv in rewritten SQL: %q", rewritten)
	}
}

func TestResolveSQL_FileStorageJSON(t *testing.T) {
	tmp := t.TempDir()
	datasetsDir := filepath.Join(tmp, "content", "datasets")
	os.MkdirAll(datasetsDir, 0755)

	jsonPath := filepath.Join(tmp, "mydata.json")
	os.WriteFile(jsonPath, []byte(`[{"a":1}]`), 0644)

	makeDatasetPage(datasetsDir, "myjson", "file", "json", jsonPath, "")

	sql := "SELECT * FROM myjson ORDER BY a"
	rewritten, temps, err := resolveSQLWithDir(datasetsDir, tmp, sql)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupTemps(temps)

	if !strings.Contains(rewritten, "read_json_auto") {
		t.Errorf("expected read_json_auto for json storage: %q", rewritten)
	}
}

func TestResolveSQL_InlineCSV(t *testing.T) {
	tmp := t.TempDir()
	datasetsDir := filepath.Join(tmp, "content", "datasets")
	os.MkdirAll(datasetsDir, 0755)

	makeDatasetPage(datasetsDir, "inline-data", "inline", "csv", "", "x,y\n10,20")

	sql := "SELECT * FROM \"inline-data\" ORDER BY x"
	rewritten, temps, err := resolveSQLWithDir(datasetsDir, tmp, sql)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupTemps(temps)

	if len(temps) == 0 {
		t.Error("inline storage should produce temp files")
	}
	if !strings.Contains(rewritten, "CREATE VIEW") {
		t.Errorf("expected CREATE VIEW: %q", rewritten)
	}
}

func TestResolveSQL_MissingDataset(t *testing.T) {
	tmp := t.TempDir()
	datasetsDir := filepath.Join(tmp, "content", "datasets")
	os.MkdirAll(datasetsDir, 0755)

	sql := "SELECT * FROM nonexistent ORDER BY 1"
	_, _, err := resolveSQLWithDir(datasetsDir, tmp, sql)
	if err == nil {
		t.Fatal("expected error for missing dataset")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention slug, got: %v", err)
	}
}
