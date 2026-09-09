package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

// SnapshotSchemaVersion is the on-disk version of a registry snapshot.
const SnapshotSchemaVersion = 1

// Snapshot is an immutable point-in-time copy of registry observations.
// Version identifies the producer version, while SchemaVersion identifies the
// JSON contract.
type Snapshot struct {
	SchemaVersion int                 `json:"schema_version"`
	Version       string              `json:"version,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	Observations  []model.Observation `json:"observations"`
}

// SnapshotInfo contains the metadata needed to display or retain a snapshot
// without exposing the implementation details of its file format.
type SnapshotInfo struct {
	Path          string
	SchemaVersion int
	Version       string
	CreatedAt     time.Time
}

// SnapshotStore persists snapshots in one directory. An empty directory uses
// ~/.toolsniff/snapshots.
type SnapshotStore struct {
	Dir     string
	Version string
}

// NewSnapshot creates a current-format snapshot with a UTC creation time.
func NewSnapshot(observations []model.Observation, version string) Snapshot {
	if observations == nil {
		observations = []model.Observation{}
	}
	return Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		Version:       version,
		CreatedAt:     time.Now().UTC(),
		Observations:  append([]model.Observation(nil), observations...),
	}
}

// NewSnapshotStore creates a store rooted at dir. Empty dir selects the
// default per-user snapshot directory.
func NewSnapshotStore(dir string) SnapshotStore {
	if dir == "" {
		dir = DefaultSnapshotDir()
	}
	return SnapshotStore{Dir: dir}
}

// DefaultSnapshotDir returns ~/.toolsniff/snapshots.
func DefaultSnapshotDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".toolsniff", "snapshots")
}

// SaveDefaultSnapshot writes a snapshot in the default snapshot directory and
// returns the generated path.
func SaveDefaultSnapshot(snapshot Snapshot) (string, error) {
	return NewSnapshotStore("").Save(snapshot)
}

// Save writes a timestamped snapshot and returns its path.
func (s SnapshotStore) Save(snapshot Snapshot) (string, error) {
	if s.Dir == "" {
		return "", fmt.Errorf("registry: empty snapshot directory")
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}
	if snapshot.Version == "" {
		snapshot.Version = s.Version
	}
	name := "snapshot-" + snapshot.CreatedAt.UTC().Format("20060102T150405.000000000Z")
	path := filepath.Join(s.Dir, name+".json")
	for suffix := 1; ; suffix++ {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("registry: checking snapshot path: %w", err)
		}
		path = filepath.Join(s.Dir, fmt.Sprintf("%s-%d.json", name, suffix))
	}
	if err := SaveSnapshot(path, snapshot); err != nil {
		return "", err
	}
	return path, nil
}

// SaveSnapshot atomically writes one snapshot to path. It validates all
// observations before creating a temporary file, so a failed save cannot
// replace an existing snapshot.
func SaveSnapshot(path string, snapshot Snapshot) error {
	if path == "" {
		return fmt.Errorf("registry: empty snapshot path")
	}
	if snapshot.SchemaVersion == 0 {
		snapshot.SchemaVersion = SnapshotSchemaVersion
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("registry: creating snapshot directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("registry: securing snapshot directory: %w", err)
	}

	canonical := append([]model.Observation(nil), snapshot.Observations...)
	if canonical == nil {
		canonical = []model.Observation{}
	}
	sortObservations(canonical)
	snapshot.Observations = canonical
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshaling snapshot: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("registry: creating snapshot temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: securing snapshot temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: writing snapshot temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: syncing snapshot temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("registry: closing snapshot temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("registry: replacing snapshot %s: %w", path, err)
	}
	return nil
}

// LoadSnapshot reads and validates one snapshot.
func LoadSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("registry: reading snapshot %s: %w", path, err)
	}
	var snapshot Snapshot
	if err := decodeStrict(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("registry: parsing snapshot %s: %w", path, err)
	}
	if err := validateSnapshot(snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("registry: invalid snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

// ListSnapshots returns valid snapshots in deterministic newest-first order.
// It never removes files and ignores temporary, unrelated, or corrupt files,
// which keeps retention callers safe in the presence of partial writes.
func ListSnapshots(dir string, limits ...int) ([]SnapshotInfo, error) {
	if dir == "" {
		dir = DefaultSnapshotDir()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotInfo{}, nil
		}
		return nil, fmt.Errorf("registry: listing snapshots: %w", err)
	}

	result := make([]SnapshotInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isSnapshotName(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		snapshot, err := LoadSnapshot(path)
		if err != nil {
			continue
		}
		result = append(result, SnapshotInfo{
			Path:          path,
			SchemaVersion: snapshot.SchemaVersion,
			Version:       snapshot.Version,
			CreatedAt:     snapshot.CreatedAt,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].Path < result[j].Path
	})
	limit := 0
	if len(limits) > 0 {
		limit = limits[0]
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// List returns snapshots from the store using the same retention-safe rules
// as ListSnapshots.
func (s SnapshotStore) List(limits ...int) ([]SnapshotInfo, error) {
	return ListSnapshots(s.Dir, limits...)
}

func validateSnapshot(snapshot Snapshot) error {
	if snapshot.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("registry: unsupported snapshot schema version %d", snapshot.SchemaVersion)
	}
	if snapshot.CreatedAt.IsZero() {
		return fmt.Errorf("registry: snapshot created_at is required")
	}
	if snapshot.Observations == nil {
		return fmt.Errorf("registry: snapshot observations must be an array")
	}
	for i, observation := range snapshot.Observations {
		if err := observation.Validate(); err != nil {
			return fmt.Errorf("registry: snapshot observation %d: %w", i, err)
		}
	}
	return nil
}

func isSnapshotName(name string) bool {
	return strings.HasPrefix(name, "snapshot-") && strings.HasSuffix(name, ".json")
}
