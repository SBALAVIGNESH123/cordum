package packs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestValidatePackManifest_TopicSchemaRefExists(t *testing.T) {
	manifest := &PackManifest{
		Metadata: PackMetadata{ID: "pack1", Version: "1.0.0"},
		Topics: []PackTopic{
			{
				Name:           "job.pack1.topic",
				InputSchemaID:  "pack1/Input",
				OutputSchemaID: "pack1/Output",
			},
		},
		Resources: PackResources{
			Schemas: []PackResource{
				{ID: "pack1/Input", Path: "schemas/input.json"},
				{ID: "pack1/Output", Path: "schemas/output.json"},
			},
			Workflows: []PackResource{
				{ID: "pack1.workflow", Path: "workflows/workflow.yaml"},
			},
		},
	}

	if err := ValidatePackManifest(manifest); err != nil {
		t.Fatalf("expected valid manifest, got: %v", err)
	}
}

func TestValidatePackManifest_TopicSchemaRefMissing(t *testing.T) {
	manifest := &PackManifest{
		Metadata: PackMetadata{ID: "pack1", Version: "1.0.0"},
		Topics: []PackTopic{
			{
				Name:          "job.pack1.topic",
				InputSchemaID: "pack1/Missing",
			},
		},
		Resources: PackResources{
			Schemas: []PackResource{
				{ID: "pack1/Input", Path: "schemas/input.json"},
			},
			Workflows: []PackResource{
				{ID: "pack1.workflow", Path: "workflows/workflow.yaml"},
			},
		},
	}

	err := ValidatePackManifest(manifest)
	if err == nil {
		t.Fatal("expected error for missing topic schema ref")
	}
	if !strings.Contains(err.Error(), "topic job.pack1.topic references unknown schema pack1/Missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePackManifest_TopicNoSchemaIsValid(t *testing.T) {
	manifest := &PackManifest{
		Metadata: PackMetadata{ID: "pack1", Version: "1.0.0"},
		Topics: []PackTopic{
			{Name: "job.pack1.topic"},
		},
		Resources: PackResources{
			Workflows: []PackResource{
				{ID: "pack1.workflow", Path: "workflows/workflow.yaml"},
			},
		},
	}

	if err := ValidatePackManifest(manifest); err != nil {
		t.Fatalf("expected manifest without topic schema refs to be valid, got: %v", err)
	}
}

// TestValidatePackManifest_TopicNamespaceReject pins the three-token error
// shape for the pack-namespace invariant: a manifest whose topic is not
// namespaced under `job.<metadata.id>.*` must be rejected with an error that
// names the offending topic, the expected prefix, AND the derived pack id.
// Operators hit this during `cordumctl pack install` and the message is the
// single-line diagnosis — dropping any of the three tokens makes the failure
// harder to triage.
func TestValidatePackManifest_TopicNamespaceReject(t *testing.T) {
	manifest := &PackManifest{
		Metadata: PackMetadata{ID: "demo-quickstart", Version: "1.0.0"},
		Topics: []PackTopic{
			{Name: "job.other-pack.greet"},
		},
	}

	err := ValidatePackManifest(manifest)
	if err == nil {
		t.Fatal("expected error for topic not namespaced under job.<id>.*")
	}
	msg := err.Error()
	for _, want := range []string{
		"job.other-pack.greet",    // offending topic
		"job.demo-quickstart.*",   // expected prefix
		`metadata.id="demo-quickstart"`, // explicit derivation source so operators see which
		// manifest field produced the prefix (not just transitively via the
		// prefix substring). Required phrasing: `metadata.id="<id>"`.
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing required token %q", msg, want)
		}
	}
}

