package s3

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/objectstorage"
)

// fixedExportTarget is the bucket/credential every golden test renders.
func fixedExportTarget(t *testing.T) exportTarget {
	t.Helper()
	target, err := newExportTarget("lsh-backups", "https://s3.us-central-1.storage.sh", "", "backups-7f3a",
		objectstorage.ClassStandard, "XL68DDURVGUUOULWPCAE", "sEcReT/KeY+123")
	if err != nil {
		t.Fatalf("newExportTarget: %v", err)
	}
	return target
}

func TestNewExportTargetDerivesHostAndRegion(t *testing.T) {
	tg := fixedExportTarget(t)
	if tg.Host != "s3.us-central-1.storage.sh" || !tg.Secure {
		t.Fatalf("host/secure: %q %v", tg.Host, tg.Secure)
	}
	if tg.SigningRegion != "us-central-1" {
		t.Fatalf("signing region should come from the endpoint, got %q", tg.SigningRegion)
	}
	plain, err := newExportTarget("p", "http://127.0.0.1:9000/", "us-east-1", "b", "", "AK", "SK")
	if err != nil {
		t.Fatal(err)
	}
	if plain.Secure || plain.Host != "127.0.0.1:9000" || plain.Endpoint != "http://127.0.0.1:9000" {
		t.Fatalf("plain http target: %+v", plain)
	}
	if _, err := newExportTarget("p", "ftp://x", "", "", "", "", ""); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("bad scheme must be a usage error, got %v", err)
	}
}

