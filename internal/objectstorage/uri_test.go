package objectstorage

import (
	"testing"

	"github.com/latitudesh/lsh/internal/exitcode"
)

func TestParseRemote(t *testing.T) {
	cases := []struct {
		in         string
		bucket     string
		key        string
		hadScheme  bool
		wantErr    bool
		wantIsDir  bool
		wantString string
	}{
		{"s3://backups", "backups", "", true, false, true, "s3://backups"},
		{"s3://backups/", "backups", "", true, false, true, "s3://backups"},
		{"S3://backups/2026/09/", "backups", "2026/09/", true, false, true, "s3://backups/2026/09/"},
		{"lsh://bkt_123/a/b.txt", "bkt_123", "a/b.txt", true, false, false, "s3://bkt_123/a/b.txt"},
		{"backups/logs/", "backups", "logs/", false, false, true, "s3://backups/logs/"},
		{"backups", "backups", "", false, false, true, "s3://backups"},
		{"", "", "", false, true, false, ""},
		{"-", "", "", false, true, false, ""},
		{"./file", "", "", false, true, false, ""},
		{"/tmp/x", "", "", false, true, false, ""},
		{"s3:///nobucket", "", "", false, true, false, ""},
	}
	for _, c := range cases {
		r, err := ParseRemote(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRemote(%q): expected error", c.in)
			} else if exitcode.Of(err) != exitcode.Usage {
				t.Errorf("ParseRemote(%q): exit code %d, want %d", c.in, exitcode.Of(err), exitcode.Usage)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRemote(%q): unexpected error %v", c.in, err)
			continue
		}
		if r.Bucket != c.bucket || r.Key != c.key || r.HadScheme != c.hadScheme || !r.Remote {
			t.Errorf("ParseRemote(%q) = %+v", c.in, r)
		}
		if r.IsDir() != c.wantIsDir {
			t.Errorf("ParseRemote(%q).IsDir() = %v, want %v", c.in, r.IsDir(), c.wantIsDir)
		}
		if r.String() != c.wantString {
			t.Errorf("ParseRemote(%q).String() = %q, want %q", c.in, r.String(), c.wantString)
		}
	}
}

func TestParseBucketOnlyRejectsKeys(t *testing.T) {
	if _, err := ParseBucketOnly("s3://b/key"); err == nil {
		t.Fatal("expected error for key in bucket-only argument")
	}
	r, err := ParseBucketOnly("b")
	if err != nil || r.Bucket != "b" {
		t.Fatalf("ParseBucketOnly(b) = %+v, %v", r, err)
	}
}

func TestParseTransferArg(t *testing.T) {
	if r, _ := ParseTransferArg("-"); !r.Stdio {
		t.Error("- should be stdio")
	}
	if r, _ := ParseTransferArg("./dir/file.txt"); r.Remote || r.Stdio {
		t.Error("bare path should be local")
	}
	if r, _ := ParseTransferArg("backups/file.txt"); r.Remote {
		t.Error("bare bucket/key without scheme must stay local in transfer args")
	}
	r, err := ParseTransferArg("s3://backups/dir/")
	if err != nil || !r.Remote || r.Bucket != "backups" || r.Key != "dir/" {
		t.Errorf("ParseTransferArg(s3://backups/dir/) = %+v, %v", r, err)
	}
}

func TestObjectRef(t *testing.T) {
	if _, err := ObjectRef("s3://b", false); err == nil {
		t.Error("bucket-only must be rejected")
	}
	if _, err := ObjectRef("s3://b/dir/", false); err == nil {
		t.Error("trailing slash must be rejected without allowPrefix")
	}
	if _, err := ObjectRef("s3://b/dir/", true); err != nil {
		t.Errorf("trailing slash with allowPrefix: %v", err)
	}
}

func TestKeyHelpers(t *testing.T) {
	if JoinKey("", "a") != "a" || JoinKey("p/", "a") != "p/a" || JoinKey("p", "a") != "p/a" {
		t.Error("JoinKey")
	}
	if BaseName("a/b/c.txt") != "c.txt" || BaseName("c.txt") != "c.txt" || BaseName("a/") != "" {
		t.Error("BaseName")
	}
	if NormalizePrefix("") != "" || NormalizePrefix("a") != "a/" || NormalizePrefix("a/") != "a/" {
		t.Error("NormalizePrefix")
	}
}