// TestValidatePackManifest_TopicNamespaceAccept is the matching accept path
// for TestValidatePackManifest_TopicNamespaceReject: same manifest shape, but
// the topic is correctly namespaced under the pack id. Guards against a
// validator refactor that over-restricts valid topic names.
func TestValidatePackManifest_TopicNamespaceAccept(t *testing.T) {
	manifest := &PackManifest{
		Metadata: PackMetadata{ID: "demo-quickstart", Version: "1.0.0"},
		Topics: []PackTopic{
			{Name: "job.demo-quickstart.greet"},
		},
	}

	if err := ValidatePackManifest(manifest); err != nil {
		t.Fatalf("expected valid namespaced topic, got: %v", err)
	}
}

// buildTarGz creates a gzipped tar archive from the given entries.
func buildTarGz(t *testing.T, entries []tar.Header, contents map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, hdr := range entries {
		h := hdr // copy
		if data, ok := contents[h.Name]; ok {
			h.Size = int64(len(data))
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatalf("write tar header %s: %v", h.Name, err)
		}
		if data, ok := contents[h.Name]; ok {
			if _, err := tw.Write([]byte(data)); err != nil {
				t.Fatalf("write tar data %s: %v", h.Name, err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return &buf
}

func TestExtractTarGzReader_RegularFilesAndDirs(t *testing.T) {
	archive := buildTarGz(t, []tar.Header{
		{Name: "mypack/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "mypack/pack.yaml", Typeflag: tar.TypeReg, Mode: 0o644},
	}, map[string]string{
		"mypack/pack.yaml": "name: test\n",
	})

	dest := t.TempDir()
	if err := ExtractTarGzReader(archive, dest); err != nil {
		t.Fatalf("expected success for regular files, got: %v", err)
	}
}

func TestExtractTarGzReader_RejectsSymlink(t *testing.T) {
	archive := buildTarGz(t, []tar.Header{
		{Name: "mypack/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "mypack/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777},
	}, nil)

	dest := t.TempDir()
	err := ExtractTarGzReader(archive, dest)
	if err == nil {
		t.Fatal("expected error for symlink, got nil")
	}
	if !strings.Contains(err.Error(), "disallowed entry type") {
		t.Fatalf("expected 'disallowed entry type' in error, got: %v", err)
	}
}

func TestExtractTarGzReader_RejectsHardlink(t *testing.T) {
	archive := buildTarGz(t, []tar.Header{
		{Name: "mypack/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "mypack/hardlink", Typeflag: tar.TypeLink, Linkname: "mypack/target", Mode: 0o644},
	}, nil)

	dest := t.TempDir()
	err := ExtractTarGzReader(archive, dest)
	if err == nil {
		t.Fatal("expected error for hardlink, got nil")
	}
	if !strings.Contains(err.Error(), "disallowed entry type") {
		t.Fatalf("expected 'disallowed entry type' in error, got: %v", err)
	}
}

func TestExtractTarGzReader_RejectsDeviceNode(t *testing.T) {
	archive := buildTarGz(t, []tar.Header{
		{Name: "mypack/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "mypack/dev", Typeflag: tar.TypeChar, Mode: 0o666},
	}, nil)

	dest := t.TempDir()
	err := ExtractTarGzReader(archive, dest)
	if err == nil {
		t.Fatal("expected error for device node, got nil")
	}
	if !strings.Contains(err.Error(), "disallowed entry type") {
		t.Fatalf("expected 'disallowed entry type' in error, got: %v", err)
	}
}

func TestExtractTarGzReader_RejectsFifo(t *testing.T) {
	archive := buildTarGz(t, []tar.Header{
		{Name: "mypack/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "mypack/pipe", Typeflag: tar.TypeFifo, Mode: 0o644},
	}, nil)

	dest := t.TempDir()
	err := ExtractTarGzReader(archive, dest)
	if err == nil {
		t.Fatal("expected error for FIFO, got nil")
	}
	if !strings.Contains(err.Error(), "disallowed entry type") {
		t.Fatalf("expected 'disallowed entry type' in error, got: %v", err)
	}
}