func TestExportEnvGolden(t *testing.T) {
	want := strings.Join([]string{
		"# bucket name on the endpoint: backups-7f3a",
		"export AWS_ACCESS_KEY_ID=XL68DDURVGUUOULWPCAE",
		"export AWS_SECRET_ACCESS_KEY=sEcReT/KeY+123",
		"export AWS_ENDPOINT_URL_S3=https://s3.us-central-1.storage.sh",
		"export AWS_REGION=us-central-1",
		"export AWS_REQUEST_CHECKSUM_CALCULATION=when_required",
		"export AWS_RESPONSE_CHECKSUM_VALIDATION=when_required",
		"export LSH_S3_ACCESS_KEY_ID=XL68DDURVGUUOULWPCAE",
		"export LSH_S3_SECRET_ACCESS_KEY=sEcReT/KeY+123",
		"export LSH_S3_ENDPOINT_URL=https://s3.us-central-1.storage.sh",
		"export LSH_S3_SIGNING_REGION=us-central-1",
		"# aws s3 ls s3://backups-7f3a/",
		"",
	}, "\n")
	if got := exportEnv(fixedExportTarget(t)); got != want {
		t.Fatalf("env template mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestExportAWSGolden(t *testing.T) {
	want := strings.Join([]string{
		"# bucket name on the endpoint: backups-7f3a",
		"# ~/.aws/config",
		"[profile lsh-backups]",
		"region = us-central-1",
		"endpoint_url = https://s3.us-central-1.storage.sh",
		"s3 =",
		"    addressing_style = path",
		"request_checksum_calculation = when_required",
		"response_checksum_validation = when_required",
		"",
		"# ~/.aws/credentials",
		"[lsh-backups]",
		"aws_access_key_id = XL68DDURVGUUOULWPCAE",
		"aws_secret_access_key = sEcReT/KeY+123",
		"",
		"# aws --profile lsh-backups s3 ls s3://backups-7f3a/",
		"",
	}, "\n")
	if got := exportAWS(fixedExportTarget(t)); got != want {
		t.Fatalf("aws template mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestExportRcloneGolden(t *testing.T) {
	want := strings.Join([]string{
		"# bucket name on the endpoint: backups-7f3a",
		"[lsh-backups]",
		"type = s3",
		"provider = Wasabi",
		"env_auth = false",
		"access_key_id = XL68DDURVGUUOULWPCAE",
		"secret_access_key = sEcReT/KeY+123",
		"endpoint = s3.us-central-1.storage.sh",
		"region = us-central-1",
		"force_path_style = true",
		"",
		"# rclone ls lsh-backups:backups-7f3a",
		"",
	}, "\n")
	if got := exportRclone(fixedExportTarget(t)); got != want {
		t.Fatalf("rclone template mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	hp := fixedExportTarget(t)
	hp.StorageClass = objectstorage.ClassHighPerformance
	if !strings.Contains(exportRclone(hp), "provider = Other\n") {
		t.Fatal("high_performance must map to provider = Other")
	}
	unknown := fixedExportTarget(t)
	unknown.StorageClass = ""
	unknown.Host = "objects.tyo4.storage.sh"
	if !strings.Contains(exportRclone(unknown), "provider = Other\n") {
		t.Fatal("objects.* hosts without a class must map to provider = Other")
	}
}

func TestExportMcGolden(t *testing.T) {
	want := "# bucket name on the endpoint: backups-7f3a\n" +
		"mc alias set lsh-backups https://s3.us-central-1.storage.sh XL68DDURVGUUOULWPCAE sEcReT/KeY+123 --api S3v4 --path on\n" +
		"# mc ls lsh-backups/backups-7f3a\n"
	if got := exportMc(fixedExportTarget(t)); got != want {
		t.Fatalf("mc template mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestExportS3cmdGolden(t *testing.T) {
	want := strings.Join([]string{
		"# bucket name on the endpoint: backups-7f3a",
		"[default]",
		"access_key = XL68DDURVGUUOULWPCAE",
		"secret_key = sEcReT/KeY+123",
		"host_base = s3.us-central-1.storage.sh",
		"host_bucket = s3.us-central-1.storage.sh",
		"bucket_location = us-central-1",
		"use_https = True",
		"signature_v2 = False",
		"",
		"# s3cmd ls s3://backups-7f3a",
		"",
	}, "\n")
	if got := exportS3cmd(fixedExportTarget(t)); got != want {
		t.Fatalf("s3cmd template mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	plain := fixedExportTarget(t)
	plain.Secure = false
	if !strings.Contains(exportS3cmd(plain), "use_https = False\n") {
		t.Fatal("http endpoints must render use_https = False")
	}
}

func TestExportProcessGolden(t *testing.T) {
	want := `{"Version":1,"AccessKeyId":"XL68DDURVGUUOULWPCAE","SecretAccessKey":"sEcReT/KeY+123"}` + "\n"
	got := exportProcess(fixedExportTarget(t))
	if got != want {
		t.Fatalf("process template mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("process output is not valid JSON: %v", err)
	}
	if decoded["Version"].(float64) != 1 {
		t.Fatalf("Version must be 1: %v", decoded)
	}
}

func TestRenderExportDispatchAndNoSecret(t *testing.T) {
	base := fixedExportTarget(t)
	base.Secret = exportSecretPlaceholder
	for _, format := range exportFormats {
		out, err := renderExport(format, base)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if out == "" || !strings.HasSuffix(out, "\n") {
			t.Fatalf("%s: output must be non-empty and newline-terminated: %q", format, out)
		}
		if strings.Contains(out, "sEcReT") {
			t.Fatalf("%s: real secret leaked with --no-secret: %s", format, out)
		}
		if !strings.Contains(out, exportSecretPlaceholder) {
			t.Fatalf("%s: placeholder missing: %s", format, out)
		}
		// Nothing time-bound: the templates never mention expiry.
		if strings.Contains(strings.ToLower(out), "expire") {
			t.Fatalf("%s: templates must not carry an expiry: %s", format, out)
		}
	}
	if _, err := renderExport("toml", base); err == nil || exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("unknown format must be a usage error, got %v", err)
	}
	// Case-insensitive format names.
	if _, err := renderExport("ENV", base); err != nil {
		t.Fatalf("format names must be case-insensitive: %v", err)
	}
}

func TestExportWithoutBucketUsesPlaceholders(t *testing.T) {
	tg, err := newExportTarget("lsh-ci", "https://objects.tyo4.storage.sh", "", "", "", "AK", "SK")
	if err != nil {
		t.Fatal(err)
	}
	env := exportEnv(tg)
	if strings.Contains(env, "# bucket name on the endpoint") {
		t.Fatalf("no bucket header expected when the bucket is unknown:\n%s", env)
	}
	if !strings.HasSuffix(env, "# aws s3 ls s3://<bucket>/\n") {
		t.Fatalf("example must use the <bucket> placeholder:\n%s", env)
	}
	if tg.SigningRegion != "tyo4" {
		t.Fatalf("signing region derived from objects.<site> host: %q", tg.SigningRegion)
	}
}

func TestShellValueQuotesUnsafeCharacters(t *testing.T) {
	cases := map[string]string{
		"abc123":      "abc123",
		"a/b+c=d":     "a/b+c=d",
		"has space":   "'has space'",
		"it's":        `'it'\''s'`,
		"":            "''",
		"dollar$sign": "'dollar$sign'",
	}
	for in, want := range cases {
		if got := cfgShellValue(in); got != want {
			t.Errorf("cfgShellValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportProfileNameDefaults(t *testing.T) {
	b := &objectstorage.Bucket{ID: "bkt_1", Name: "backups", BucketName: "backups-7f3a"}
	saved := objectstorage.NewCredential("AK", "SK", "saved")
	saved.Name = "ci-deploy"
	if got := exportProfileName("  custom ", b, saved); got != "custom" {
		t.Fatalf("explicit name wins: %q", got)
	}
	if got := exportProfileName("", b, saved); got != "lsh-backups" {
		t.Fatalf("bucket display name: %q", got)
	}
	if got := exportProfileName("", nil, saved); got != "lsh-ci-deploy" {
		t.Fatalf("saved key name: %q", got)
	}
	env := objectstorage.NewCredential("AKIAENV", "SK", "env")
	if got := exportProfileName("", nil, env); got != "lsh-akiaenv" {
		t.Fatalf("env credential falls back to the key id: %q", got)
	}
	if got := exportProfileName("", nil, objectstorage.Credential{}); got != "lsh" {
		t.Fatalf("last resort: %q", got)
	}
}
